// Package audioapi is the Wails adapter for the catalog and engine services.
package audioapi

import (
	"context"
	"errors"
	"sync"
	"time"

	"cadence/internal/catalog"
	"cadence/internal/engine"
	"cadence/internal/settings"
	"cadence/internal/soloist"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// AudioEvent is emitted when the local audio engine starts, stops or changes
// pairing state.
const AudioEvent = "audio:state"

// installProgressInterval throttles download progress events. A 64 MB archive
// arrives in thousands of 32 KiB chunks; one event per chunk would flood the
// frontend bridge. Stage changes and the final state are still emitted
// immediately.
const installProgressInterval = 200 * time.Millisecond

// SoloistBackend is the subset of *soloist.Service the audio adapter uses.
// *soloist.Service satisfies it structurally.
type SoloistBackend interface {
	State() soloist.State
	Start(context.Context) error
	Stop(context.Context) error
	Restart(context.Context) error
	Install(context.Context, func(soloist.Progress)) (*soloist.Binary, error)
	UpdateDue(time.Time) bool
	AutoUpdate(context.Context, func(soloist.Progress)) (*soloist.Binary, bool, error)
	Prune() error
	SaveAPIKey(string) error
	ClearAPIKey() error
	HasAPIKey() bool
	SetEmitter(func(soloist.State))
}

// Options wires the audio service.
type Options struct {
	Catalog         *catalog.Service
	Supervisor      *engine.Supervisor
	Settings        *settings.Store
	Enabled         bool
	Soloist         SoloistBackend
	Web             WebBackend
	Developer       DeveloperBackend
	PlaybackBackend string
}

// installProgress is the mutable install snapshot merged into SoloistState. It
// has its own mutex so install progress callbacks never touch the service lock.
type installProgress struct {
	mu         sync.Mutex
	installing bool
	stage      string
	downloaded int64
	total      int64
}

// Service exposes the local audio engine, pairing and library browsing to the
// frontend. It never exposes stored playback credentials.
type Service struct {
	catalog    *catalog.Service
	supervisor *engine.Supervisor
	settings   *settings.Store
	enabled    bool
	soloist    SoloistBackend
	web        WebBackend
	developer  DeveloperBackend
	backend    string
	progress   installProgress

	mu                 sync.Mutex
	app                *application.App
	maintenanceCancel  context.CancelFunc
	webProvisionCancel context.CancelFunc
	webProvisionDone   chan struct{}
}

func NewService(options Options) *Service {
	return &Service{
		catalog:    options.Catalog,
		supervisor: options.Supervisor,
		settings:   options.Settings,
		enabled:    options.Enabled,
		soloist:    options.Soloist,
		web:        options.Web,
		developer:  options.Developer,
		backend:    settings.NormalizePlaybackBackend(options.PlaybackBackend),
	}
}

func (s *Service) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	s.mu.Lock()
	s.app = application.Get()
	s.mu.Unlock()

	s.supervisor.SetEmitter(func(AudioState) {
		s.emitMerged()
	})
	if s.soloist != nil {
		s.soloist.SetEmitter(func(soloist.State) {
			s.emitMerged()
		})
	}
	if s.enabled {
		if s.soloist != nil && s.backend == settings.BackendSoloist {
			ctx, cancel := context.WithCancel(context.Background())
			s.mu.Lock()
			s.maintenanceCancel = cancel
			s.mu.Unlock()
			go s.maintainSoloist(ctx)
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			_ = s.supervisor.Start(ctx)
		}()
		if s.web != nil && s.backend == settings.BackendWeb {
			// The web backend downloads its runtime, signs in and starts the
			// helper once at startup so a first run needs no Settings visit.
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			s.mu.Lock()
			s.webProvisionCancel = cancel
			s.webProvisionDone = done
			s.mu.Unlock()
			go func() {
				defer close(done)
				s.provisionWeb(ctx)
			}()
		}
	}
	return nil
}

func (s *Service) ServiceShutdown() error {
	s.mu.Lock()
	cancel := s.maintenanceCancel
	s.maintenanceCancel = nil
	webCancel := s.webProvisionCancel
	s.webProvisionCancel = nil
	webDone := s.webProvisionDone
	s.webProvisionDone = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if webCancel != nil {
		webCancel()
	}
	if webDone != nil {
		// Give the background setup a bounded chance to release the runtime
		// staging directory before the process exits.
		waitCtx, waitCancel := context.WithTimeout(context.Background(), 5*time.Second)
		select {
		case <-webDone:
		case <-waitCtx.Done():
		}
		waitCancel()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var errs []error
	if s.supervisor != nil {
		errs = append(errs, s.supervisor.Stop(ctx))
	}
	if s.soloist != nil {
		errs = append(errs, s.soloist.Stop(ctx))
	}
	return errors.Join(errs...)
}

// maintainSoloist runs maintenance immediately and then every six hours until
// the context is cancelled by ServiceShutdown.
func (s *Service) maintainSoloist(ctx context.Context) {
	s.runMaintenance(ctx)
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runMaintenance(ctx)
		}
	}
}

// runMaintenance installs or updates Soloist when due, starts it when a key
// exists, and restarts a running process after an update. Failures stay in the
// merged state; this path never logs credentials or blocks startup.
func (s *Service) runMaintenance(ctx context.Context) {
	if s.soloist == nil {
		return
	}
	s.progress.mu.Lock()
	installing := s.progress.installing
	s.progress.mu.Unlock()
	if installing {
		// A manual install owns the progress state.
		return
	}

	if !s.soloist.UpdateDue(time.Now()) {
		if s.soloist.HasAPIKey() && !s.soloist.State().Running {
			s.startSoloistMaintenance(ctx, 45*time.Second)
			s.emitMerged()
		}
		return
	}

	s.setInstallProgress(true, "downloading", 0, 0)
	s.emitMerged()
	stage := "downloading"
	var lastEmit time.Time
	_, updated, err := s.soloist.AutoUpdate(ctx, func(progress soloist.Progress) {
		// The snapshot is always current; only the event is throttled.
		s.setInstallProgress(true, progress.Stage, progress.Downloaded, progress.Total)
		now := time.Now()
		switch {
		case progress.Stage != stage:
			stage = progress.Stage
			lastEmit = now
			s.emitMerged()
		case progress.Stage != "downloading":
			lastEmit = now
			s.emitMerged()
		case now.Sub(lastEmit) >= installProgressInterval:
			lastEmit = now
			s.emitMerged()
		}
	})
	s.setInstallProgress(false, "", 0, 0)
	s.emitMerged()
	if err != nil {
		// The Soloist service already stored the error in its state.
		return
	}
	if !s.soloist.HasAPIKey() {
		// Leave it stopped so the onboarding modal can ask for the key.
		return
	}
	switch {
	case updated && s.soloist.State().Running:
		s.restartSoloistMaintenance(ctx)
	case !s.soloist.State().Running:
		s.startSoloistMaintenance(ctx, 45*time.Second)
	}
	s.emitMerged()
}

func (s *Service) startSoloistMaintenance(ctx context.Context, timeout time.Duration) {
	startCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	// Pairing is a state, not a maintenance failure. Any other error is
	// already reflected in the merged state by the Soloist service.
	_ = s.soloist.Start(startCtx)
}

func (s *Service) restartSoloistMaintenance(ctx context.Context) {
	restartCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	_ = s.soloist.Restart(restartCtx)
}

// GetState returns the engine state merged with the selected playback backend
// and, in Soloist mode, the Soloist process and install state.
func (s *Service) GetState() AudioState {
	return s.mergeState(s.catalog.GetState())
}

// mergeState adds the backend selection and Soloist state to a catalog state.
// It is used by GetState and by the actions that return the state produced by
// catalog.ResetSession and catalog.Restart, which carry neither field.
func (s *Service) mergeState(state AudioState) AudioState {
	state.PlaybackBackend = s.backend
	// Soloist state is merged in both modes so Settings can install it and
	// store the API key before the user switches backends. The process only
	// runs in Soloist mode.
	if s.soloist != nil {
		state.Soloist = s.soloistState()
	}
	return state
}

// emitMerged emits one audio:state event carrying both backends. It snapshots
// the app under the mutex and never holds it while querying either backend.
func (s *Service) emitMerged() {
	s.mu.Lock()
	app := s.app
	s.mu.Unlock()
	if app == nil {
		return
	}
	app.Event.Emit(AudioEvent, s.GetState())
}

func (s *Service) soloistState() *engine.SoloistState {
	domain := s.soloist.State()
	mapped := engine.SoloistState{
		Installed:    domain.Installed,
		BinaryPath:   domain.BinaryPath,
		Version:      domain.Version,
		BuiltAt:      formatUTC(domain.BuiltAt),
		ExpiresAt:    formatUTC(domain.ExpiresAt),
		Expired:      domain.Expired,
		DaysLeft:     domain.DaysLeft,
		Running:      domain.Running,
		LoggedIn:     domain.LoggedIn,
		NeedsPairing: domain.NeedsPairing,
		Active:       domain.Active,
		DeviceName:   domain.DeviceName,
		Accounts:     append([]string(nil), domain.Accounts...),
		APIKeySet:    s.soloist.HasAPIKey(),
		Error:        domain.Error,
		ErrorCode:    domain.ErrorCode,
	}

	s.progress.mu.Lock()
	mapped.Installing = s.progress.installing
	mapped.InstallStage = s.progress.stage
	mapped.Downloaded = s.progress.downloaded
	mapped.Total = s.progress.total
	s.progress.mu.Unlock()
	return &mapped
}

func formatUTC(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

// OpenPairing opens the current Spotify device pairing page in the system
// browser.
func (s *Service) OpenPairing() error {
	state := s.catalog.GetState()
	if state.Pairing == nil || state.Pairing.URL == "" {
		return errors.New("there is no pending Spotify pairing request")
	}
	s.mu.Lock()
	app := s.app
	s.mu.Unlock()
	if app == nil {
		return errors.New("application is not ready")
	}
	return app.Browser.OpenURL(state.Pairing.URL)
}

// ResetSession removes the stored go-librespot credentials and starts a new
// pairing flow.
func (s *Service) ResetSession() (AudioState, error) {
	state, err := s.catalog.ResetSession()
	return s.mergeState(state), err
}

// Restart restarts the engine while keeping the paired credentials.
func (s *Service) Restart() (AudioState, error) {
	state, err := s.catalog.Restart()
	return s.mergeState(state), err
}

// InstallSoloist downloads and installs the newest Spotify Soloist build,
// reporting progress through the merged audio state.
func (s *Service) InstallSoloist() (AudioState, error) {
	if s.soloist == nil {
		return s.GetState(), errors.New("Spotify Soloist is not enabled")
	}
	s.setInstallProgress(true, "downloading", 0, 0)
	s.emitMerged()
	stage := "downloading"
	var lastEmit time.Time
	_, err := s.soloist.Install(context.Background(), func(progress soloist.Progress) {
		// The snapshot is always current; only the event is throttled.
		s.setInstallProgress(true, progress.Stage, progress.Downloaded, progress.Total)
		now := time.Now()
		switch {
		case progress.Stage != stage:
			stage = progress.Stage
			lastEmit = now
			s.emitMerged()
		case progress.Stage != "downloading":
			// Extracting/verifying/installing emit immediately.
			lastEmit = now
			s.emitMerged()
		case now.Sub(lastEmit) >= installProgressInterval:
			lastEmit = now
			s.emitMerged()
		}
	})
	s.setInstallProgress(false, "", 0, 0)
	s.emitMerged()
	return s.GetState(), err
}

// SaveSoloistAPIKey stores the user's Soloist API key without returning it.
func (s *Service) SaveSoloistAPIKey(key string) (AudioState, error) {
	if s.soloist == nil {
		return s.GetState(), errors.New("Spotify Soloist is not enabled")
	}
	err := s.soloist.SaveAPIKey(key)
	s.emitMerged()
	return s.GetState(), err
}

// ClearSoloistAPIKey removes the stored Soloist API key.
func (s *Service) ClearSoloistAPIKey() (AudioState, error) {
	if s.soloist == nil {
		return s.GetState(), errors.New("Spotify Soloist is not enabled")
	}
	err := s.soloist.ClearAPIKey()
	s.emitMerged()
	return s.GetState(), err
}

// StartSoloist starts the Soloist process. A pairing requirement is reported
// through the state, not as an action error.
func (s *Service) StartSoloist() (AudioState, error) {
	if s.soloist == nil {
		return s.GetState(), errors.New("Spotify Soloist is not enabled")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	err := s.soloist.Start(ctx)
	if soloist.CodeOf(err) == soloist.CodePairingRequired {
		err = nil
	}
	s.emitMerged()
	return s.GetState(), err
}

// StopSoloist stops the Soloist process.
func (s *Service) StopSoloist() (AudioState, error) {
	if s.soloist == nil {
		return s.GetState(), errors.New("Spotify Soloist is not enabled")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := s.soloist.Stop(ctx)
	s.emitMerged()
	return s.GetState(), err
}

// RestartSoloist restarts the Soloist process. A pairing requirement is
// reported through the state, not as an action error.
func (s *Service) RestartSoloist() (AudioState, error) {
	if s.soloist == nil {
		return s.GetState(), errors.New("Spotify Soloist is not enabled")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	err := s.soloist.Restart(ctx)
	if soloist.CodeOf(err) == soloist.CodePairingRequired {
		err = nil
	}
	s.emitMerged()
	return s.GetState(), err
}

// OpenSoloistDashboard opens Spotify's dashboard page where the user creates a
// Soloist API key.
func (s *Service) OpenSoloistDashboard() error {
	return s.openURL(soloist.DashboardURL)
}

// OpenSoloistDownloads opens Spotify's official Soloist download page.
func (s *Service) OpenSoloistDownloads() error {
	return s.openURL(soloist.DownloadsURL)
}

func (s *Service) openURL(url string) error {
	s.mu.Lock()
	app := s.app
	s.mu.Unlock()
	if app == nil {
		return errors.New("application is not ready")
	}
	return app.Browser.OpenURL(url)
}

func (s *Service) setInstallProgress(installing bool, stage string, downloaded, total int64) {
	s.progress.mu.Lock()
	s.progress.installing = installing
	s.progress.stage = stage
	s.progress.downloaded = downloaded
	s.progress.total = total
	s.progress.mu.Unlock()
}

// GetEngineSettings returns the persisted engine playback and cache options,
// falling back to the defaults when the settings file has no engine section.
func (s *Service) GetEngineSettings() (EngineSettings, error) {
	if s.settings == nil {
		return EngineSettings{}, errors.New("the settings store is not configured")
	}
	config, err := s.settings.Load()
	if err != nil {
		return EngineSettings{}, err
	}
	if config.Engine == nil {
		defaults := settings.Default()
		return *defaults.Engine, nil
	}
	return *config.Engine, nil
}

// SetEngineSettings validates and persists playback options and applies them
// to the supervisor. It never restarts the engine: the new values take effect
// on the next start, or after the caller invokes Restart following an explicit
// user confirmation.
func (s *Service) SetEngineSettings(value EngineSettings) (EngineSettings, error) {
	if s.settings == nil {
		return EngineSettings{}, errors.New("the settings store is not configured")
	}
	normalized := engine.NormalizePlaybackConfig(playbackConfig(value))
	saved := engineState(normalized)
	if err := s.settings.UpdateEngine(func(state *settings.EngineState) error {
		*state = saved
		return nil
	}); err != nil {
		return EngineSettings{}, err
	}
	s.supervisor.ApplyPlaybackConfig(normalized)
	return saved, nil
}

// GetPlaybackSettings returns the persisted audio backend, defaulting to the
// web player when the settings file has no playback section.
func (s *Service) GetPlaybackSettings() (PlaybackSettings, error) {
	if s.settings == nil {
		return PlaybackSettings{}, errors.New("the settings store is not configured")
	}
	config, err := s.settings.Load()
	if err != nil {
		return PlaybackSettings{}, err
	}
	if config.Playback == nil {
		return PlaybackSettings{Backend: settings.BackendWeb}, nil
	}
	return PlaybackSettings{
		Backend:          settings.NormalizePlaybackBackend(config.Playback.Backend),
		KeepWebAvailable: config.Playback.KeepWebAvailable,
	}, nil
}

// SetPlaybackSettings validates and persists the audio backend. It never
// starts, stops or switches a backend: the new value applies on the next
// application start.
func (s *Service) SetPlaybackSettings(value PlaybackSettings) (PlaybackSettings, error) {
	if s.settings == nil {
		return PlaybackSettings{}, errors.New("the settings store is not configured")
	}
	saved := PlaybackSettings{
		Backend:          settings.NormalizePlaybackBackend(value.Backend),
		KeepWebAvailable: value.KeepWebAvailable,
	}
	if err := s.settings.UpdatePlayback(func(state *settings.PlaybackState) error {
		*state = saved
		return nil
	}); err != nil {
		return PlaybackSettings{}, err
	}
	if s.web != nil {
		s.web.SetKeepAvailable(saved.KeepWebAvailable)
		if saved.KeepWebAvailable && saved.Backend == settings.BackendWeb {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
				defer cancel()
				_ = s.web.EnsureStarted(ctx)
			}()
		}
	}
	return saved, nil
}

func playbackConfig(state settings.EngineState) engine.PlaybackConfig {
	return engine.PlaybackConfig{
		Bitrate:                   state.Bitrate,
		CrossfadeDurationMS:       state.CrossfadeDurationMS,
		NormalisationDisabled:     state.NormalisationDisabled,
		NormalisationUseAlbumGain: state.NormalisationUseAlbumGain,
		NormalisationPregain:      state.NormalisationPregain,
		CacheEnabled:              state.CacheEnabled,
		CacheDir:                  state.CacheDir,
		CacheSizeLimit:            state.CacheSizeLimit,
		FlacEnabled:               state.FlacEnabled,
		MPRISEnabled:              state.MPRISEnabled,
		DeviceIsGroup:             state.DeviceIsGroup,
		ImageSize:                 state.ImageSize,
	}
}

func engineState(config engine.PlaybackConfig) settings.EngineState {
	return settings.EngineState{
		Bitrate:                   config.Bitrate,
		CrossfadeDurationMS:       config.CrossfadeDurationMS,
		NormalisationDisabled:     config.NormalisationDisabled,
		NormalisationUseAlbumGain: config.NormalisationUseAlbumGain,
		NormalisationPregain:      config.NormalisationPregain,
		CacheEnabled:              config.CacheEnabled,
		CacheDir:                  config.CacheDir,
		CacheSizeLimit:            config.CacheSizeLimit,
		FlacEnabled:               config.FlacEnabled,
		MPRISEnabled:              config.MPRISEnabled,
		DeviceIsGroup:             config.DeviceIsGroup,
		ImageSize:                 config.ImageSize,
	}
}

// Playlists returns a page of the user's library playlists.
func (s *Service) Playlists(offset, limit int) (PlaylistPage, error) {
	return s.catalog.Playlists(offset, limit)
}

// Albums returns a page of the user's saved albums.
func (s *Service) Albums(offset, limit int) (AlbumPage, error) {
	return s.catalog.Albums(offset, limit)
}

// Artists returns a page of the user's followed artists.
func (s *Service) Artists(offset, limit int) (ArtistPage, error) {
	return s.catalog.Artists(offset, limit)
}

// ContextTracks lists the tracks of a playlist, album, artist, show or the
// Liked Songs collection.
func (s *Service) ContextTracks(uri string) (LibraryContext, error) {
	return s.catalog.ContextTracks(uri)
}

// YourEpisodes returns the user's saved podcast episodes.
func (s *Service) YourEpisodes() (LibraryContext, error) {
	return s.catalog.YourEpisodes()
}

// PlayContext starts playback of a playlist, album, artist, show or Liked
// Songs collection. skipToURI selects a track inside the context.
func (s *Service) PlayContext(uri, skipToURI string) error {
	return s.catalog.PlayContext(uri, skipToURI)
}

// PlayTrack starts playback of a single track URI.
func (s *Service) PlayTrack(uri string) error {
	return s.catalog.PlayTrack(uri)
}

// AddToQueue appends a track to the go-librespot queue.
func (s *Service) AddToQueue(uri string) error {
	return s.catalog.AddToQueue(uri)
}

// SetLiked adds or removes a track in the user's collection.
func (s *Service) SetLiked(uri string, liked bool) error {
	return s.catalog.SetLiked(uri, liked)
}

// IsLiked reports whether a track is in the user's collection.
func (s *Service) IsLiked(uri string) (bool, error) {
	return s.catalog.IsLiked(uri)
}

// SaveAlbum saves or removes an album in the user's collection.
func (s *Service) SaveAlbum(uri string, saved bool) error {
	return s.catalog.SaveAlbum(uri, saved)
}

// IsAlbumSaved reports whether an album is in the user's collection.
func (s *Service) IsAlbumSaved(uri string) (bool, error) {
	return s.catalog.IsAlbumSaved(uri)
}

// SaveShow saves or removes a show in the user's collection.
func (s *Service) SaveShow(uri string, saved bool) error {
	return s.catalog.SaveShow(uri, saved)
}

// IsShowSaved reports whether a show is in the user's collection.
func (s *Service) IsShowSaved(uri string) (bool, error) {
	return s.catalog.IsShowSaved(uri)
}

// FollowArtist follows or unfollows an artist.
func (s *Service) FollowArtist(uri string, following bool) error {
	return s.catalog.FollowArtist(uri, following)
}

// IsArtistFollowed reports whether the user follows an artist.
func (s *Service) IsArtistFollowed(uri string) (bool, error) {
	return s.catalog.IsArtistFollowed(uri)
}

// AddToPlaylist appends a track or episode to a playlist.
func (s *Service) AddToPlaylist(playlistURI, uri string) error {
	return s.catalog.AddToPlaylist(playlistURI, uri)
}

// RemoveFromPlaylist removes a track or episode from a playlist.
func (s *Service) RemoveFromPlaylist(playlistURI, uri string) error {
	return s.catalog.RemoveFromPlaylist(playlistURI, uri)
}

// Search finds tracks, albums, artists and playlists in the Spotify catalog.
func (s *Service) Search(query string, limit int) (SearchResults, error) {
	return s.catalog.Search(query, limit)
}

// StartRadio plays a Spotify radio station seeded by a track, album, artist or
// playlist uri.
func (s *Service) StartRadio(uri string) error {
	return s.catalog.StartRadio(uri)
}
