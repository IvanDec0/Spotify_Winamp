package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// APIError is a local go-librespot API failure.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("go-librespot API returned status %d", e.StatusCode)
}

// Client talks to the loopback go-librespot API.
type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 12 * time.Second}
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: httpClient}
}

func (c *Client) Root(ctx context.Context) (Root, error) {
	var root Root
	err := c.request(ctx, http.MethodGet, "/", nil, nil, &root)
	return root, err
}

func (c *Client) Status(ctx context.Context) (*Status, error) {
	var status Status
	found, err := c.requestFound(ctx, http.MethodGet, "/status", nil, nil, &status)
	if err != nil || !found {
		return nil, err
	}
	return &status, nil
}

func (c *Client) AuthCode(ctx context.Context) (*authCode, error) {
	var code authCode
	found, err := c.requestFound(ctx, http.MethodGet, "/auth/code", nil, nil, &code)
	if err != nil || !found {
		return nil, err
	}
	return &code, nil
}

func (c *Client) Play(ctx context.Context, uri, skipToURI string, paused bool, position int64) error {
	body := map[string]any{"uri": uri}
	if skipToURI != "" {
		body["skip_to_uri"] = skipToURI
	}
	if paused {
		body["paused"] = true
	}
	if position > 0 {
		body["position"] = position
	}
	return c.request(ctx, http.MethodPost, "/player/play", nil, body, nil)
}

func (c *Client) Resume(ctx context.Context) error {
	return c.request(ctx, http.MethodPost, "/player/resume", nil, nil, nil)
}

func (c *Client) Pause(ctx context.Context) error {
	return c.request(ctx, http.MethodPost, "/player/pause", nil, nil, nil)
}

func (c *Client) PlayPause(ctx context.Context) error {
	return c.request(ctx, http.MethodPost, "/player/playpause", nil, nil, nil)
}

func (c *Client) Next(ctx context.Context) error {
	return c.request(ctx, http.MethodPost, "/player/next", nil, nil, nil)
}

func (c *Client) Previous(ctx context.Context) error {
	return c.request(ctx, http.MethodPost, "/player/prev", nil, nil, nil)
}

func (c *Client) Seek(ctx context.Context, positionMS int64) error {
	return c.request(ctx, http.MethodPost, "/player/seek", nil, map[string]any{"position": positionMS}, nil)
}

func (c *Client) Volume(ctx context.Context) (uint32, uint32, error) {
	var volume valueResponse
	if err := c.request(ctx, http.MethodGet, "/player/volume", nil, nil, &volume); err != nil {
		return 0, 0, err
	}
	return volume.Value, volume.Max, nil
}

func (c *Client) SetVolume(ctx context.Context, volume int) error {
	return c.request(ctx, http.MethodPost, "/player/volume", nil, map[string]any{"volume": volume}, nil)
}

func (c *Client) SetShuffle(ctx context.Context, enabled bool) error {
	return c.request(ctx, http.MethodPost, "/player/shuffle_context", nil, map[string]any{"shuffle_context": enabled}, nil)
}

func (c *Client) SetRepeatContext(ctx context.Context, enabled bool) error {
	return c.request(ctx, http.MethodPost, "/player/repeat_context", nil, map[string]any{"repeat_context": enabled}, nil)
}

func (c *Client) SetRepeatTrack(ctx context.Context, enabled bool) error {
	return c.request(ctx, http.MethodPost, "/player/repeat_track", nil, map[string]any{"repeat_track": enabled}, nil)
}

func (c *Client) AddToQueue(ctx context.Context, uri string) error {
	return c.request(ctx, http.MethodPost, "/player/add_to_queue", nil, map[string]any{"uri": uri}, nil)
}

// Search queries the engine's internal Spotify search.
func (c *Client) Search(ctx context.Context, query string, limit int) (SearchResults, error) {
	var results SearchResults
	params := url.Values{
		"q":     {query},
		"limit": {strconv.Itoa(limit)},
	}
	err := c.request(ctx, http.MethodGet, "/search", params, nil, &results)
	return results, err
}

// StartRadio asks the engine to build and play a Spotify radio station from a
// seed uri (track, album, artist or playlist).
func (c *Client) StartRadio(ctx context.Context, uri string) error {
	return c.request(ctx, http.MethodPost, "/player/radio", nil, map[string]any{"uri": uri}, nil)
}

type rawPlaylistPage struct {
	Total  int `json:"total"`
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
	Items  []struct {
		URI           string   `json:"uri"`
		Name          string   `json:"name"`
		Description   string   `json:"description"`
		OwnerUsername string   `json:"owner_username"`
		Length        int      `json:"length"`
		ImageURL      *string  `json:"image_url"`
		Collaborative bool     `json:"collaborative"`
		CanEdit       bool     `json:"can_edit"`
		Folder        []string `json:"folder"`
	} `json:"items"`
}

func (c *Client) Playlists(ctx context.Context, offset, limit int) (PlaylistPage, error) {
	var raw rawPlaylistPage
	query := url.Values{"offset": {strconv.Itoa(offset)}, "limit": {strconv.Itoa(limit)}}
	if err := c.request(ctx, http.MethodGet, "/library/playlists", query, nil, &raw); err != nil {
		return PlaylistPage{}, err
	}
	page := PlaylistPage{Total: raw.Total, Offset: raw.Offset, Limit: raw.Limit}
	for _, item := range raw.Items {
		image := ""
		if item.ImageURL != nil {
			image = *item.ImageURL
		}
		page.Items = append(page.Items, LibraryPlaylist{
			URI: item.URI, Name: item.Name, Description: item.Description, Owner: item.OwnerUsername,
			Length: item.Length, ImageURL: image, Collaborative: item.Collaborative,
			CanEdit: item.CanEdit, Folder: append([]string(nil), item.Folder...),
		})
	}
	return page, nil
}

type rawAlbumPage struct {
	Total  int `json:"total"`
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
	Items  []struct {
		URI         string   `json:"uri"`
		Name        string   `json:"name"`
		ArtistNames []string `json:"artist_names"`
		Year        int      `json:"year"`
		ImageURL    *string  `json:"image_url"`
	} `json:"items"`
}

func (c *Client) Albums(ctx context.Context, offset, limit int) (AlbumPage, error) {
	var raw rawAlbumPage
	query := url.Values{"offset": {strconv.Itoa(offset)}, "limit": {strconv.Itoa(limit)}}
	if err := c.request(ctx, http.MethodGet, "/library/albums", query, nil, &raw); err != nil {
		return AlbumPage{}, err
	}
	page := AlbumPage{Total: raw.Total, Offset: raw.Offset, Limit: raw.Limit}
	for _, item := range raw.Items {
		image := ""
		if item.ImageURL != nil {
			image = *item.ImageURL
		}
		page.Items = append(page.Items, LibraryAlbum{
			URI: item.URI, Name: item.Name, ArtistNames: append([]string(nil), item.ArtistNames...), Year: item.Year, ImageURL: image,
		})
	}
	return page, nil
}

type rawArtistPage struct {
	Total  int `json:"total"`
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
	Items  []struct {
		URI      string  `json:"uri"`
		Name     string  `json:"name"`
		ImageURL *string `json:"image_url"`
	} `json:"items"`
}

func (c *Client) Artists(ctx context.Context, offset, limit int) (ArtistPage, error) {
	var raw rawArtistPage
	query := url.Values{"offset": {strconv.Itoa(offset)}, "limit": {strconv.Itoa(limit)}}
	if err := c.request(ctx, http.MethodGet, "/library/artists", query, nil, &raw); err != nil {
		return ArtistPage{}, err
	}
	page := ArtistPage{Total: raw.Total, Offset: raw.Offset, Limit: raw.Limit}
	for _, item := range raw.Items {
		image := ""
		if item.ImageURL != nil {
			image = *item.ImageURL
		}
		page.Items = append(page.Items, LibraryArtist{URI: item.URI, Name: item.Name, ImageURL: image})
	}
	return page, nil
}

type rawContextTracks struct {
	URI    string `json:"uri"`
	Ready  bool   `json:"ready"`
	Length int    `json:"length"`
	Cached int    `json:"cached"`
	Tracks []struct {
		URI   string `json:"uri"`
		Track *Track `json:"track"`
	} `json:"tracks"`
}

func (c *Client) ContextTracks(ctx context.Context, uri string) (ContextTracks, error) {
	var raw rawContextTracks
	query := url.Values{"uri": {uri}}
	if err := c.request(ctx, http.MethodGet, "/context/tracks", query, nil, &raw); err != nil {
		return ContextTracks{}, err
	}
	result := ContextTracks{URI: raw.URI, Ready: raw.Ready, Length: raw.Length, Cached: raw.Cached}
	for _, item := range raw.Tracks {
		track := Track{URI: item.URI}
		if item.Track != nil {
			track = *item.Track
		}
		result.Items = append(result.Items, ContextTrack{URI: item.URI, Track: track})
	}
	return result, nil
}

func (c *Client) SetDeviceName(ctx context.Context, name string) error {
	return c.request(ctx, http.MethodPost, "/set_device_name", nil, map[string]any{"name": name}, nil)
}

func (c *Client) request(ctx context.Context, method, path string, query url.Values, body, out any) error {
	_, err := c.requestFound(ctx, method, path, query, body, out)
	return err
}

func (c *Client) requestFound(ctx context.Context, method, path string, query url.Values, body, out any) (bool, error) {
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return false, err
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return false, err
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(request)
	if err != nil {
		return false, fmt.Errorf("go-librespot is unavailable: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNoContent {
		return false, nil
	}
	data, readErr := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if readErr != nil {
		return false, fmt.Errorf("read go-librespot response: %w", readErr)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := strings.TrimSpace(string(data))
		if len(message) > 300 {
			message = message[:300]
		}
		if message == "" {
			message = http.StatusText(response.StatusCode)
		}
		return false, &APIError{StatusCode: response.StatusCode, Message: message}
	}
	if out == nil {
		return true, nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return false, &APIError{StatusCode: response.StatusCode, Message: "go-librespot returned an invalid response"}
	}
	return true, nil
}
