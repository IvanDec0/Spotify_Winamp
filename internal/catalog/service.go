// Package catalog orchestrates the engine-backed library, search and context
// browsing surfaces. It has no Wails imports; the Wails audio adapter delegates
// to this service so the domain stays reusable.
package catalog

import (
	"context"
	"errors"
	"strings"
	"time"

	"cadence/internal/artwork"
	"cadence/internal/engine"
)

// Options wires the catalog service.
type Options struct {
	Supervisor *engine.Supervisor
	Art        *artwork.Cache
	Enabled    bool
}

// Service exposes the local audio engine, pairing and library browsing to the
// adapters. It never exposes stored playback credentials.
type Service struct {
	supervisor *engine.Supervisor
	art        *artwork.Cache
	enabled    bool
}

func NewService(options Options) *Service {
	return &Service{supervisor: options.Supervisor, art: options.Art, enabled: options.Enabled}
}

// ContextTrackInfo is the compact track metadata returned to the frontend.
type ContextTrackInfo struct {
	URI        string   `json:"uri"`
	Title      string   `json:"title"`
	Artists    []string `json:"artists"`
	Album      string   `json:"album"`
	Cover      string   `json:"cover"`
	DurationMS int      `json:"durationMs"`
}

// LibraryContext is the frontend-facing context listing.
type LibraryContext struct {
	URI    string             `json:"uri"`
	Ready  bool               `json:"ready"`
	Length int                `json:"length"`
	Cached int                `json:"cached"`
	Items  []ContextTrackInfo `json:"items"`
}

// GetState returns the engine and pairing state.
func (s *Service) GetState() engine.AudioState {
	if !s.enabled {
		return engine.AudioState{
			Available: false,
			Running:   false,
			Engine:    engine.Name,
			Error:     "The local audio engine is disabled in demo mode.",
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.supervisor.State(ctx)
}

// ResetSession removes the stored go-librespot credentials and starts a new
// pairing flow.
func (s *Service) ResetSession() (engine.AudioState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := s.supervisor.ResetSession(ctx); err != nil {
		return s.GetState(), err
	}
	return s.GetState(), nil
}

// Restart restarts the engine while keeping the paired credentials.
func (s *Service) Restart() (engine.AudioState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := s.supervisor.Restart(ctx); err != nil {
		return s.GetState(), err
	}
	return s.GetState(), nil
}

// Playlists returns a page of the user's library playlists.
func (s *Service) Playlists(offset, limit int) (engine.PlaylistPage, error) {
	client, err := s.client()
	if err != nil {
		return engine.PlaylistPage{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	page, err := client.Playlists(ctx, offset, limit)
	if err != nil {
		return engine.PlaylistPage{}, err
	}
	for index := range page.Items {
		page.Items[index].ImageURL = s.artURL(page.Items[index].ImageURL)
	}
	return page, nil
}

// Albums returns a page of the user's saved albums.
func (s *Service) Albums(offset, limit int) (engine.AlbumPage, error) {
	client, err := s.client()
	if err != nil {
		return engine.AlbumPage{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	page, err := client.Albums(ctx, offset, limit)
	if err != nil {
		return engine.AlbumPage{}, err
	}
	for index := range page.Items {
		page.Items[index].ImageURL = s.artURL(page.Items[index].ImageURL)
	}
	return page, nil
}

// Artists returns a page of the user's followed artists.
func (s *Service) Artists(offset, limit int) (engine.ArtistPage, error) {
	client, err := s.client()
	if err != nil {
		return engine.ArtistPage{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	page, err := client.Artists(ctx, offset, limit)
	if err != nil {
		return engine.ArtistPage{}, err
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

// Search finds tracks, albums, artists and playlists in the Spotify catalog.
func (s *Service) Search(query string, limit int) (engine.SearchResults, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return engine.SearchResults{}, errors.New("the search query is empty")
	}
	client, err := s.client()
	if err != nil {
		return engine.SearchResults{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	results, err := client.Search(ctx, query, limit)
	if err != nil {
		return engine.SearchResults{}, err
	}
	for _, group := range []*engine.SearchGroup{&results.Tracks, &results.Albums, &results.Artists, &results.Playlists} {
		for index := range group.Hits {
			group.Hits[index].Image = s.artURL(group.Hits[index].Image)
		}
	}
	return results, nil
}

// StartRadio plays a Spotify radio station seeded by a track, album, artist or
// playlist uri.
func (s *Service) StartRadio(uri string) error {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return errors.New("the radio seed is empty")
	}
	client, err := s.client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return client.StartRadio(ctx, uri)
}

func (s *Service) client() (*engine.Client, error) {
	client := s.supervisor.Client()
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
