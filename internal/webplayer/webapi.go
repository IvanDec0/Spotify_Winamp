package webplayer

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

const (
	defaultAPIBaseURL = "https://api.spotify.com/v1"
	defaultAPITimeout = 20 * time.Second
	maxAPIBody        = 8 << 20
	maxAPIErrorBody   = 1 << 20
	maxRetryAfter     = 10 * time.Second
)

// WebAPIOptions configures the Spotify Web API client.
type WebAPIOptions struct {
	BaseURL    string
	Tokens     TokenSource
	HTTPClient *http.Client
	Now        func() time.Time
	Sleep      func(ctx context.Context, d time.Duration) error
	MaxRetries int
}

// WebAPI is a loopback-independent Spotify Web API client.
type WebAPI struct {
	baseURL    string
	tokens     TokenSource
	client     *http.Client
	now        func() time.Time
	sleep      func(context.Context, time.Duration) error
	maxRetries int
}

// APIError is a typed Spotify Web API failure. It never carries request
// headers or tokens.
type APIError struct {
	Status     int
	Reason     string
	Message    string
	RetryAfter int
}

// Error implements error.
func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	switch {
	case e.Reason != "" && e.Message != "":
		return fmt.Sprintf("Spotify Web API HTTP %d (%s): %s", e.Status, e.Reason, e.Message)
	case e.Message != "":
		return fmt.Sprintf("Spotify Web API HTTP %d: %s", e.Status, e.Message)
	case e.Reason != "":
		return fmt.Sprintf("Spotify Web API HTTP %d (%s)", e.Status, e.Reason)
	default:
		return fmt.Sprintf("Spotify Web API HTTP %d", e.Status)
	}
}

type refreshingTokenSource interface {
	TokenSource
	Refresh(ctx context.Context) (string, time.Time, error)
}

type apiErrorResponse struct {
	Error struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
		Reason  string `json:"reason"`
	} `json:"error"`
}

type playerResponse struct {
	Device       *apiDevice `json:"device"`
	RepeatState  string     `json:"repeat_state"`
	ShuffleState bool       `json:"shuffle_state"`
	IsPlaying    bool       `json:"is_playing"`
	Item         *apiTrack  `json:"item"`
	ProgressMS   int        `json:"progress_ms"`
	Timestamp    int64      `json:"timestamp"`
}

type apiDevice struct {
	ID            string `json:"id"`
	IsActive      bool   `json:"is_active"`
	IsRestricted  bool   `json:"is_restricted"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	VolumePercent int    `json:"volume_percent"`
}

type apiTrack struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	URI        string      `json:"uri"`
	DurationMS int         `json:"duration_ms"`
	Artists    []apiArtist `json:"artists"`
	Album      *apiAlbum   `json:"album"`
}

type apiArtist struct {
	Name string `json:"name"`
}

type apiAlbum struct {
	Name   string     `json:"name"`
	Images []apiImage `json:"images"`
}

type apiImage struct {
	URL    string `json:"url"`
	Height int    `json:"height"`
	Width  int    `json:"width"`
}

type queueResponse struct {
	CurrentlyPlaying *apiTrack  `json:"currently_playing"`
	Queue            []apiTrack `json:"queue"`
}

type profile struct {
	ID          string     `json:"id"`
	DisplayName string     `json:"display_name"`
	Email       string     `json:"email"`
	Product     string     `json:"product"`
	Country     string     `json:"country"`
	Images      []apiImage `json:"images"`
}

type devicesResponse struct {
	Devices []apiDevice `json:"devices"`
}

// NewWebAPI applies defaults and returns a Spotify Web API client.
func NewWebAPI(opts WebAPIOptions) *WebAPI {
	baseURL := strings.TrimRight(strings.TrimSpace(opts.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultAPIBaseURL
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: defaultAPITimeout}
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Sleep == nil {
		opts.Sleep = sleepContext
	}
	maxRetries := opts.MaxRetries
	if maxRetries == 0 {
		maxRetries = 2
	}
	if maxRetries < 0 {
		maxRetries = 0
	}
	return &WebAPI{
		baseURL:    baseURL,
		tokens:     opts.Tokens,
		client:     client,
		now:        opts.Now,
		sleep:      opts.Sleep,
		maxRetries: maxRetries,
	}
}

// Player returns the current playback state, or nil when nothing is active.
func (a *WebAPI) Player(ctx context.Context) (*playerResponse, error) {
	var response playerResponse
	status, err := a.do(ctx, http.MethodGet, "/me/player", nil, nil, &response)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNoContent {
		return nil, nil
	}
	return &response, nil
}

// Devices lists the available Spotify Connect devices.
func (a *WebAPI) Devices(ctx context.Context) ([]apiDevice, error) {
	var response devicesResponse
	if _, err := a.do(ctx, http.MethodGet, "/me/player/devices", nil, nil, &response); err != nil {
		return nil, err
	}
	if response.Devices == nil {
		return []apiDevice{}, nil
	}
	return response.Devices, nil
}

// Queue returns the current item and the upcoming queue, or nil when nothing
// is active.
func (a *WebAPI) Queue(ctx context.Context) (*queueResponse, error) {
	var response queueResponse
	status, err := a.do(ctx, http.MethodGet, "/me/player/queue", nil, nil, &response)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNoContent {
		return nil, nil
	}
	return &response, nil
}

// Me returns the current Spotify profile.
func (a *WebAPI) Me(ctx context.Context) (*profile, error) {
	var response profile
	if _, err := a.do(ctx, http.MethodGet, "/me", nil, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// Transfer activates a Spotify Connect device without starting playback.
func (a *WebAPI) Transfer(ctx context.Context, deviceID string) error {
	body := struct {
		DeviceIDs []string `json:"device_ids"`
		Play      bool     `json:"play"`
	}{
		DeviceIDs: []string{deviceID},
		Play:      false,
	}
	_, err := a.do(ctx, http.MethodPut, "/me/player", nil, body, nil)
	return err
}

// Play starts or resumes playback.
func (a *WebAPI) Play(ctx context.Context, deviceID string, body any) error {
	_, err := a.do(ctx, http.MethodPut, "/me/player/play", deviceQuery(deviceID), body, nil)
	return err
}

// Pause pauses playback.
func (a *WebAPI) Pause(ctx context.Context, deviceID string) error {
	_, err := a.do(ctx, http.MethodPut, "/me/player/pause", deviceQuery(deviceID), nil, nil)
	return err
}

// Next skips to the next item.
func (a *WebAPI) Next(ctx context.Context, deviceID string) error {
	_, err := a.do(ctx, http.MethodPost, "/me/player/next", deviceQuery(deviceID), nil, nil)
	return err
}

// Previous skips to the previous item.
func (a *WebAPI) Previous(ctx context.Context, deviceID string) error {
	_, err := a.do(ctx, http.MethodPost, "/me/player/previous", deviceQuery(deviceID), nil, nil)
	return err
}

// Seek moves playback to positionMS.
func (a *WebAPI) Seek(ctx context.Context, deviceID string, positionMS int) error {
	query := deviceQuery(deviceID)
	query.Set("position_ms", strconv.Itoa(positionMS))
	_, err := a.do(ctx, http.MethodPut, "/me/player/seek", query, nil, nil)
	return err
}

// SetVolume sets the active device volume percentage.
func (a *WebAPI) SetVolume(ctx context.Context, deviceID string, percent int) error {
	query := deviceQuery(deviceID)
	query.Set("volume_percent", strconv.Itoa(percent))
	_, err := a.do(ctx, http.MethodPut, "/me/player/volume", query, nil, nil)
	return err
}

// SetShuffle enables or disables shuffle.
func (a *WebAPI) SetShuffle(ctx context.Context, deviceID string, enabled bool) error {
	query := deviceQuery(deviceID)
	query.Set("state", strconv.FormatBool(enabled))
	_, err := a.do(ctx, http.MethodPut, "/me/player/shuffle", query, nil, nil)
	return err
}

// SetRepeat sets the repeat mode: off, context or track.
func (a *WebAPI) SetRepeat(ctx context.Context, deviceID, mode string) error {
	query := deviceQuery(deviceID)
	query.Set("state", mode)
	_, err := a.do(ctx, http.MethodPut, "/me/player/repeat", query, nil, nil)
	return err
}

// AddToQueue appends a track or episode to the active device queue.
func (a *WebAPI) AddToQueue(ctx context.Context, deviceID, uri string) error {
	query := deviceQuery(deviceID)
	query.Set("uri", uri)
	_, err := a.do(ctx, http.MethodPost, "/me/player/queue", query, nil, nil)
	return err
}

func deviceQuery(deviceID string) url.Values {
	query := url.Values{}
	if deviceID = strings.TrimSpace(deviceID); deviceID != "" {
		query.Set("device_id", deviceID)
	}
	return query
}

// do sends one request, applies the retry and refresh policy and decodes a
// successful body into out when out is non-nil.
func (a *WebAPI) do(ctx context.Context, method, path string, query url.Values, body any, out any) (int, error) {
	if a == nil || a.tokens == nil {
		return 0, codedError(CodeAPI, "the Spotify token source is not configured", nil)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var payload []byte
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return 0, codedError(CodeInvalidArgument, "cannot encode the Spotify request", err)
		}
		payload = encoded
	}
	endpoint := a.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	refreshed := false
	refreshedToken := ""
	retries := 0
	for {
		token := refreshedToken
		if token == "" {
			var err error
			token, _, err = a.tokens.AccessToken(ctx)
			if err != nil {
				return 0, err
			}
		}
		request, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(payload))
		if err != nil {
			return 0, codedError(CodeAPI, "cannot prepare the Spotify request", err)
		}
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Accept", "application/json")
		if body != nil {
			request.Header.Set("Content-Type", "application/json")
		}

		response, err := a.client.Do(request)
		if err != nil {
			if response != nil {
				_ = response.Body.Close()
			}
			if ctxErr := ctx.Err(); ctxErr != nil {
				return 0, ctxErr
			}
			return 0, codedError(CodeAPI, "cannot reach the Spotify Web API", err)
		}

		if response.StatusCode >= 200 && response.StatusCode < 300 {
			status := response.StatusCode
			err := a.readSuccess(response, out)
			return status, err
		}

		apiErr := a.readAPIError(response)
		switch response.StatusCode {
		case http.StatusUnauthorized:
			refresher, ok := a.tokens.(refreshingTokenSource)
			if refreshed || !ok {
				return response.StatusCode, ErrLoginRequired
			}
			refreshed = true
			newToken, _, err := refresher.Refresh(ctx)
			if err != nil {
				if errors.Is(err, ErrLoginRequired) {
					return response.StatusCode, ErrLoginRequired
				}
				return response.StatusCode, err
			}
			refreshedToken = newToken
			continue
		case http.StatusTooManyRequests:
			if method != http.MethodGet || retries >= a.maxRetries {
				return response.StatusCode, apiErr
			}
			retries++
			delay := time.Duration(apiErr.RetryAfter) * time.Second
			if delay <= 0 {
				delay = time.Second
			}
			if delay > maxRetryAfter {
				delay = maxRetryAfter
			}
			if err := a.sleep(ctx, delay); err != nil {
				return response.StatusCode, err
			}
			continue
		case http.StatusForbidden:
			message := strings.ToLower(apiErr.Message + " " + apiErr.Reason)
			if strings.Contains(message, "premium") {
				return response.StatusCode, ErrPremiumRequired
			}
			return response.StatusCode, apiErr
		case http.StatusNotFound:
			if strings.EqualFold(apiErr.Reason, "NO_ACTIVE_DEVICE") {
				return response.StatusCode, ErrNoActiveDevice
			}
			return response.StatusCode, apiErr
		default:
			if response.StatusCode >= 500 && method == http.MethodGet && retries < a.maxRetries {
				retries++
				delay := time.Duration(250<<(retries-1)) * time.Millisecond
				if err := a.sleep(ctx, delay); err != nil {
					return response.StatusCode, err
				}
				continue
			}
			return response.StatusCode, apiErr
		}
	}
}

func (a *WebAPI) readSuccess(response *http.Response, out any) error {
	defer response.Body.Close()
	if response.StatusCode == http.StatusNoContent || out == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxAPIBody))
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxAPIBody))
	if err != nil {
		return codedError(CodeAPI, "cannot read the Spotify response", err)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return codedError(CodeAPI, "Spotify returned an invalid response", err)
	}
	return nil
}

func (a *WebAPI) readAPIError(response *http.Response) *APIError {
	defer response.Body.Close()
	apiErr := &APIError{Status: response.StatusCode}
	if raw := strings.TrimSpace(response.Header.Get("Retry-After")); raw != "" {
		if seconds, err := strconv.Atoi(raw); err == nil && seconds >= 0 {
			apiErr.RetryAfter = seconds
		}
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxAPIErrorBody))
	if err != nil {
		return apiErr
	}
	var payload apiErrorResponse
	if err := json.Unmarshal(body, &payload); err == nil {
		apiErr.Reason = payload.Error.Reason
		apiErr.Message = payload.Error.Message
		if apiErr.Status == 0 {
			apiErr.Status = payload.Error.Status
		}
	}
	return apiErr
}

// firstImageURL returns the first non-empty image URL in Spotify order.
func firstImageURL(images []apiImage) string {
	for _, image := range images {
		if trimmed := strings.TrimSpace(image.URL); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
