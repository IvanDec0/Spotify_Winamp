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

// Playback is the playback surface the catalog needs. The default
// implementation wraps the go-librespot client; the Soloist provider is
// injected in Soloist mode.
type Playback interface {
	PlayContext(ctx context.Context, uri, skipToURI string) error
	PlayTrack(ctx context.Context, uri string) error
	AddToQueue(ctx context.Context, uri string) error
	StartRadio(ctx context.Context, uri string) error
}

// Options wires the catalog service.
type Options struct {
	Supervisor *engine.Supervisor
	Art        *artwork.Cache
	Enabled    bool
	Playback   Playback
}

// Service exposes the local audio engine, pairing and library browsing to the
// adapters. It never exposes stored playback credentials.
type Service struct {
	supervisor *engine.Supervisor
	art        *artwork.Cache
	enabled    bool
	playback   Playback
}

func NewService(options Options) *Service {
	playback := options.Playback
	if playback == nil {
		playback = enginePlayback{supervisor: options.Supervisor}
	}
	return &Service{
		supervisor: options.Supervisor,
		art:        options.Art,
		enabled:    options.Enabled,
		playback:   playback,
	}
}

// enginePlayback adapts the go-librespot supervisor client to the Playback
// seam. It preserves the behavior the catalog had before the seam existed.
type enginePlayback struct {
	supervisor *engine.Supervisor
}

func (e enginePlayback) PlayContext(ctx context.Context, uri, skipToURI string) error {
	client, err := engineClient(e.supervisor)
	if err != nil {
		return err
	}
	return client.Play(ctx, uri, skipToURI, false, 0)
}

func (e enginePlayback) PlayTrack(ctx context.Context, uri string) error {
	return e.PlayContext(ctx, uri, "")
}

func (e enginePlayback) AddToQueue(ctx context.Context, uri string) error {
	client, err := engineClient(e.supervisor)
	if err != nil {
		return err
	}
	return client.AddToQueue(ctx, uri)
}

func (e enginePlayback) StartRadio(ctx context.Context, uri string) error {
	client, err := engineClient(e.supervisor)
	if err != nil {
		return err
	}
	return client.StartRadio(ctx, uri)
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
			ErrorCode: engine.ErrorCodeDisabled,
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
	return s.libraryContext(tracks), nil
}

// libraryContext maps an engine context listing to the frontend model and
// rewrites its covers through the artwork cache.
func (s *Service) libraryContext(tracks engine.ContextTracks) LibraryContext {
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
	return result
}

// YourEpisodes returns the user's saved podcast episodes.
func (s *Service) YourEpisodes() (LibraryContext, error) {
	client, err := s.client()
	if err != nil {
		return LibraryContext{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	status, err := client.Status(ctx)
	if err != nil {
		return LibraryContext{}, err
	}
	if status == nil || strings.TrimSpace(status.Username) == "" {
		return LibraryContext{}, errors.New("the engine did not report a username")
	}
	tracks, err := client.ContextTracks(ctx, "spotify:user:"+status.Username+":collection:your-episodes")
	if err != nil {
		return LibraryContext{}, err
	}
	return s.libraryContext(tracks), nil
}

// PlayContext starts playback of a playlist, album, artist, show or Liked
// Songs collection. skipToURI selects a track inside the context.
func (s *Service) PlayContext(uri, skipToURI string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return s.playback.PlayContext(ctx, uri, skipToURI)
}

// PlayTrack starts playback of a single track URI.
func (s *Service) PlayTrack(uri string) error {
	return s.PlayContext(uri, "")
}

// AddToQueue appends a track to the active backend queue.
func (s *Service) AddToQueue(uri string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return s.playback.AddToQueue(ctx, uri)
}

// collectionSet, artistSet and showSet are the collection sets for saved
// media, followed artists and saved shows.
const (
	collectionSet = "collection"
	artistSet     = "artist"
	showSet       = "show"
)

// SetLiked adds or removes a track in the user's collection.
func (s *Service) SetLiked(uri string, liked bool) error {
	return s.collectionWrite(collectionSet, uri, !liked)
}

// IsLiked reports whether a track is in the user's collection.
func (s *Service) IsLiked(uri string) (bool, error) {
	return s.collectionContains(collectionSet, uri)
}

// SaveAlbum saves or removes an album in the user's collection.
func (s *Service) SaveAlbum(uri string, saved bool) error {
	return s.collectionWrite(collectionSet, uri, !saved)
}

// IsAlbumSaved reports whether an album is in the user's collection.
func (s *Service) IsAlbumSaved(uri string) (bool, error) {
	return s.collectionContains(collectionSet, uri)
}

// SaveShow saves or removes a show in the user's show set. The saved-shows
// listing UI lands in Phase 6.
func (s *Service) SaveShow(uri string, saved bool) error {
	return s.collectionWrite(showSet, uri, !saved)
}

// IsShowSaved reports whether a show is in the user's show set.
func (s *Service) IsShowSaved(uri string) (bool, error) {
	return s.collectionContains(showSet, uri)
}

// FollowArtist follows or unfollows an artist.
func (s *Service) FollowArtist(uri string, following bool) error {
	return s.collectionWrite(artistSet, uri, !following)
}

// IsArtistFollowed reports whether the user follows an artist.
func (s *Service) IsArtistFollowed(uri string) (bool, error) {
	return s.collectionContains(artistSet, uri)
}

// AddToPlaylist appends a track or episode to a playlist.
func (s *Service) AddToPlaylist(playlistURI, uri string) error {
	client, err := s.client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return client.AddToPlaylist(ctx, playlistURI, uri)
}

// RemoveFromPlaylist removes the first occurrence of a track or episode from a
// playlist.
func (s *Service) RemoveFromPlaylist(playlistURI, uri string) error {
	client, err := s.client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return client.RemoveFromPlaylist(ctx, playlistURI, uri)
}

func (s *Service) collectionWrite(set, uri string, remove bool) error {
	client, err := s.client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return client.CollectionWrite(ctx, set, []string{uri}, remove)
}

func (s *Service) collectionContains(set, uri string) (bool, error) {
	client, err := s.client()
	if err != nil {
		return false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	contains, err := client.CollectionContains(ctx, set, []string{uri})
	if err != nil {
		return false, err
	}
	if len(contains) != 1 {
		return false, errors.New("the engine returned an incomplete collection response")
	}
	return contains[0], nil
}

// Search finds tracks, albums, artists, playlists, shows and episodes in the
// Spotify catalog.
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
	for _, group := range []*engine.SearchGroup{&results.Tracks, &results.Albums, &results.Artists, &results.Playlists, &results.Shows, &results.Episodes} {
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return s.playback.StartRadio(ctx, uri)
}

func (s *Service) client() (*engine.Client, error) {
	// In web and Soloist modes go-librespot is only the catalog sidecar. Start
	// it on the first catalog call instead of keeping it resident from boot.
	if s.enabled && s.supervisor != nil && s.supervisor.Available() && s.supervisor.Client() == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := s.supervisor.Start(ctx); err != nil {
			return nil, err
		}
	}
	return engineClient(s.supervisor)
}

// engineClient returns the go-librespot client or the coded not-running error
// shared by the catalog and its default playback wrapper.
func engineClient(supervisor *engine.Supervisor) (*engine.Client, error) {
	client := supervisor.Client()
	if client == nil {
		return nil, engine.NewCodedError(
			engine.ErrorCodeNotRunning,
			"the local audio engine is not running",
		)
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
