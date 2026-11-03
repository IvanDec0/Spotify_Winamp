package skins

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"cadence/internal/settings"
)

var (
	builtinThemes = []string{"graphite", "ice", "violet", "amber", "acid", "crimson"}
	builtinModes  = []string{"spectrum", "radial", "oscilloscope", "grid"}
)

// postInstallLoader is a test seam for the post-rename validation step. It is
// always loadSkin in production.
var postInstallLoader = loadSkin

// Options configures a Manager. SkinsDir and Store are required; DevDir is
// optional and only used while developing skins in the repository.
type Options struct {
	SkinsDir string
	DevDir   string
	Store    *settings.Store
	SafeMode bool
	Emit     func(SkinCatalog)
}

type managedSkin struct {
	summary     SkinSummary
	manifest    Manifest
	layout      Layout
	root        string
	source      string
	stylesheets map[string]string
}

// Manager owns the installed/development skin catalog, the persisted active
// selection and the built-in Clarity fallback. All exported methods are safe
// for concurrent use.
type Manager struct {
	mu       sync.Mutex
	skinsDir string
	devDir   string
	store    *settings.Store
	safeMode bool
	emit     func(SkinCatalog)

	installed map[string]*managedSkin
	develop   map[string]*managedSkin
	config    settings.Config
	warnings  []string
	lastError string
}

// NewManager scans the managed directories and resolves a safe active skin.
// A broken active skin never prevents construction: the manager falls back to
// the last known good skin, then to the built-in skin.
func NewManager(options Options) (*Manager, error) {
	if options.SkinsDir == "" {
		return nil, errors.New("skins directory is required")
	}
	if options.Store == nil {
		return nil, errors.New("settings store is required")
	}
	if err := os.MkdirAll(options.SkinsDir, 0o700); err != nil {
		return nil, fmt.Errorf("create skins directory: %w", err)
	}

	manager := &Manager{
		skinsDir:  options.SkinsDir,
		devDir:    options.DevDir,
		store:     options.Store,
		safeMode:  options.SafeMode,
		emit:      options.Emit,
		installed: make(map[string]*managedSkin),
		develop:   make(map[string]*managedSkin),
	}
	if manager.devDir != "" {
		if manager.devDir == manager.skinsDir {
			manager.devDir = ""
		} else if info, err := os.Stat(manager.devDir); err != nil || !info.IsDir() {
			manager.devDir = ""
		}
	}

	config, err := manager.store.Load()
	if err != nil {
		manager.warnings = append(manager.warnings, "settings: "+err.Error())
		config = settings.Default()
	}
	manager.config = normalizeConfig(config)

	manager.cleanupTransientLocked()
	manager.scanLocked()
	manager.resolveActiveLocked()
	if !manager.safeMode {
		if err := manager.store.Save(manager.config); err != nil {
			manager.warnings = append(manager.warnings, "save settings: "+err.Error())
		}
	}
	return manager, nil
}

// SetEmitter installs the callback used to publish catalog updates. It is
// normally called once from the Wails service startup hook.
func (m *Manager) SetEmitter(emit func(SkinCatalog)) {
	m.mu.Lock()
	m.emit = emit
	m.mu.Unlock()
}

// Catalog returns a snapshot of the current skin state.
func (m *Manager) Catalog() SkinCatalog {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.catalogLocked()
}

// ActiveRuntime returns the declarative runtime for the active skin. Built-in
// Clarity is represented by an empty layout with Builtin=true; the frontend
// renders the native Clarity skin in that case.
func (m *Manager) ActiveRuntime() (SkinRuntime, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.safeMode || m.config.Skins.ActiveID == BuiltinID {
		return builtinRuntime(), nil
	}
	skin, ok := m.installed[m.config.Skins.ActiveID]
	if !ok {
		skin = m.develop[m.config.Skins.ActiveID]
	}
	if skin == nil {
		return SkinRuntime{}, fmt.Errorf("active skin %q is not installed", m.config.Skins.ActiveID)
	}
	return runtimeFor(skin), nil
}

// Activate switches to a skin. The previous skin is retained as the last
// known good fallback until the frontend confirms that the new one mounted.
func (m *Manager) Activate(id string) (SkinCatalog, error) {
	m.mu.Lock()
	if m.safeMode {
		catalog := m.catalogLocked()
		m.mu.Unlock()
		return catalog, errors.New("safe mode only allows the built-in skin")
	}
	if id != BuiltinID && m.effectiveThirdPartyDisabledLocked() {
		catalog := m.catalogLocked()
		m.mu.Unlock()
		return catalog, errors.New("third-party skins are disabled")
	}

	if id == BuiltinID {
		m.config.Skins.ActiveID = BuiltinID
		m.config.Skins.ActiveTheme = themeOrDefault(m.config.Skins.ActiveTheme, builtinThemes, "graphite")
		m.config.Skins.LastKnownGoodID = BuiltinID
		m.config.Skins.LastKnownGoodTheme = m.config.Skins.ActiveTheme
		m.lastError = ""
		catalog := m.catalogLocked()
		saveErr := m.storeSaveLocked()
		m.mu.Unlock()
		m.notify(catalog)
		return catalog, saveErr
	}

	skin, ok := m.installed[id]
	if !ok {
		skin = m.develop[id]
	}
	if skin == nil {
		catalog := m.catalogLocked()
		m.mu.Unlock()
		return catalog, fmt.Errorf("skin %q is not installed", id)
	}
	reloaded, err := loadSkin(skin.root, skin.source)
	if err != nil {
		delete(m.installed, id)
		delete(m.develop, id)
		m.warnings = append(m.warnings, fmt.Sprintf("skin %q was removed after it became invalid: %v", id, err))
		catalog := m.catalogLocked()
		m.mu.Unlock()
		return catalog, fmt.Errorf("skin %q failed validation: %w", id, err)
	}
	if reloaded.manifest.ID != id {
		catalog := m.catalogLocked()
		m.mu.Unlock()
		return catalog, fmt.Errorf("skin directory %q does not match manifest id %q", id, reloaded.manifest.ID)
	}
	if skin.source == sourceInstalled {
		m.installed[id] = reloaded
	} else {
		m.develop[id] = reloaded
	}

	m.config.Skins.ActiveID = id
	m.config.Skins.ActiveTheme = themeOrDefault(m.config.Skins.ActiveTheme, reloaded.manifest.Themes, reloaded.manifest.DefaultTheme)
	m.lastError = ""
	catalog := m.catalogLocked()
	saveErr := m.storeSaveLocked()
	m.mu.Unlock()
	m.notify(catalog)
	return catalog, saveErr
}

// SetTheme selects one of the active skin's declared themes.
func (m *Manager) SetTheme(theme string) (SkinCatalog, error) {
	m.mu.Lock()
	validThemes, _ := m.activeThemesLocked()
	if !contains(validThemes, theme) {
		catalog := m.catalogLocked()
		m.mu.Unlock()
		return catalog, fmt.Errorf("theme %q is not available for the active skin", theme)
	}
	m.config.Skins.ActiveTheme = theme
	catalog := m.catalogLocked()
	saveErr := m.storeSaveLocked()
	m.mu.Unlock()
	m.notify(catalog)
	return catalog, saveErr
}

// SetThirdPartyDisabled toggles third-party skins. Disabling immediately
// returns to the built-in skin.
func (m *Manager) SetThirdPartyDisabled(disabled bool) (SkinCatalog, error) {
	m.mu.Lock()
	if m.safeMode && !disabled {
		catalog := m.catalogLocked()
		m.mu.Unlock()
		return catalog, errors.New("third-party skins cannot be enabled in safe mode")
	}
	m.config.Skins.ThirdPartyDisabled = disabled
	if disabled && m.config.Skins.ActiveID != BuiltinID {
		m.config.Skins.ActiveID = BuiltinID
		m.config.Skins.ActiveTheme = "graphite"
		m.config.Skins.LastKnownGoodID = BuiltinID
		m.config.Skins.LastKnownGoodTheme = "graphite"
	}
	catalog := m.catalogLocked()
	saveErr := m.storeSaveLocked()
	m.mu.Unlock()
	m.notify(catalog)
	return catalog, saveErr
}

// Reset returns the skin system to the built-in default. It also clears the
// persisted third-party opt-out; the --safe-mode flag stays active for the
// lifetime of the process.
func (m *Manager) Reset() (SkinCatalog, error) {
	m.mu.Lock()
	m.config.Skins = settings.SkinState{
		ActiveID:           BuiltinID,
		ActiveTheme:        "graphite",
		LastKnownGoodID:    BuiltinID,
		LastKnownGoodTheme: "graphite",
	}
	m.lastError = ""
	catalog := m.catalogLocked()
	saveErr := m.storeSaveLocked()
	m.mu.Unlock()
	m.notify(catalog)
	return catalog, saveErr
}

// ReportRuntimeReady records that a skin mounted successfully. The frontend
// calls it after the Shadow DOM host has rendered.
func (m *Manager) ReportRuntimeReady(id, theme string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id != m.config.Skins.ActiveID {
		return nil
	}
	themes, defaultTheme := m.activeThemesLocked()
	m.config.Skins.LastKnownGoodID = id
	m.config.Skins.LastKnownGoodTheme = themeOrDefault(theme, themes, defaultTheme)
	return m.storeSaveLocked()
}

// ReportRuntimeFailure rolls back to the last known good skin, or to Clarity
// when no safe fallback exists. It is called by the frontend when mounting the
// declarative runtime fails.
func (m *Manager) ReportRuntimeFailure(id, reason string) (SkinCatalog, error) {
	m.mu.Lock()
	if id != m.config.Skins.ActiveID {
		catalog := m.catalogLocked()
		m.mu.Unlock()
		return catalog, nil
	}
	if reason == "" {
		reason = "the skin runtime could not be mounted"
	}
	m.lastError = fmt.Sprintf("skin %q failed: %s", id, reason)

	fallbackID := m.config.Skins.LastKnownGoodID
	fallbackTheme := m.config.Skins.LastKnownGoodTheme
	if fallbackID == id || !m.skinExistsLocked(fallbackID) {
		fallbackID = BuiltinID
		fallbackTheme = "graphite"
	}
	m.config.Skins.ActiveID = fallbackID
	themes, defaultTheme := m.themesForLocked(fallbackID)
	m.config.Skins.ActiveTheme = themeOrDefault(fallbackTheme, themes, defaultTheme)
	m.config.Skins.LastKnownGoodID = fallbackID
	m.config.Skins.LastKnownGoodTheme = m.config.Skins.ActiveTheme
	catalog := m.catalogLocked()
	saveErr := m.storeSaveLocked()
	m.mu.Unlock()
	m.notify(catalog)
	return catalog, saveErr
}

// Install validates and installs a directory or ZIP package. It does not
// activate the skin automatically.
func (m *Manager) Install(sourcePath string) (InstallResult, error) {
	m.mu.Lock()
	result, err := m.installLocked(sourcePath)
	catalog := m.catalogLocked()
	m.mu.Unlock()
	if err == nil {
		m.notify(catalog)
	}
	return result, err
}

func (m *Manager) installLocked(sourcePath string) (InstallResult, error) {
	if m.safeMode {
		return InstallResult{}, errors.New("skins cannot be installed in safe mode")
	}

	sourcePath = filepath.Clean(sourcePath)
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return InstallResult{}, fmt.Errorf("open skin package: %w", err)
	}
	if sourcePath == m.skinsDir || strings.HasPrefix(sourcePath, m.skinsDir+string(os.PathSeparator)) {
		return InstallResult{}, validationError("choose a skin outside the managed skins directory")
	}

	isDir := info.IsDir()
	if !isDir && (!info.Mode().IsRegular() || !isZipFile(sourcePath)) {
		return InstallResult{}, validationError("choose a skin directory or a .zip archive")
	}
	if isDir {
		if _, _, err := ValidateDir(sourcePath); err != nil {
			return InstallResult{}, err
		}
	}

	staging, err := os.MkdirTemp(m.skinsDir, ".staging-")
	if err != nil {
		return InstallResult{}, fmt.Errorf("create staging directory: %w", err)
	}
	defer removeIfExists(staging)

	if isDir {
		if err := copySkinDirectory(sourcePath, staging); err != nil {
			return InstallResult{}, err
		}
	} else {
		if err := extractZip(sourcePath, staging); err != nil {
			return InstallResult{}, err
		}
	}

	manifest, layout, err := ValidateDir(staging)
	if err != nil {
		return InstallResult{}, err
	}
	_ = layout

	destination := filepath.Join(m.skinsDir, manifest.ID)
	backup := ""
	if _, err := os.Lstat(destination); err == nil {
		backup, err = os.MkdirTemp(m.skinsDir, ".backup-"+manifest.ID+"-")
		if err != nil {
			return InstallResult{}, fmt.Errorf("prepare rollback: %w", err)
		}
		if err := os.Remove(backup); err != nil {
			return InstallResult{}, err
		}
		if err := os.Rename(destination, backup); err != nil {
			return InstallResult{}, fmt.Errorf("move previous skin aside: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return InstallResult{}, err
	}

	rollback := func(cause error) error {
		_ = os.RemoveAll(destination)
		if backup != "" {
			_ = os.Rename(backup, destination)
		}
		return cause
	}

	if err := os.Rename(staging, destination); err != nil {
		return InstallResult{}, rollback(fmt.Errorf("activate staged skin: %w", err))
	}
	if err := os.Chmod(destination, 0o700); err != nil {
		return InstallResult{}, rollback(err)
	}
	installed, err := postInstallLoader(destination, sourceInstalled)
	if err != nil {
		return InstallResult{}, rollback(fmt.Errorf("validate installed skin: %w", err))
	}
	if installed.manifest.ID != manifest.ID {
		return InstallResult{}, rollback(errors.New("installed skin id changed during installation"))
	}
	if backup != "" {
		_ = os.RemoveAll(backup)
	}

	m.installed[manifest.ID] = installed
	delete(m.develop, manifest.ID)
	m.lastError = ""
	if m.config.Skins.ActiveID == manifest.ID {
		m.config.Skins.ActiveTheme = themeOrDefault(
			m.config.Skins.ActiveTheme,
			installed.manifest.Themes,
			installed.manifest.DefaultTheme,
		)
		if m.config.Skins.LastKnownGoodID == manifest.ID {
			m.config.Skins.LastKnownGoodTheme = m.config.Skins.ActiveTheme
		}
		_ = m.storeSaveLocked()
	}
	return InstallResult{Skin: installed.summary, Catalog: m.catalogLocked()}, nil
}

// Uninstall removes a managed skin. The built-in and development skins cannot
// be removed through this API.
func (m *Manager) Uninstall(id string) (SkinCatalog, error) {
	m.mu.Lock()
	if id == BuiltinID {
		catalog := m.catalogLocked()
		m.mu.Unlock()
		return catalog, errors.New("the built-in skin cannot be uninstalled")
	}
	if _, ok := m.develop[id]; ok {
		catalog := m.catalogLocked()
		m.mu.Unlock()
		return catalog, errors.New("development skins cannot be uninstalled; remove the source directory instead")
	}
	if _, ok := m.installed[id]; !ok {
		catalog := m.catalogLocked()
		m.mu.Unlock()
		return catalog, fmt.Errorf("skin %q is not installed", id)
	}

	destination := filepath.Join(m.skinsDir, id)
	trash, err := os.MkdirTemp(m.skinsDir, ".uninstall-"+id+"-")
	if err != nil {
		catalog := m.catalogLocked()
		m.mu.Unlock()
		return catalog, err
	}
	if err := os.Remove(trash); err != nil {
		catalog := m.catalogLocked()
		m.mu.Unlock()
		return catalog, err
	}
	if err := os.Rename(destination, trash); err != nil {
		catalog := m.catalogLocked()
		m.mu.Unlock()
		return catalog, fmt.Errorf("remove skin directory: %w", err)
	}
	delete(m.installed, id)
	if m.config.Skins.ActiveID == id {
		m.config.Skins.ActiveID = BuiltinID
		m.config.Skins.ActiveTheme = "graphite"
		m.config.Skins.LastKnownGoodID = BuiltinID
		m.config.Skins.LastKnownGoodTheme = "graphite"
		m.lastError = ""
	}
	catalog := m.catalogLocked()
	saveErr := m.storeSaveLocked()
	m.mu.Unlock()
	_ = os.RemoveAll(trash)
	m.notify(catalog)
	return catalog, saveErr
}

// ActiveSkinDir returns the root directory of the active declarative skin.
// It returns false for the built-in skin.
func (m *Manager) ActiveSkinDir() (string, string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.safeMode || m.config.Skins.ActiveID == BuiltinID {
		return "", "", false
	}
	skin, ok := m.installed[m.config.Skins.ActiveID]
	if !ok {
		skin = m.develop[m.config.Skins.ActiveID]
	}
	if skin == nil {
		return "", "", false
	}
	return skin.summary.ID, skin.root, true
}

func (m *Manager) cleanupTransientLocked() {
	entries, err := os.ReadDir(m.skinsDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".staging-") || strings.HasPrefix(name, ".uninstall-") {
			_ = os.RemoveAll(filepath.Join(m.skinsDir, name))
		}
	}
}

func (m *Manager) resolveActiveLocked() {
	original := m.config.Skins.ActiveID
	if m.safeMode {
		m.config.Skins.ActiveID = BuiltinID
		m.config.Skins.ActiveTheme = "graphite"
		m.config.Skins.LastKnownGoodID = BuiltinID
		m.config.Skins.LastKnownGoodTheme = "graphite"
		return
	}
	if m.config.Skins.ThirdPartyDisabled {
		m.config.Skins.ActiveID = BuiltinID
		m.config.Skins.ActiveTheme = "graphite"
		m.config.Skins.LastKnownGoodID = BuiltinID
		m.config.Skins.LastKnownGoodTheme = "graphite"
		return
	}
	if m.skinExistsLocked(m.config.Skins.ActiveID) {
		themes, defaultTheme := m.themesForLocked(m.config.Skins.ActiveID)
		m.config.Skins.ActiveTheme = themeOrDefault(m.config.Skins.ActiveTheme, themes, defaultTheme)
		return
	}

	fallback := m.config.Skins.LastKnownGoodID
	if !m.skinExistsLocked(fallback) {
		fallback = BuiltinID
	}
	themes, defaultTheme := m.themesForLocked(fallback)
	m.config.Skins.ActiveID = fallback
	m.config.Skins.ActiveTheme = themeOrDefault(m.config.Skins.LastKnownGoodTheme, themes, defaultTheme)
	if original != "" && original != fallback {
		m.lastError = fmt.Sprintf("skin %q is unavailable; using %q", original, fallback)
	}
}

func (m *Manager) scanLocked() {
	if entries, err := os.ReadDir(m.skinsDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			root := filepath.Join(m.skinsDir, entry.Name())
			skin, err := loadSkin(root, sourceInstalled)
			if err != nil {
				m.warnings = append(m.warnings, fmt.Sprintf("installed skin %q: %v", entry.Name(), err))
				continue
			}
			if skin.manifest.ID != entry.Name() {
				m.warnings = append(m.warnings, fmt.Sprintf("installed skin %q: manifest id is %q", entry.Name(), skin.manifest.ID))
				continue
			}
			m.installed[skin.manifest.ID] = skin
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		m.warnings = append(m.warnings, "read skins directory: "+err.Error())
	}

	if m.devDir == "" {
		return
	}
	if entries, err := os.ReadDir(m.devDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			if _, exists := m.installed[entry.Name()]; exists {
				m.warnings = append(m.warnings, fmt.Sprintf("development skin %q is shadowed by an installed skin", entry.Name()))
				continue
			}
			root := filepath.Join(m.devDir, entry.Name())
			skin, err := loadSkin(root, sourceDevelopment)
			if err != nil {
				m.warnings = append(m.warnings, fmt.Sprintf("development skin %q: %v", entry.Name(), err))
				continue
			}
			if skin.manifest.ID != entry.Name() {
				m.warnings = append(m.warnings, fmt.Sprintf("development skin %q: manifest id is %q", entry.Name(), skin.manifest.ID))
				continue
			}
			m.develop[skin.manifest.ID] = skin
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		m.warnings = append(m.warnings, "read development skins directory: "+err.Error())
	}
}

func loadSkin(root, source string) (*managedSkin, error) {
	manifest, layout, err := ValidateDir(root)
	if err != nil {
		return nil, err
	}
	stylesheets := make(map[string]string)
	for _, theme := range manifest.Themes {
		relative := path.Join("themes", theme+".css")
		if info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(relative))); err == nil && info.Mode().IsRegular() {
			stylesheets[theme] = "/skins/" + manifest.ID + "/" + relative
		}
	}
	summary := SkinSummary{
		ID:              manifest.ID,
		Name:            manifest.Name,
		Version:         manifest.Version,
		Description:     manifest.Description,
		Author:          manifest.Author,
		License:         manifest.License,
		Source:          source,
		Builtin:         false,
		Removable:       source == sourceInstalled,
		Themes:          append([]string(nil), manifest.Themes...),
		DefaultTheme:    manifest.DefaultTheme,
		VisualizerModes: append([]string(nil), manifest.Visualizer.Modes...),
	}
	return &managedSkin{
		summary:     summary,
		manifest:    *manifest,
		layout:      *layout,
		root:        root,
		source:      source,
		stylesheets: stylesheets,
	}, nil
}

func builtinSummary(active bool) SkinSummary {
	return SkinSummary{
		ID:              BuiltinID,
		Name:            "Clarity",
		Version:         "1.0.0",
		Description:     "Original minimal futuristic skin for Cadence.",
		Author:          "Cadence",
		License:         "CC BY-SA 4.0",
		Source:          sourceBuiltin,
		Builtin:         true,
		Removable:       false,
		Active:          active,
		Themes:          append([]string(nil), builtinThemes...),
		DefaultTheme:    "graphite",
		VisualizerModes: append([]string(nil), builtinModes...),
	}
}

func builtinRuntime() SkinRuntime {
	return SkinRuntime{
		ID:                BuiltinID,
		Name:              "Clarity",
		Version:           "1.0.0",
		Author:            "Cadence",
		License:           "CC BY-SA 4.0",
		Description:       "Original minimal futuristic skin for Cadence.",
		Builtin:           true,
		Themes:            append([]string(nil), builtinThemes...),
		DefaultTheme:      "graphite",
		VisualizerModes:   append([]string(nil), builtinModes...),
		DefaultVisualizer: "spectrum",
		Intensity:         0.7,
		Layout:            Layout{SchemaVersion: 1},
		ThemeStylesheets:  map[string]string{},
		BaseHeight:        220,
		QueueHeight:       460,
	}
}

func runtimeFor(skin *managedSkin) SkinRuntime {
	stylesheets := make(map[string]string, len(skin.stylesheets))
	for theme, url := range skin.stylesheets {
		stylesheets[theme] = url
	}
	return SkinRuntime{
		ID:                skin.manifest.ID,
		Name:              skin.manifest.Name,
		Version:           skin.manifest.Version,
		Author:            skin.manifest.Author,
		License:           skin.manifest.License,
		Description:       skin.manifest.Description,
		Builtin:           false,
		Themes:            append([]string(nil), skin.manifest.Themes...),
		DefaultTheme:      skin.manifest.DefaultTheme,
		VisualizerModes:   append([]string(nil), skin.manifest.Visualizer.Modes...),
		DefaultVisualizer: skin.manifest.Visualizer.Default,
		Intensity:         skin.manifest.Visualizer.Intensity,
		Layout:            skin.layout,
		StylesheetURL:     "/skins/" + skin.manifest.ID + "/styles.css",
		ThemeStylesheets:  stylesheets,
		BaseHeight:        skin.manifest.Window.Height,
		QueueHeight:       skin.manifest.Window.QueueHeight,
	}
}

func (m *Manager) catalogLocked() SkinCatalog {
	skins := make([]SkinSummary, 0, 1+len(m.installed)+len(m.develop))
	skins = append(skins, builtinSummary(m.config.Skins.ActiveID == BuiltinID))
	for _, skin := range m.installed {
		summary := skin.summary
		summary.Active = m.config.Skins.ActiveID == summary.ID
		summary.Themes = append([]string(nil), skin.summary.Themes...)
		summary.VisualizerModes = append([]string(nil), skin.summary.VisualizerModes...)
		skins = append(skins, summary)
	}
	for _, skin := range m.develop {
		if _, exists := m.installed[skin.summary.ID]; exists {
			continue
		}
		summary := skin.summary
		summary.Active = m.config.Skins.ActiveID == summary.ID
		summary.Themes = append([]string(nil), skin.summary.Themes...)
		summary.VisualizerModes = append([]string(nil), skin.summary.VisualizerModes...)
		skins = append(skins, summary)
	}
	sort.Slice(skins, func(i, j int) bool {
		if skins[i].Builtin != skins[j].Builtin {
			return skins[i].Builtin
		}
		if skins[i].Name != skins[j].Name {
			return skins[i].Name < skins[j].Name
		}
		return skins[i].ID < skins[j].ID
	})

	return SkinCatalog{
		Skins:               skins,
		ActiveID:            m.config.Skins.ActiveID,
		ActiveTheme:         m.config.Skins.ActiveTheme,
		SafeMode:            m.safeMode,
		ThirdPartyDisabled:  m.effectiveThirdPartyDisabledLocked(),
		Warnings:            append([]string(nil), m.warnings...),
		LastError:           m.lastError,
		CanInstall:          !m.safeMode,
		CanChangeActiveSkin: !m.safeMode && !m.effectiveThirdPartyDisabledLocked(),
	}
}

func (m *Manager) activeThemesLocked() ([]string, string) {
	return m.themesForLocked(m.config.Skins.ActiveID)
}

func (m *Manager) themesForLocked(id string) ([]string, string) {
	if id == BuiltinID {
		return append([]string(nil), builtinThemes...), "graphite"
	}
	if skin, ok := m.installed[id]; ok {
		return append([]string(nil), skin.manifest.Themes...), skin.manifest.DefaultTheme
	}
	if skin, ok := m.develop[id]; ok {
		return append([]string(nil), skin.manifest.Themes...), skin.manifest.DefaultTheme
	}
	return append([]string(nil), builtinThemes...), "graphite"
}

func (m *Manager) skinExistsLocked(id string) bool {
	if id == BuiltinID {
		return true
	}
	if _, ok := m.installed[id]; ok {
		return true
	}
	_, ok := m.develop[id]
	return ok
}

func (m *Manager) effectiveThirdPartyDisabledLocked() bool {
	return m.safeMode || m.config.Skins.ThirdPartyDisabled
}

func (m *Manager) storeSaveLocked() error {
	// Safe mode is a session-only diagnostic state. It must not overwrite the
	// user's persisted active skin or theme while the app is running.
	if m.safeMode {
		return nil
	}
	if err := m.store.Save(m.config); err != nil {
		return fmt.Errorf("save skin settings: %w", err)
	}
	return nil
}

func (m *Manager) notify(catalog SkinCatalog) {
	m.mu.Lock()
	emit := m.emit
	m.mu.Unlock()
	if emit != nil {
		emit(catalog)
	}
}

func normalizeConfig(config settings.Config) settings.Config {
	defaults := settings.Default()
	if config.SchemaVersion == 0 {
		config.SchemaVersion = settings.SchemaVersion
	}
	if config.Skins.ActiveID == "" {
		config.Skins.ActiveID = defaults.Skins.ActiveID
	}
	if config.Skins.ActiveTheme == "" {
		config.Skins.ActiveTheme = defaults.Skins.ActiveTheme
	}
	if config.Skins.LastKnownGoodID == "" {
		config.Skins.LastKnownGoodID = defaults.Skins.LastKnownGoodID
	}
	if config.Skins.LastKnownGoodTheme == "" {
		config.Skins.LastKnownGoodTheme = defaults.Skins.LastKnownGoodTheme
	}
	return config
}

func themeOrDefault(candidate string, themes []string, fallback string) string {
	if candidate != "" && contains(themes, candidate) {
		return candidate
	}
	if fallback != "" && contains(themes, fallback) {
		return fallback
	}
	if len(themes) > 0 {
		return themes[0]
	}
	return "graphite"
}
