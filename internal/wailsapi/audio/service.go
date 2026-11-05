// Package audioapi is the Wails adapter for the catalog and engine services.
package audioapi

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

// Options wires the audio service.
type Options struct {
	Catalog    *catalog.Service
	Supervisor *engine.Supervisor
	Enabled    bool
}

// Service exposes the local audio engine, pairing and library browsing to the
// frontend. It never exposes stored playback credentials.
type Service struct {
	catalog    *catalog.Service
	supervisor *engine.Supervisor
	enabled    bool

	mu  sync.Mutex
	app *application.App
}

func NewService(options Options) *Service {
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
	s.supervisor.SetEmitter(func(state AudioState) {
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
func (s *Service) GetState() AudioState {
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
func (s *Service) ResetSession() (AudioState, error) {
	return s.catalog.ResetSession()
}

// Restart restarts the engine while keeping the paired credentials.
func (s *Service) Restart() (AudioState, error) {
	return s.catalog.Restart()
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
