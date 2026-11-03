package spotify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultAPIBase = "https://api.spotify.com/v1"

type Image struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type Artist struct {
	Name string `json:"name"`
}

type Album struct {
	Name   string  `json:"name"`
	Images []Image `json:"images"`
}

type Show struct {
	Name      string  `json:"name"`
	Publisher string  `json:"publisher"`
	Images    []Image `json:"images"`
}

type TrackDTO struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	URI        string   `json:"uri"`
	Type       string   `json:"type"`
	DurationMS int      `json:"duration_ms"`
	Artists    []Artist `json:"artists"`
	Album      Album    `json:"album"`
	Show       Show     `json:"show"`
}

type DeviceDTO struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	IsActive       bool   `json:"is_active"`
	IsRestricted   bool   `json:"is_restricted"`
	VolumePercent  int    `json:"volume_percent"`
	SupportsVolume bool   `json:"supports_volume"`
}

type PlaybackDTO struct {
	Device               *DeviceDTO `json:"device"`
	IsPlaying            bool       `json:"is_playing"`
	ProgressMS           int        `json:"progress_ms"`
	RepeatState          string     `json:"repeat_state"`
	ShuffleState         bool       `json:"shuffle_state"`
	Item                 *TrackDTO  `json:"item"`
	CurrentlyPlayingType string     `json:"currently_playing_type"`
	Actions              struct {
		Disallows map[string]bool `json:"disallows"`
	} `json:"actions"`
}

type DevicesResponse struct {
	Devices []DeviceDTO `json:"devices"`
}

type QueueResponse struct {
	CurrentlyPlaying *TrackDTO  `json:"currently_playing"`
	Queue            []TrackDTO `json:"queue"`
}

// Client is a small Spotify Web API client. It owns request/retry behavior but
// not playback state mapping.
type Client struct {
	baseURL string
	http    *http.Client
	tokens  *TokenSource
}

func NewClient(tokens *TokenSource, httpClient *http.Client) *Client {
	return NewClientWithBase(tokens, defaultAPIBase, httpClient)
}

func NewClientWithBase(tokens *TokenSource, baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    httpClient,
		tokens:  tokens,
	}
}

func (c *Client) Playback(ctx context.Context) (PlaybackDTO, bool, error) {
	var playback PlaybackDTO
	found, err := c.request(ctx, http.MethodGet, "/me/player", nil, nil, &playback)
	return playback, found, err
}

func (c *Client) Devices(ctx context.Context) ([]DeviceDTO, error) {
	var response DevicesResponse
	if _, err := c.request(ctx, http.MethodGet, "/me/player/devices", nil, nil, &response); err != nil {
		return nil, err
	}
	return response.Devices, nil
}

func (c *Client) Queue(ctx context.Context) (QueueResponse, error) {
	var response QueueResponse
	if _, err := c.request(ctx, http.MethodGet, "/me/player/queue", nil, nil, &response); err != nil {
		return QueueResponse{}, err
	}
	return response, nil
}

func (c *Client) Play(ctx context.Context) error {
	_, err := c.request(ctx, http.MethodPut, "/me/player/play", nil, nil, nil)
	return err
}

func (c *Client) Pause(ctx context.Context) error {
	_, err := c.request(ctx, http.MethodPut, "/me/player/pause", nil, nil, nil)
	return err
}

func (c *Client) Next(ctx context.Context) error {
	_, err := c.request(ctx, http.MethodPost, "/me/player/next", nil, nil, nil)
	return err
}

func (c *Client) Previous(ctx context.Context) error {
	_, err := c.request(ctx, http.MethodPost, "/me/player/previous", nil, nil, nil)
	return err
}

func (c *Client) Seek(ctx context.Context, positionMS int) error {
	query := url.Values{"position_ms": {strconv.Itoa(positionMS)}}
	_, err := c.request(ctx, http.MethodPut, "/me/player/seek", query, nil, nil)
	return err
}

func (c *Client) SetVolume(ctx context.Context, volume int) error {
	query := url.Values{"volume_percent": {strconv.Itoa(volume)}}
	_, err := c.request(ctx, http.MethodPut, "/me/player/volume", query, nil, nil)
	return err
}

func (c *Client) SetShuffle(ctx context.Context, enabled bool) error {
	query := url.Values{"state": {strconv.FormatBool(enabled)}}
	_, err := c.request(ctx, http.MethodPut, "/me/player/shuffle", query, nil, nil)
	return err
}

func (c *Client) SetRepeat(ctx context.Context, mode string) error {
	query := url.Values{"state": {mode}}
	_, err := c.request(ctx, http.MethodPut, "/me/player/repeat", query, nil, nil)
	return err
}

func (c *Client) Transfer(ctx context.Context, deviceID string) error {
	body := struct {
		DeviceIDs []string `json:"device_ids"`
	}{DeviceIDs: []string{deviceID}}
	_, err := c.request(ctx, http.MethodPut, "/me/player", nil, body, nil)
	return err
}

// request returns found=false for a 204 response. A 401 is retried once after
// invalidating the access token.
func (c *Client) request(ctx context.Context, method, path string, query url.Values, body, out any) (bool, error) {
	for attempt := 0; attempt < 2; attempt++ {
		accessToken, err := c.tokens.AccessToken(ctx)
		if err != nil {
			return false, err
		}
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
		request.Header.Set("Authorization", "Bearer "+accessToken)
		request.Header.Set("Accept", "application/json")
		request.Header.Set("User-Agent", "Cadence/0.1")
		if body != nil {
			request.Header.Set("Content-Type", "application/json")
		}

		response, err := c.http.Do(request)
		if err != nil {
			return false, fmt.Errorf("Spotify request failed: %w", err)
		}
		if response.StatusCode == http.StatusUnauthorized && attempt == 0 {
			_ = response.Body.Close()
			c.tokens.Invalidate()
			continue
		}
		found, err := decodeResponse(response, out)
		return found, err
	}
	return false, &APIError{StatusCode: http.StatusUnauthorized, Code: "unauthorized", Message: "Spotify authorization is no longer valid"}
}

func decodeResponse(response *http.Response, out any) (bool, error) {
	defer response.Body.Close()
	if response.StatusCode == http.StatusNoContent {
		return false, nil
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if readErr != nil {
		return false, fmt.Errorf("read Spotify response: %w", readErr)
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		if out == nil || len(bytes.TrimSpace(body)) == 0 {
			return true, nil
		}
		if err := json.Unmarshal(body, out); err != nil {
			return false, &APIError{StatusCode: response.StatusCode, Code: "invalid_response", Message: "Spotify returned an invalid response"}
		}
		return true, nil
	}

	code := "api_error"
	message := http.StatusText(response.StatusCode)
	envelope := struct {
		Error json.RawMessage `json:"error"`
	}{}
	if json.Unmarshal(body, &envelope) == nil && len(envelope.Error) > 0 {
		var object struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(envelope.Error, &object) == nil && object.Message != "" {
			message = object.Message
		} else {
			var text string
			if json.Unmarshal(envelope.Error, &text) == nil && text != "" {
				message = text
			}
		}
	}
	if len(message) > 300 {
		message = message[:300]
	}
	apiError := &APIError{StatusCode: response.StatusCode, Code: code, Message: message}
	switch response.StatusCode {
	case http.StatusUnauthorized:
		apiError.Code = "unauthorized"
	case http.StatusForbidden:
		apiError.Code = "forbidden"
	case http.StatusNotFound:
		apiError.Code = "not_found"
	case http.StatusTooManyRequests:
		apiError.Code = "rate_limited"
		apiError.RetryAfter = parseRetryAfter(response.Header.Get("Retry-After"))
		if apiError.RetryAfter <= 0 {
			apiError.RetryAfter = 2 * time.Second
		}
	default:
		if response.StatusCode >= 500 {
			apiError.Code = "server_error"
		}
	}
	return false, apiError
}

func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil {
		return time.Until(when)
	}
	return 0
}

func isAuthError(err error) bool {
	return errors.Is(err, ErrAuthRevoked) || errors.Is(err, ErrNotAuthenticated)
}
