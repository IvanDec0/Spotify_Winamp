package golibrespot

import (
	"context"
	"errors"
	"sync"
	"time"

	"cadence/internal/catalog"
	"cadence/internal/engine"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// AudioEvent is emitted when the local audio engine starts, stops or changes
// pairing state.
const AudioEvent = "audio:state"

// ServiceOptions wires the temporary Wails audio adapter.
type ServiceOptions struct {
	Catalog    *catalog.Service
	Supervisor *engine.Supervisor
	Enabled    bool
}

// Service exposes the local audio engine, pairing and library browsing to the
// frontend. It never exposes stored playback credentials. It is a temporary
// adapter that moves to internal/wailsapi/audio in Phase 3.
type Service struct {
	catalog    *catalog.Service
	supervisor *engine.Supervisor
	enabled    bool

	mu  sync.Mutex
	app *application.App
}

func NewService(options ServiceOptions) *Service {
	return &Service{
		catalog:    options.Catalog,
		supervisor: options.Supervisor,
		enabled:    options.Enabled,
	}
}

func (s *Service) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	s.mu.Lock()
	s.app = application.Get()
	app := s.app
	s.mu.Unlock()
	s.supervisor.SetEmitter(func(state engine.AudioState) {
		if app != nil {
			app.Event.Emit(AudioEvent, state)
		}
	})
	if s.enabled {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			_ = s.supervisor.Start(ctx)
		}()
	}
	return nil
}

func (s *Service) ServiceShutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.supervisor.Stop(ctx)
}

// GetState returns the engine and pairing state.
func (s *Service) GetState() engine.AudioState {
	return s.catalog.GetState()
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
func (s *Service) ResetSession() (engine.AudioState, error) {
	return s.catalog.ResetSession()
}

// Restart restarts the engine while keeping the paired credentials.
func (s *Service) Restart() (engine.AudioState, error) {
	return s.catalog.Restart()
}

// Playlists returns a page of the user's library playlists.
func (s *Service) Playlists(offset, limit int) (engine.PlaylistPage, error) {
	return s.catalog.Playlists(offset, limit)
}

// Albums returns a page of the user's saved albums.
func (s *Service) Albums(offset, limit int) (engine.AlbumPage, error) {
	return s.catalog.Albums(offset, limit)
}

// Artists returns a page of the user's followed artists.
func (s *Service) Artists(offset, limit int) (engine.ArtistPage, error) {
	return s.catalog.Artists(offset, limit)
}

// ContextTracks lists the tracks of a playlist, album, artist, show or the
// Liked Songs collection.
func (s *Service) ContextTracks(uri string) (catalog.LibraryContext, error) {
	return s.catalog.ContextTracks(uri)
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

// Search finds tracks, albums, artists and playlists in the Spotify catalog.
func (s *Service) Search(query string, limit int) (engine.SearchResults, error) {
	return s.catalog.Search(query, limit)
}

// StartRadio plays a Spotify radio station seeded by a track, album, artist or
// playlist uri.
func (s *Service) StartRadio(uri string) error {
	return s.catalog.StartRadio(uri)
}
