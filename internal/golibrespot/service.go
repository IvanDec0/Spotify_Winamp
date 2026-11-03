package golibrespot

import (
	"context"
	"errors"
	"sync"
	"time"

	"cadence/internal/artcache"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// AudioEvent is emitted when the local audio engine starts, stops or changes
// pairing state.
const AudioEvent = "audio:state"

// ServiceOptions wires the audio service.
type ServiceOptions struct {
	Runtime *Runtime
	Art     *artcache.Cache
	Enabled bool
}

// Service exposes the local audio engine, pairing and library browsing to the
// frontend. It never exposes stored playback credentials.
type Service struct {
	runtime *Runtime
	art     *artcache.Cache
	enabled bool

	mu  sync.Mutex
	app *application.App
}

func NewService(options ServiceOptions) *Service {
	return &Service{runtime: options.Runtime, art: options.Art, enabled: options.Enabled}
}

func (s *Service) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	s.mu.Lock()
	s.app = application.Get()
	app := s.app
	s.mu.Unlock()
	s.runtime.SetEmitter(func(state AudioState) {
		if app != nil {
			app.Event.Emit(AudioEvent, state)
		}
	})
	if s.enabled {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			_ = s.runtime.Start(ctx)
		}()
	}
	return nil
}

func (s *Service) ServiceShutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.runtime.Stop(ctx)
}

// GetState returns the engine and pairing state.
func (s *Service) GetState() AudioState {
	if !s.enabled {
		return AudioState{
			Available: false,
			Running:   false,
			Engine:    engineName,
			Error:     "The local audio engine is disabled in demo mode.",
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.runtime.State(ctx)
}

// OpenPairing opens the current Spotify device pairing page in the system
// browser.
func (s *Service) OpenPairing() error {
	state := s.GetState()
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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := s.runtime.ResetSession(ctx); err != nil {
		return s.GetState(), err
	}
	return s.GetState(), nil
}

// Restart restarts the engine while keeping the paired credentials.
func (s *Service) Restart() (AudioState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := s.runtime.Restart(ctx); err != nil {
		return s.GetState(), err
	}
	return s.GetState(), nil
}

// Playlists returns a page of the user's library playlists.
func (s *Service) Playlists(offset, limit int) (PlaylistPage, error) {
	client, err := s.client()
	if err != nil {
		return PlaylistPage{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	page, err := client.Playlists(ctx, offset, limit)
	if err != nil {
		return PlaylistPage{}, err
	}
	for index := range page.Items {
		page.Items[index].ImageURL = s.artURL(page.Items[index].ImageURL)
	}
	return page, nil
}

// Albums returns a page of the user's saved albums.
func (s *Service) Albums(offset, limit int) (AlbumPage, error) {
	client, err := s.client()
	if err != nil {
		return AlbumPage{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	page, err := client.Albums(ctx, offset, limit)
	if err != nil {
		return AlbumPage{}, err
	}
	for index := range page.Items {
		page.Items[index].ImageURL = s.artURL(page.Items[index].ImageURL)
	}
	return page, nil
}

// Artists returns a page of the user's followed artists.
func (s *Service) Artists(offset, limit int) (ArtistPage, error) {
	client, err := s.client()
	if err != nil {
		return ArtistPage{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	page, err := client.Artists(ctx, offset, limit)
	if err != nil {
		return ArtistPage{}, err
	}
	for index := range page.Items {
		page.Items[index].ImageURL = s.artURL(page.Items[index].ImageURL)
	}
	return page, nil
}

// ContextTracks lists the tracks of a playlist, album, artist, show or the
// Liked Songs collection.
func (s *Service) ContextTracks(uri string) (LibraryContext, error) {
	client, err := s.client()
	if err != nil {
		return LibraryContext{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	tracks, err := client.ContextTracks(ctx, uri)
	if err != nil {
		return LibraryContext{}, err
	}
	result := LibraryContext{URI: tracks.URI, Ready: tracks.Ready, Length: tracks.Length, Cached: tracks.Cached}
	for _, item := range tracks.Items {
		track := item.Track
		cover := ""
		if track.AlbumCoverURL != nil {
			cover = s.artURL(*track.AlbumCoverURL)
		}
		result.Items = append(result.Items, ContextTrackInfo{
			URI:        item.URI,
			Title:      track.Name,
			Artists:    append([]string(nil), track.ArtistNames...),
			Album:      track.AlbumName,
			Cover:      cover,
			DurationMS: track.Duration,
		})
	}
	return result, nil
}

// PlayContext starts playback of a playlist, album, artist, show or Liked
// Songs collection. skipToURI selects a track inside the context.
func (s *Service) PlayContext(uri, skipToURI string) error {
	client, err := s.client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return client.Play(ctx, uri, skipToURI, false, 0)
}

// PlayTrack starts playback of a single track URI.
func (s *Service) PlayTrack(uri string) error {
	return s.PlayContext(uri, "")
}

// AddToQueue appends a track to the go-librespot queue.
func (s *Service) AddToQueue(uri string) error {
	client, err := s.client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return client.AddToQueue(ctx, uri)
}

func (s *Service) client() (*Client, error) {
	client := s.runtime.Client()
	if client == nil {
		return nil, errors.New("the local audio engine is not running")
	}
	return client, nil
}

func (s *Service) artURL(raw string) string {
	if raw == "" || s.art == nil {
		return ""
	}
	key := s.art.Register(raw)
	if key == "" {
		return ""
	}
	return "/art/" + key
}
