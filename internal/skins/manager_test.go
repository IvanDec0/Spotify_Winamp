package skins

import (
	"archive/zip"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cadence/internal/settings"
)

func newTestManager(t *testing.T, safeMode bool, devDir string) (*Manager, string) {
	t.Helper()
	base := t.TempDir()
	skinsDir := filepath.Join(base, "skins")
	manager, err := NewManager(Options{
		SkinsDir: skinsDir,
		DevDir:   devDir,
		Store:    settings.NewStore(filepath.Join(base, "config.json")),
		SafeMode: safeMode,
	})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	return manager, skinsDir
}

func TestManagerInstallActivateThemeAndUninstall(t *testing.T) {
	manager, skinsDir := newTestManager(t, false, "")
	source := filepath.Join(t.TempDir(), "afterglow")
	writeSkinPackage(t, source, "afterglow")
	if err := os.MkdirAll(filepath.Join(source, "themes"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(source, "themes", "ember.css"), ":host { --accent: #ffb347; }\n")

	catalog := manager.Catalog()
	if len(catalog.Skins) != 1 || !catalog.Skins[0].Builtin {
		t.Fatalf("initial catalog = %#v", catalog)
	}

	result, err := manager.Install(source)
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if result.Skin.ID != "afterglow" || len(result.Catalog.Skins) != 2 {
		t.Fatalf("install result = %#v", result)
	}
	if _, err := os.Stat(filepath.Join(skinsDir, "afterglow", "manifest.json")); err != nil {
		t.Fatalf("installed manifest: %v", err)
	}

	catalog, err = manager.Activate("afterglow")
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	if catalog.ActiveID != "afterglow" || catalog.ActiveTheme != "ember" {
		t.Fatalf("catalog active = %q theme = %q", catalog.ActiveID, catalog.ActiveTheme)
	}

	runtime, err := manager.ActiveRuntime()
	if err != nil {
		t.Fatalf("ActiveRuntime() error = %v", err)
	}
	if runtime.Builtin || runtime.Layout.Root.Type != "column" {
		t.Fatalf("runtime = %#v", runtime)
	}
	if runtime.StylesheetURL != "/skins/afterglow/styles.css" {
		t.Fatalf("stylesheet URL = %q", runtime.StylesheetURL)
	}
	if runtime.ThemeStylesheets["ember"] != "/skins/afterglow/themes/ember.css" {
		t.Fatalf("theme stylesheets = %#v", runtime.ThemeStylesheets)
	}

	if _, err := manager.SetTheme("violet"); err == nil {
		t.Fatal("SetTheme(violet) error = nil")
	}
	catalog, err = manager.SetTheme("tide")
	if err != nil || catalog.ActiveTheme != "tide" {
		t.Fatalf("SetTheme(tide) = %#v, %v", catalog, err)
	}

	if err := manager.ReportRuntimeReady("afterglow", "tide"); err != nil {
		t.Fatalf("ReportRuntimeReady() error = %v", err)
	}

	catalog, err = manager.Uninstall("afterglow")
	if err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}
	if catalog.ActiveID != BuiltinID || len(catalog.Skins) != 1 {
		t.Fatalf("catalog after uninstall = %#v", catalog)
	}
	if _, err := os.Stat(filepath.Join(skinsDir, "afterglow")); !os.IsNotExist(err) {
		t.Fatalf("skin directory still exists: %v", err)
	}
}

func TestManagerInstallEmitsCatalog(t *testing.T) {
	manager, _ := newTestManager(t, false, "")
	source := filepath.Join(t.TempDir(), "afterglow")
	writeSkinPackage(t, source, "afterglow")
	emitted := 0
	var last SkinCatalog
	manager.SetEmitter(func(catalog SkinCatalog) {
		emitted++
		last = catalog
	})
	if _, err := manager.Install(source); err != nil {
		t.Fatal(err)
	}
	if emitted != 1 {
		t.Fatalf("emitted %d catalogs, want 1", emitted)
	}
	if len(last.Skins) != 2 || last.ActiveID != BuiltinID {
		t.Fatalf("emitted catalog = %#v", last)
	}
}

func TestManagerRollbackRestoresPreviousSkin(t *testing.T) {
	manager, skinsDir := newTestManager(t, false, "")
	source := filepath.Join(t.TempDir(), "afterglow")
	writeSkinPackage(t, source, "afterglow")
	if _, err := manager.Install(source); err != nil {
		t.Fatal(err)
	}

	replacement := filepath.Join(t.TempDir(), "afterglow-v2")
	writeSkinPackage(t, replacement, "afterglow")
	manifestPath := filepath.Join(replacement, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, []byte(strings.Replace(string(data), `"version": "1.0.0"`, `"version": "2.0.0"`, 1)), 0o600); err != nil {
		t.Fatal(err)
	}

	originalLoader := postInstallLoader
	postInstallLoader = func(string, string) (*managedSkin, error) {
		return nil, errors.New("forced post-install failure")
	}
	defer func() { postInstallLoader = originalLoader }()

	if _, err := manager.Install(replacement); err == nil {
		t.Fatal("Install() error = nil")
	}
	manifest, _, err := ValidateDir(filepath.Join(skinsDir, "afterglow"))
	if err != nil {
		t.Fatalf("previous skin was not restored: %v", err)
	}
	if manifest.Version != "1.0.0" {
		t.Fatalf("restored version = %q", manifest.Version)
	}
}

func TestManagerKeepsExistingSkinWhenReplacementIsInvalid(t *testing.T) {
	manager, skinsDir := newTestManager(t, false, "")
	source := filepath.Join(t.TempDir(), "afterglow")
	writeSkinPackage(t, source, "afterglow")
	if _, err := manager.Install(source); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	invalid := filepath.Join(t.TempDir(), "afterglow-invalid")
	writeSkinPackage(t, invalid, "afterglow")
	writeFile(t, filepath.Join(invalid, "layout.json"), `{"schemaVersion":1,"root":{"type":"column","children":[{"type":"component","component":"unknown"}]}}`)

	if _, err := manager.Install(invalid); err == nil {
		t.Fatal("Install(invalid) error = nil")
	}
	if _, err := os.Stat(filepath.Join(skinsDir, "afterglow", "layout.json")); err != nil {
		t.Fatalf("existing skin was removed: %v", err)
	}
	manifest, _, err := ValidateDir(filepath.Join(skinsDir, "afterglow"))
	if err != nil || manifest.ID != "afterglow" {
		t.Fatalf("existing skin invalid: %v", err)
	}
}

func TestManagerSafeMode(t *testing.T) {
	base := t.TempDir()
	skinsDir := filepath.Join(base, "skins")
	configPath := filepath.Join(base, "config.json")
	store := settings.NewStore(configPath)
	config := settings.Default()
	config.Skins.ActiveID = "afterglow"
	config.Skins.ActiveTheme = "ember"
	if err := store.Save(config); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(base, "afterglow-source")
	writeSkinPackage(t, source, "afterglow")
	if err := os.MkdirAll(skinsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(source, filepath.Join(skinsDir, "afterglow")); err != nil {
		t.Fatal(err)
	}

	manager, err := NewManager(Options{
		SkinsDir: skinsDir,
		Store:    store,
		SafeMode: true,
	})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	catalog := manager.Catalog()
	if catalog.ActiveID != BuiltinID || !catalog.SafeMode || !catalog.ThirdPartyDisabled {
		t.Fatalf("safe-mode catalog = %#v", catalog)
	}
	if catalog.CanChangeActiveSkin {
		t.Fatal("CanChangeActiveSkin = true in safe mode")
	}
	if _, err := manager.Activate("afterglow"); err == nil {
		t.Fatal("Activate() in safe mode error = nil")
	}
	if _, err := manager.Install(source); err == nil {
		t.Fatal("Install() in safe mode error = nil")
	}
	runtime, err := manager.ActiveRuntime()
	if err != nil || !runtime.Builtin {
		t.Fatalf("ActiveRuntime() = %#v, %v", runtime, err)
	}
	persisted, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Skins.ActiveID != "afterglow" || persisted.Skins.ActiveTheme != "ember" {
		t.Fatalf("safe mode overwrote persisted skin state: %#v", persisted.Skins)
	}
}

func TestManagerFallsBackWhenActiveSkinIsBroken(t *testing.T) {
	base := t.TempDir()
	skinsDir := filepath.Join(base, "skins")
	configPath := filepath.Join(base, "config.json")
	store := settings.NewStore(configPath)
	config := settings.Default()
	config.Skins.ActiveID = "afterglow"
	config.Skins.ActiveTheme = "ember"
	if err := store.Save(config); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(skinsDir, "afterglow"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(skinsDir, "afterglow", "manifest.json"), "{broken")

	manager, err := NewManager(Options{SkinsDir: skinsDir, Store: store})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	catalog := manager.Catalog()
	if catalog.ActiveID != BuiltinID {
		t.Fatalf("active = %q, want clarity", catalog.ActiveID)
	}
	if catalog.LastError == "" {
		t.Fatal("LastError is empty")
	}
	if len(catalog.Warnings) == 0 {
		t.Fatal("Warnings is empty")
	}
}

func TestManagerRuntimeFailureRollsBackToLastKnownGood(t *testing.T) {
	manager, _ := newTestManager(t, false, "")

	sourceA := filepath.Join(t.TempDir(), "afterglow")
	writeSkinPackage(t, sourceA, "afterglow")
	if _, err := manager.Install(sourceA); err != nil {
		t.Fatal(err)
	}
	sourceB := filepath.Join(t.TempDir(), "aurora")
	writeSkinPackage(t, sourceB, "aurora")
	if _, err := manager.Install(sourceB); err != nil {
		t.Fatal(err)
	}

	if _, err := manager.Activate("afterglow"); err != nil {
		t.Fatal(err)
	}
	if err := manager.ReportRuntimeReady("afterglow", "ember"); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Activate("aurora"); err != nil {
		t.Fatal(err)
	}
	catalog, err := manager.ReportRuntimeFailure("aurora", "mount failed")
	if err != nil {
		t.Fatal(err)
	}
	if catalog.ActiveID != "afterglow" || catalog.ActiveTheme != "ember" {
		t.Fatalf("fallback = %q/%q", catalog.ActiveID, catalog.ActiveTheme)
	}
	if catalog.LastError == "" {
		t.Fatal("LastError is empty")
	}
}

func TestManagerThirdPartyDisabledFallsBackToBuiltin(t *testing.T) {
	manager, _ := newTestManager(t, false, "")
	source := filepath.Join(t.TempDir(), "afterglow")
	writeSkinPackage(t, source, "afterglow")
	if _, err := manager.Install(source); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Activate("afterglow"); err != nil {
		t.Fatal(err)
	}
	catalog, err := manager.SetThirdPartyDisabled(true)
	if err != nil {
		t.Fatal(err)
	}
	if catalog.ActiveID != BuiltinID || !catalog.ThirdPartyDisabled {
		t.Fatalf("catalog = %#v", catalog)
	}
	if _, err := manager.Activate("afterglow"); err == nil {
		t.Fatal("Activate() while disabled error = nil")
	}
}

func TestManagerThirdPartyDisabledAtStartupUsesBuiltin(t *testing.T) {
	base := t.TempDir()
	skinsDir := filepath.Join(base, "skins")
	store := settings.NewStore(filepath.Join(base, "config.json"))
	config := settings.Default()
	config.Skins.ActiveID = "afterglow"
	config.Skins.ActiveTheme = "ember"
	config.Skins.ThirdPartyDisabled = true
	if err := store.Save(config); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(base, "afterglow-source")
	writeSkinPackage(t, source, "afterglow")
	if err := os.MkdirAll(skinsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(source, filepath.Join(skinsDir, "afterglow")); err != nil {
		t.Fatal(err)
	}

	manager, err := NewManager(Options{SkinsDir: skinsDir, Store: store})
	if err != nil {
		t.Fatal(err)
	}
	catalog := manager.Catalog()
	if catalog.ActiveID != BuiltinID || catalog.ActiveTheme != "graphite" {
		t.Fatalf("catalog = %#v", catalog)
	}
	if catalog.CanChangeActiveSkin {
		t.Fatal("CanChangeActiveSkin = true while third-party skins are disabled")
	}
}

func TestManagerDevelopmentSkin(t *testing.T) {
	devDir := filepath.Join(t.TempDir(), "skins-dev")
	if err := os.MkdirAll(devDir, 0o700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(devDir, "afterglow")
	writeSkinPackage(t, source, "afterglow")

	manager, _ := newTestManager(t, false, devDir)
	catalog := manager.Catalog()
	if len(catalog.Skins) != 2 || catalog.Skins[1].Source != sourceDevelopment || catalog.Skins[1].Removable {
		t.Fatalf("catalog = %#v", catalog)
	}
	if _, err := manager.Activate("afterglow"); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Uninstall("afterglow"); err == nil {
		t.Fatal("Uninstall(development) error = nil")
	}
}

func TestManagerServesOnlyActiveSkinAssets(t *testing.T) {
	manager, _ := newTestManager(t, false, "")
	source := filepath.Join(t.TempDir(), "afterglow")
	writeSkinPackage(t, source, "afterglow")
	if _, err := manager.Install(source); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/skins/afterglow/styles.css", nil)
	response := httptest.NewRecorder()
	manager.ServeAsset(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("inactive skin status = %d", response.Code)
	}

	if _, err := manager.Activate("afterglow"); err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodGet, "/skins/afterglow/styles.css", nil)
	response = httptest.NewRecorder()
	manager.ServeAsset(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("styles status = %d", response.Code)
	}
	if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/css") {
		t.Fatalf("content type = %q", contentType)
	}

	request = httptest.NewRequest(http.MethodGet, "/skins/afterglow/assets/bg.png", nil)
	response = httptest.NewRecorder()
	manager.ServeAsset(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("asset status = %d", response.Code)
	}

	for _, target := range []string{
		"/skins/afterglow/manifest.json",
		"/skins/afterglow/../manifest.json",
		"/skins/afterglow/assets/%2e%2e/%2e%2e/manifest.json",
		"/skins/other/styles.css",
	} {
		request = httptest.NewRequest(http.MethodGet, target, nil)
		response = httptest.NewRecorder()
		manager.ServeAsset(response, request)
		if response.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d", target, response.Code)
		}
	}
}

func TestExtractZipValidAndInstall(t *testing.T) {
	source := filepath.Join(t.TempDir(), "afterglow")
	writeSkinPackage(t, source, "afterglow")
	archive := filepath.Join(t.TempDir(), "afterglow.zip")
	writeDirectoryZip(t, archive, source)

	manager, _ := newTestManager(t, false, "")
	result, err := manager.Install(archive)
	if err != nil {
		t.Fatalf("Install(zip) error = %v", err)
	}
	if result.Skin.ID != "afterglow" {
		t.Fatalf("installed id = %q", result.Skin.ID)
	}
}

func TestExtractZipRejectsUnsafeArchives(t *testing.T) {
	tests := []struct {
		name    string
		entries []zipTestEntry
		want    string
	}{
		{
			name:    "traversal",
			entries: []zipTestEntry{{Name: "../evil.txt", Body: "x"}},
			want:    "escapes the skin directory",
		},
		{
			name:    "absolute",
			entries: []zipTestEntry{{Name: "/evil.txt", Body: "x"}},
			want:    "absolute",
		},
		{
			name: "duplicate",
			entries: []zipTestEntry{
				{Name: "manifest.json", Body: "{}"},
				{Name: "manifest.json", Body: "{}"},
			},
			want: "duplicate",
		},
		{
			name: "symlink",
			entries: []zipTestEntry{{
				Name: "link",
				Body: "/etc/passwd",
				Mode: os.ModeSymlink | 0o777,
			}},
			want: "symbolic links",
		},
		{
			name: "nested archive",
			entries: []zipTestEntry{
				{Name: "manifest.json", Body: "{}"},
				{Name: "inner.zip", Body: "PK"},
			},
			want: "unsupported file type",
		},
		{
			name: "bomb ratio",
			entries: []zipTestEntry{
				{Name: "manifest.json", Body: "{}"},
				{Name: "assets/big.png", Body: strings.Repeat("0", 1<<20)},
			},
			want: "compression ratio",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			archive := filepath.Join(t.TempDir(), "skin.zip")
			writeTestZip(t, archive, test.entries)
			destination := t.TempDir()
			err := extractZip(archive, destination)
			if err == nil {
				t.Fatal("extractZip() error = nil")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %q, want containing %q", err, test.want)
			}
		})
	}
}

type zipTestEntry struct {
	Name string
	Body string
	Mode os.FileMode
}

func writeTestZip(t *testing.T, path string, entries []zipTestEntry) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.Name, Method: zip.Deflate}
		if entry.Mode != 0 {
			header.SetMode(entry.Mode)
		}
		part, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(part, entry.Body); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeDirectoryZip(t *testing.T, archive, directory string) {
	t.Helper()
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	err = filepath.WalkDir(directory, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(directory, current)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		header := &zip.FileHeader{Name: filepath.ToSlash(relative), Method: zip.Deflate}
		header.SetMode(info.Mode())
		header.Modified = info.ModTime()
		part, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		input, err := os.Open(current)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(part, input)
		closeErr := input.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
