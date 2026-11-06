package audioapi

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"cadence/internal/catalog"
	"cadence/internal/engine"
	"cadence/internal/settings"
	"cadence/internal/soloist"
)

func TestEngineSettingsDefaultsAndPersist(t *testing.T) {
	store := settings.NewStore(filepath.Join(t.TempDir(), "config.json"))
	supervisor := engine.NewSupervisor(engine.Options{})
	service := NewService(Options{Supervisor: supervisor, Settings: store})

	defaults, err := service.GetEngineSettings()
	if err != nil {
		t.Fatalf("GetEngineSettings() error = %v", err)
	}
	if defaults.Bitrate != 320 || defaults.CacheSizeLimit != "1GB" || defaults.ImageSize != "default" {
		t.Fatalf("default engine settings = %#v", defaults)
	}

	if err := store.UpdateSkins(func(state *settings.SkinState) error {
		state.ActiveID = "afterglow"
		state.ActiveTheme = "ember"
		return nil
	}); err != nil {
		t.Fatalf("UpdateSkins() error = %v", err)
	}

	saved, err := service.SetEngineSettings(EngineSettings{
		Bitrate:                   128,
		CrossfadeDurationMS:       12001,
		NormalisationDisabled:     true,
		NormalisationUseAlbumGain: true,
		NormalisationPregain:      21,
		CacheEnabled:              true,
		CacheDir:                  "/tmp/cadence-audio-cache",
		CacheSizeLimit:            "3GB",
		FlacEnabled:               true,
		MPRISEnabled:              true,
		DeviceIsGroup:             true,
		ImageSize:                 "huge",
	})
	if err != nil {
		t.Fatalf("SetEngineSettings() error = %v", err)
	}
	want := EngineSettings{
		Bitrate:                   320,
		CrossfadeDurationMS:       0,
		NormalisationDisabled:     true,
		NormalisationUseAlbumGain: true,
		NormalisationPregain:      0,
		CacheEnabled:              true,
		CacheDir:                  "/tmp/cadence-audio-cache",
		CacheSizeLimit:            "1GB",
		FlacEnabled:               true,
		MPRISEnabled:              true,
		DeviceIsGroup:             true,
		ImageSize:                 "default",
	}
	if saved != want {
		t.Fatalf("saved = %#v, want %#v", saved, want)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Skins.ActiveID != "afterglow" || loaded.Skins.ActiveTheme != "ember" {
		t.Fatalf("engine update overwrote skins: %#v", loaded.Skins)
	}
	if loaded.Engine == nil || *loaded.Engine != want {
		t.Fatalf("persisted engine = %#v, want %#v", loaded.Engine, want)
	}

	after, err := service.GetEngineSettings()
	if err != nil {
		t.Fatalf("GetEngineSettings() after set error = %v", err)
	}
	if after != want {
		t.Fatalf("GetEngineSettings() = %#v, want %#v", after, want)
	}
	if supervisor.Client() != nil {
		t.Fatal("SetEngineSettings started or replaced the engine client")
	}
}

func TestEngineSettingsRequireStore(t *testing.T) {
	service := NewService(Options{Supervisor: engine.NewSupervisor(engine.Options{})})
	if _, err := service.GetEngineSettings(); err == nil {
		t.Fatal("GetEngineSettings() error = nil")
	}
	if _, err := service.SetEngineSettings(EngineSettings{}); err == nil {
		t.Fatal("SetEngineSettings() error = nil")
	}
}

// fakeSoloistBackend is a mutable, call-recording SoloistBackend for adapter
// tests. It never touches the filesystem or a process.
type fakeSoloistBackend struct {
	mu sync.Mutex

	state      soloist.State
	startErr   error
	stopErr    error
	restartErr error
	installErr error

	installVersion    string
	savedKey          string
	hasKey            bool
	due               bool
	autoUpdated       bool
	autoUpdateErr     error
	autoUpdateVersion string

	startCalls        int
	stopCalls         int
	restartCalls      int
	installCalls      int
	progressCalls     int
	updateDueCalls    int
	autoUpdateCalls   int
	autoProgressCalls int
	pruneCalls        int

	onProgress func()
	emitter    func(soloist.State)
}

func (f *fakeSoloistBackend) State() soloist.State {
	f.mu.Lock()
	defer f.mu.Unlock()
	state := f.state
	state.Accounts = append([]string(nil), f.state.Accounts...)
	return state
}

func (f *fakeSoloistBackend) Start(context.Context) error {
	f.mu.Lock()
	f.startCalls++
	err := f.startErr
	f.mu.Unlock()
	return err
}

func (f *fakeSoloistBackend) Stop(context.Context) error {
	f.mu.Lock()
	f.stopCalls++
	err := f.stopErr
	f.state.Running = false
	f.mu.Unlock()
	return err
}

func (f *fakeSoloistBackend) Restart(context.Context) error {
	f.mu.Lock()
	f.restartCalls++
	err := f.restartErr
	f.mu.Unlock()
	return err
}

func (f *fakeSoloistBackend) Install(_ context.Context, progress func(soloist.Progress)) (*soloist.Binary, error) {
	f.mu.Lock()
	f.installCalls++
	err := f.installErr
	version := f.installVersion
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}

	f.mu.Lock()
	f.progressCalls++
	f.mu.Unlock()
	progress(soloist.Progress{Stage: "downloading", Downloaded: 10, Total: 100})
	if f.onProgress != nil {
		f.onProgress()
	}
	f.mu.Lock()
	f.progressCalls++
	f.mu.Unlock()
	progress(soloist.Progress{Stage: "downloading", Downloaded: 100, Total: 100})

	f.mu.Lock()
	f.state.Installed = true
	f.state.Version = version
	f.mu.Unlock()
	return &soloist.Binary{Path: "/tmp/soloist", Version: version}, nil
}

func (f *fakeSoloistBackend) UpdateDue(time.Time) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updateDueCalls++
	return f.due
}

func (f *fakeSoloistBackend) AutoUpdate(_ context.Context, progress func(soloist.Progress)) (*soloist.Binary, bool, error) {
	f.mu.Lock()
	f.autoUpdateCalls++
	err := f.autoUpdateErr
	updated := f.autoUpdated
	version := f.autoUpdateVersion
	if version == "" {
		version = f.installVersion
	}
	// A successful auto-update installs the build; a running process keeps
	// running until the adapter restarts it.
	f.state.Installed = true
	if version != "" {
		f.state.Version = version
	}
	f.mu.Unlock()

	if err != nil {
		return nil, false, err
	}
	if progress != nil {
		f.mu.Lock()
		f.autoProgressCalls++
		f.mu.Unlock()
		progress(soloist.Progress{Stage: "downloading", Downloaded: 10, Total: 100})
		if f.onProgress != nil {
			f.onProgress()
		}
		f.mu.Lock()
		f.autoProgressCalls++
		f.mu.Unlock()
		progress(soloist.Progress{Stage: "installing", Downloaded: 100, Total: 100})
	}
	return &soloist.Binary{Path: "/tmp/soloist", Version: version}, updated, nil
}

func (f *fakeSoloistBackend) Prune() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pruneCalls++
	return nil
}

func (f *fakeSoloistBackend) SaveAPIKey(key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.savedKey = key
	f.hasKey = key != ""
	return nil
}

func (f *fakeSoloistBackend) ClearAPIKey() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.savedKey = ""
	f.hasKey = false
	return nil
}

func (f *fakeSoloistBackend) HasAPIKey() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hasKey
}

func (f *fakeSoloistBackend) SetEmitter(emit func(soloist.State)) {
	f.mu.Lock()
	f.emitter = emit
	f.mu.Unlock()
}

func newTestService(t *testing.T, backend string, fake *fakeSoloistBackend) (*Service, *settings.Store) {
	t.Helper()
	store := settings.NewStore(filepath.Join(t.TempDir(), "config.json"))
	supervisor := engine.NewSupervisor(engine.Options{})
	catalogService := catalog.NewService(catalog.Options{Supervisor: supervisor})
	var soloistBackend SoloistBackend
	if fake != nil {
		soloistBackend = fake
	}
	service := NewService(Options{
		Catalog:         catalogService,
		Supervisor:      supervisor,
		Settings:        store,
		Soloist:         soloistBackend,
		PlaybackBackend: backend,
	})
	return service, store
}

func TestPlaybackSettingsDefaultsAndPersist(t *testing.T) {
	service, store := newTestService(t, settings.BackendLibrespot, nil)

	defaults, err := service.GetPlaybackSettings()
	if err != nil {
		t.Fatalf("GetPlaybackSettings() error = %v", err)
	}
	if defaults.Backend != settings.BackendSoloist {
		t.Fatalf("default backend = %q, want %q", defaults.Backend, settings.BackendSoloist)
	}

	if err := store.UpdateSkins(func(state *settings.SkinState) error {
		state.ActiveID = "afterglow"
		state.ActiveTheme = "ember"
		return nil
	}); err != nil {
		t.Fatalf("UpdateSkins() error = %v", err)
	}
	if err := store.UpdateEngine(func(state *settings.EngineState) error {
		state.Bitrate = 96
		state.CacheEnabled = true
		return nil
	}); err != nil {
		t.Fatalf("UpdateEngine() error = %v", err)
	}

	saved, err := service.SetPlaybackSettings(PlaybackSettings{Backend: " SOLOIST "})
	if err != nil {
		t.Fatalf("SetPlaybackSettings() error = %v", err)
	}
	if saved.Backend != settings.BackendSoloist {
		t.Fatalf("saved backend = %q, want %q", saved.Backend, settings.BackendSoloist)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Playback == nil || loaded.Playback.Backend != settings.BackendSoloist {
		t.Fatalf("persisted playback = %#v", loaded.Playback)
	}
	if loaded.Skins.ActiveID != "afterglow" || loaded.Skins.ActiveTheme != "ember" {
		t.Fatalf("playback update overwrote skins: %#v", loaded.Skins)
	}
	if loaded.Engine == nil || loaded.Engine.Bitrate != 96 || !loaded.Engine.CacheEnabled {
		t.Fatalf("playback update overwrote engine: %#v", loaded.Engine)
	}

	after, err := service.GetPlaybackSettings()
	if err != nil {
		t.Fatalf("GetPlaybackSettings() after set error = %v", err)
	}
	if after.Backend != settings.BackendSoloist {
		t.Fatalf("GetPlaybackSettings() = %q, want %q", after.Backend, settings.BackendSoloist)
	}

	// Saving the setting must not switch the already running backend.
	if got := service.GetState().PlaybackBackend; got != settings.BackendLibrespot {
		t.Fatalf("running backend changed to %q", got)
	}
}

func TestPlaybackSettingsNormalizesUnknownBackend(t *testing.T) {
	service, _ := newTestService(t, settings.BackendLibrespot, nil)

	saved, err := service.SetPlaybackSettings(PlaybackSettings{Backend: "pulse"})
	if err != nil {
		t.Fatalf("SetPlaybackSettings() error = %v", err)
	}
	if saved.Backend != settings.BackendSoloist {
		t.Fatalf("saved backend = %q, want %q", saved.Backend, settings.BackendSoloist)
	}
	if got, err := service.GetPlaybackSettings(); err != nil || got.Backend != settings.BackendSoloist {
		t.Fatalf("GetPlaybackSettings() = %#v, %v", got, err)
	}
}

func TestPlaybackSettingsRequireStore(t *testing.T) {
	service := NewService(Options{Supervisor: engine.NewSupervisor(engine.Options{})})
	if _, err := service.GetPlaybackSettings(); err == nil {
		t.Fatal("GetPlaybackSettings() error = nil")
	}
	if _, err := service.SetPlaybackSettings(PlaybackSettings{}); err == nil {
		t.Fatal("SetPlaybackSettings() error = nil")
	}
}

func TestGetStateMergesSoloistState(t *testing.T) {
	builtAt := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	expiresAt := builtAt.Add(90 * 24 * time.Hour)
	fake := &fakeSoloistBackend{state: soloist.State{
		Installed:    true,
		BinaryPath:   "/data/soloist",
		Version:      "1.3.10.12",
		BuiltAt:      builtAt,
		ExpiresAt:    expiresAt,
		Expired:      false,
		DaysLeft:     78,
		Running:      true,
		LoggedIn:     false,
		NeedsPairing: true,
		Active:       false,
		DeviceName:   "Cadence",
		Accounts:     []string{"listener@example.com"},
		ErrorCode:    soloist.CodePairingRequired,
	}, hasKey: true}
	service, _ := newTestService(t, settings.BackendSoloist, fake)

	state := service.GetState()
	if state.PlaybackBackend != settings.BackendSoloist {
		t.Fatalf("PlaybackBackend = %q, want %q", state.PlaybackBackend, settings.BackendSoloist)
	}
	if state.Soloist == nil {
		t.Fatal("Soloist = nil in Soloist mode")
	}
	if state.Soloist.Version != "1.3.10.12" || !state.Soloist.Installed || !state.Soloist.Running {
		t.Fatalf("Soloist = %#v", state.Soloist)
	}
	if state.Soloist.BuiltAt != builtAt.Format(time.RFC3339) {
		t.Fatalf("BuiltAt = %q", state.Soloist.BuiltAt)
	}
	if state.Soloist.ExpiresAt != expiresAt.Format(time.RFC3339) {
		t.Fatalf("ExpiresAt = %q", state.Soloist.ExpiresAt)
	}
	if !state.Soloist.NeedsPairing || state.Soloist.ErrorCode != soloist.CodePairingRequired {
		t.Fatalf("pairing state = %#v", state.Soloist)
	}
	if len(state.Soloist.Accounts) != 1 || state.Soloist.Accounts[0] != "listener@example.com" {
		t.Fatalf("Accounts = %#v", state.Soloist.Accounts)
	}
	if !state.Soloist.APIKeySet {
		t.Fatal("APIKeySet = false, want true")
	}
}

func TestGetStateIncludesSoloistStateInLibrespotMode(t *testing.T) {
	fake := &fakeSoloistBackend{state: soloist.State{Installed: true, Version: "1.3.10.12"}}
	service, _ := newTestService(t, settings.BackendLibrespot, fake)

	state := service.GetState()
	if state.PlaybackBackend != settings.BackendLibrespot {
		t.Fatalf("PlaybackBackend = %q, want %q", state.PlaybackBackend, settings.BackendLibrespot)
	}
	if state.Soloist == nil {
		t.Fatal("Soloist state is nil in librespot mode; Settings needs it to install and store the key before switching")
	}
	if state.Soloist.Version != "1.3.10.12" {
		t.Fatalf("Soloist.Version = %q", state.Soloist.Version)
	}
}

func TestMergeStateCarriesBackendAndSoloist(t *testing.T) {
	fake := &fakeSoloistBackend{state: soloist.State{
		Installed: true,
		Version:   "1.3.10.12",
		Running:   true,
		Accounts:  []string{"listener@example.com"},
	}}
	for _, backend := range []string{settings.BackendSoloist, settings.BackendLibrespot} {
		t.Run(backend, func(t *testing.T) {
			service, _ := newTestService(t, backend, fake)
			// catalog.ResetSession and catalog.Restart return a state without
			// either field; mergeState must fill them in on its own.
			merged := service.mergeState(AudioState{Engine: "go-librespot"})
			if merged.PlaybackBackend != backend {
				t.Fatalf("PlaybackBackend = %q, want %q", merged.PlaybackBackend, backend)
			}
			if merged.Soloist == nil || merged.Soloist.Version != "1.3.10.12" {
				t.Fatalf("Soloist = %#v, want the backend state in both modes", merged.Soloist)
			}
			if len(merged.Soloist.Accounts) != 1 || merged.Soloist.Accounts[0] != "listener@example.com" {
				t.Fatalf("Accounts = %#v", merged.Soloist.Accounts)
			}
		})
	}
}

func TestStartSoloistPairingIsStateNotError(t *testing.T) {
	fake := &fakeSoloistBackend{
		state:    soloist.State{NeedsPairing: true, ErrorCode: soloist.CodePairingRequired},
		startErr: soloist.ErrPairingRequired,
	}
	service, _ := newTestService(t, settings.BackendSoloist, fake)

	state, err := service.StartSoloist()
	if err != nil {
		t.Fatalf("StartSoloist() error = %v", err)
	}
	if fake.startCalls != 1 {
		t.Fatalf("startCalls = %d, want 1", fake.startCalls)
	}
	if state.Soloist == nil || !state.Soloist.NeedsPairing || state.Soloist.ErrorCode != soloist.CodePairingRequired {
		t.Fatalf("state.Soloist = %#v", state.Soloist)
	}
}

func TestInstallSoloistReportsProgress(t *testing.T) {
	fake := &fakeSoloistBackend{installVersion: "1.3.10.12"}
	service, _ := newTestService(t, settings.BackendSoloist, fake)

	var duringInstalling bool
	var duringStage string
	fake.onProgress = func() {
		state := service.GetState()
		duringInstalling = state.Soloist != nil && state.Soloist.Installing
		duringStage = state.Soloist.InstallStage
	}

	state, err := service.InstallSoloist()
	if err != nil {
		t.Fatalf("InstallSoloist() error = %v", err)
	}
	if fake.installCalls != 1 || fake.progressCalls != 2 {
		t.Fatalf("installCalls = %d, progressCalls = %d", fake.installCalls, fake.progressCalls)
	}
	if !duringInstalling {
		t.Fatal("Installing = false during progress")
	}
	if duringStage != "downloading" {
		t.Fatalf("stage during progress = %q", duringStage)
	}
	if state.Soloist == nil {
		t.Fatal("Soloist = nil after install")
	}
	if state.Soloist.Installing || state.Soloist.InstallStage != "" {
		t.Fatalf("install progress not cleared: %#v", state.Soloist)
	}
	if state.Soloist.Version != "1.3.10.12" || !state.Soloist.Installed {
		t.Fatalf("installed state = %#v", state.Soloist)
	}
}

func TestStopAndRestartSoloist(t *testing.T) {
	fake := &fakeSoloistBackend{state: soloist.State{Installed: true, Running: true}}
	service, _ := newTestService(t, settings.BackendSoloist, fake)

	state, err := service.StopSoloist()
	if err != nil {
		t.Fatalf("StopSoloist() error = %v", err)
	}
	if fake.stopCalls != 1 {
		t.Fatalf("stopCalls = %d, want 1", fake.stopCalls)
	}
	if state.PlaybackBackend != settings.BackendSoloist || state.Soloist == nil {
		t.Fatalf("stop state = %#v", state)
	}

	state, err = service.RestartSoloist()
	if err != nil {
		t.Fatalf("RestartSoloist() error = %v", err)
	}
	if fake.restartCalls != 1 {
		t.Fatalf("restartCalls = %d, want 1", fake.restartCalls)
	}
	if state.PlaybackBackend != settings.BackendSoloist || state.Soloist == nil {
		t.Fatalf("restart state = %#v", state)
	}
}

func TestSoloistActionsRequireBackend(t *testing.T) {
	service, _ := newTestService(t, settings.BackendSoloist, nil)
	if _, err := service.InstallSoloist(); err == nil {
		t.Fatal("InstallSoloist() error = nil")
	}
	if _, err := service.SaveSoloistAPIKey("key"); err == nil {
		t.Fatal("SaveSoloistAPIKey() error = nil")
	}
	if _, err := service.ClearSoloistAPIKey(); err == nil {
		t.Fatal("ClearSoloistAPIKey() error = nil")
	}
	if _, err := service.StartSoloist(); err == nil {
		t.Fatal("StartSoloist() error = nil")
	}
	if _, err := service.StopSoloist(); err == nil {
		t.Fatal("StopSoloist() error = nil")
	}
	if _, err := service.RestartSoloist(); err == nil {
		t.Fatal("RestartSoloist() error = nil")
	}
}

func TestMaintenanceInstallsAndStartsWithKey(t *testing.T) {
	fake := &fakeSoloistBackend{
		due:               true,
		hasKey:            true,
		autoUpdated:       true,
		autoUpdateVersion: "1.3.10.12",
	}
	service, _ := newTestService(t, settings.BackendSoloist, fake)

	var duringInstalling bool
	var duringStage string
	fake.onProgress = func() {
		state := service.GetState()
		duringInstalling = state.Soloist != nil && state.Soloist.Installing
		if state.Soloist != nil {
			duringStage = state.Soloist.InstallStage
		}
	}

	service.runMaintenance(t.Context())

	if fake.autoUpdateCalls != 1 {
		t.Fatalf("autoUpdateCalls = %d, want 1", fake.autoUpdateCalls)
	}
	if fake.startCalls != 1 {
		t.Fatalf("startCalls = %d, want 1", fake.startCalls)
	}
	if fake.restartCalls != 0 {
		t.Fatalf("restartCalls = %d, want 0", fake.restartCalls)
	}
	if !duringInstalling || duringStage != "downloading" {
		t.Fatalf("progress during update = installing:%v stage:%q", duringInstalling, duringStage)
	}
	if fake.autoProgressCalls != 2 {
		t.Fatalf("autoProgressCalls = %d, want 2", fake.autoProgressCalls)
	}
	state := service.GetState()
	if state.Soloist == nil || state.Soloist.Installing || state.Soloist.InstallStage != "" {
		t.Fatalf("install progress not cleared: %#v", state.Soloist)
	}
	if !state.Soloist.Installed || state.Soloist.Version != "1.3.10.12" {
		t.Fatalf("state.Soloist = %#v, want the installed build", state.Soloist)
	}
}

func TestMaintenanceSkipsStartWithoutKey(t *testing.T) {
	fake := &fakeSoloistBackend{due: true, hasKey: false, autoUpdated: true}
	service, _ := newTestService(t, settings.BackendSoloist, fake)

	service.runMaintenance(t.Context())

	if fake.autoUpdateCalls != 1 {
		t.Fatalf("autoUpdateCalls = %d, want 1", fake.autoUpdateCalls)
	}
	if fake.startCalls != 0 {
		t.Fatalf("startCalls = %d, want 0 without a key", fake.startCalls)
	}
	state := service.GetState()
	if state.Soloist == nil || state.Soloist.Installing {
		t.Fatalf("install progress not cleared: %#v", state.Soloist)
	}
}

func TestMaintenanceUpdatesAndRestarts(t *testing.T) {
	fake := &fakeSoloistBackend{
		state:             soloist.State{Installed: true, Running: true},
		due:               true,
		hasKey:            true,
		autoUpdated:       true,
		autoUpdateVersion: "1.3.11.0",
	}
	service, _ := newTestService(t, settings.BackendSoloist, fake)

	service.runMaintenance(t.Context())

	if fake.autoUpdateCalls != 1 {
		t.Fatalf("autoUpdateCalls = %d, want 1", fake.autoUpdateCalls)
	}
	if fake.restartCalls != 1 {
		t.Fatalf("restartCalls = %d, want 1", fake.restartCalls)
	}
	if fake.startCalls != 0 {
		t.Fatalf("startCalls = %d, want 0 after an update restart", fake.startCalls)
	}
}

func TestMaintenanceNoOpWhenFresh(t *testing.T) {
	fake := &fakeSoloistBackend{
		state:  soloist.State{Installed: true},
		due:    false,
		hasKey: true,
	}
	service, _ := newTestService(t, settings.BackendSoloist, fake)

	service.runMaintenance(t.Context())

	if fake.autoUpdateCalls != 0 {
		t.Fatalf("autoUpdateCalls = %d, want 0 for a fresh build", fake.autoUpdateCalls)
	}
	if fake.startCalls != 1 {
		t.Fatalf("startCalls = %d, want 1 for a stopped fresh install", fake.startCalls)
	}
}

func TestMaintenanceFreshRunningNoOp(t *testing.T) {
	fake := &fakeSoloistBackend{
		state:  soloist.State{Installed: true, Running: true},
		due:    false,
		hasKey: true,
	}
	service, _ := newTestService(t, settings.BackendSoloist, fake)

	service.runMaintenance(t.Context())

	if fake.autoUpdateCalls != 0 {
		t.Fatalf("autoUpdateCalls = %d, want 0", fake.autoUpdateCalls)
	}
	if fake.startCalls != 0 {
		t.Fatalf("startCalls = %d, want 0", fake.startCalls)
	}
}

func TestSaveAndClearSoloistAPIKey(t *testing.T) {
	fake := &fakeSoloistBackend{}
	service, _ := newTestService(t, settings.BackendSoloist, fake)

	if _, err := service.SaveSoloistAPIKey("secret"); err != nil {
		t.Fatalf("SaveSoloistAPIKey() error = %v", err)
	}
	if fake.savedKey != "secret" || !fake.hasKey {
		t.Fatalf("saved key state = %q, %v", fake.savedKey, fake.hasKey)
	}
	if _, err := service.ClearSoloistAPIKey(); err != nil {
		t.Fatalf("ClearSoloistAPIKey() error = %v", err)
	}
	if fake.savedKey != "" || fake.hasKey {
		t.Fatalf("cleared key state = %q, %v", fake.savedKey, fake.hasKey)
	}
}
