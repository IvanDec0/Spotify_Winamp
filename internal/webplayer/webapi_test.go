package webplayer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type recordedAPIRequest struct {
	method string
	path   string
	query  url.Values
	body   []byte
	auth   string
}

type apiRecorder struct {
	mu       sync.Mutex
	requests []recordedAPIRequest
}

func (r *apiRecorder) record(request *http.Request, body []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = append(r.requests, recordedAPIRequest{
		method: request.Method,
		path:   request.URL.Path,
		query:  request.URL.Query(),
		body:   append([]byte(nil), body...),
		auth:   request.Header.Get("Authorization"),
	})
}

func (r *apiRecorder) snapshot() []recordedAPIRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]recordedAPIRequest(nil), r.requests...)
}

func newAPITestServer(t *testing.T, respond func(http.ResponseWriter, *http.Request, []byte)) (*httptest.Server, *apiRecorder) {
	t.Helper()
	recorder := &apiRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			http.Error(writer, "read error", http.StatusInternalServerError)
			return
		}
		recorder.record(request, body)
		respond(writer, request, body)
	}))
	t.Cleanup(server.Close)
	return server, recorder
}

func webAPIForTest(t *testing.T, baseURL string, tokens TokenSource, sleep func(context.Context, time.Duration) error) *WebAPI {
	t.Helper()
	return NewWebAPI(WebAPIOptions{
		BaseURL:    baseURL,
		Tokens:     tokens,
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		Sleep:      sleep,
		MaxRetries: 2,
	})
}

type fakeRefreshingTokens struct {
	mu           sync.Mutex
	token        string
	nextToken    string
	refreshErr   error
	refreshCount int
}

func (f *fakeRefreshingTokens) AccessToken(context.Context) (string, time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.token, time.Now().Add(time.Hour), nil
}

func (f *fakeRefreshingTokens) Refresh(context.Context) (string, time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refreshCount++
	if f.refreshErr != nil {
		return "", time.Time{}, f.refreshErr
	}
	f.token = f.nextToken
	return f.token, time.Now().Add(time.Hour), nil
}

func (f *fakeRefreshingTokens) refreshes() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.refreshCount
}

type sleepRecorder struct {
	mu        sync.Mutex
	durations []time.Duration
}

func (s *sleepRecorder) sleep(_ context.Context, duration time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.durations = append(s.durations, duration)
	return nil
}

func (s *sleepRecorder) snapshot() []time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]time.Duration(nil), s.durations...)
}

func TestWebAPIPlayerAndDevices(t *testing.T) {
	server, _ := newAPITestServer(t, func(writer http.ResponseWriter, request *http.Request, _ []byte) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/me/player":
			_, _ = writer.Write([]byte(`{
				"device":{"id":"dev-1","is_active":true,"is_restricted":false,"name":"Cadence Web","type":"Computer","volume_percent":42},
				"repeat_state":"context","shuffle_state":true,"is_playing":true,
				"progress_ms":1234,
				"item":{"id":"track-1","name":"Song","uri":"spotify:track:1","duration_ms":300000,
					"artists":[{"name":"Artist One"},{"name":"Artist Two"}],
					"album":{"name":"Album","images":[{"url":""},{"url":"https://i.scdn.co/image/cover"}]}}
			}`))
		case "/me/player/devices":
			_, _ = writer.Write([]byte(`{"devices":[
				{"id":"dev-1","is_active":true,"is_restricted":false,"name":"Cadence Web","type":"Computer","volume_percent":42},
				{"id":"dev-2","is_active":false,"is_restricted":true,"name":"Phone","type":"Smartphone","volume_percent":80}
			]}`))
		default:
			http.NotFound(writer, request)
		}
	})

	api := webAPIForTest(t, server.URL, fakeTokenSource{token: "token", expiry: time.Now().Add(time.Hour)}, nil)
	player, err := api.Player(context.Background())
	if err != nil {
		t.Fatalf("Player() error = %v", err)
	}
	if player == nil || player.Item == nil || player.Device == nil {
		t.Fatalf("Player() = %#v", player)
	}
	if player.Item.ID != "track-1" || player.Item.Name != "Song" || player.Item.DurationMS != 300000 {
		t.Fatalf("player item = %#v", player.Item)
	}
	if player.ProgressMS != 1234 || !player.IsPlaying || !player.ShuffleState || player.RepeatState != "context" {
		t.Fatalf("player = %#v", player)
	}

	devices, err := api.Devices(context.Background())
	if err != nil {
		t.Fatalf("Devices() error = %v", err)
	}
	if len(devices) != 2 || devices[1].ID != "dev-2" || !devices[1].IsRestricted {
		t.Fatalf("devices = %#v", devices)
	}
}

func TestWebAPIEmptyResponses(t *testing.T) {
	server, _ := newAPITestServer(t, func(writer http.ResponseWriter, request *http.Request, _ []byte) {
		writer.WriteHeader(http.StatusNoContent)
	})
	api := webAPIForTest(t, server.URL, fakeTokenSource{token: "token", expiry: time.Now().Add(time.Hour)}, nil)

	player, err := api.Player(context.Background())
	if err != nil || player != nil {
		t.Fatalf("Player() = %#v, %v; want nil, nil", player, err)
	}
	queue, err := api.Queue(context.Background())
	if err != nil || queue != nil {
		t.Fatalf("Queue() = %#v, %v; want nil, nil", queue, err)
	}
}

func TestWebAPIQueueAndMe(t *testing.T) {
	server, _ := newAPITestServer(t, func(writer http.ResponseWriter, request *http.Request, _ []byte) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/me/player/queue":
			_, _ = writer.Write([]byte(`{
				"currently_playing":{"id":"track-1","name":"Current","uri":"spotify:track:1","duration_ms":1000,"artists":[{"name":"A"}],"album":{"name":"Album","images":[]}},
				"queue":[{"id":"track-2","name":"Next","uri":"spotify:track:2","duration_ms":2000,"artists":[{"name":"B"}],"album":{"name":"Album","images":[]}}]
			}`))
		case "/me":
			_, _ = writer.Write([]byte(`{"id":"user","display_name":"User","email":"user@example.com","product":"premium","country":"AR"}`))
		default:
			http.NotFound(writer, request)
		}
	})
	api := webAPIForTest(t, server.URL, fakeTokenSource{token: "token", expiry: time.Now().Add(time.Hour)}, nil)

	queue, err := api.Queue(context.Background())
	if err != nil {
		t.Fatalf("Queue() error = %v", err)
	}
	if queue == nil || queue.CurrentlyPlaying == nil || len(queue.Queue) != 1 {
		t.Fatalf("queue = %#v", queue)
	}
	profile, err := api.Me(context.Background())
	if err != nil {
		t.Fatalf("Me() error = %v", err)
	}
	if profile.ID != "user" || profile.Product != "premium" {
		t.Fatalf("profile = %#v", profile)
	}
}

func TestWebAPICommands(t *testing.T) {
	commands := []struct {
		name   string
		call   func(*WebAPI) error
		method string
		path   string
		query  url.Values
		body   any
	}{
		{
			name:   "transfer",
			call:   func(api *WebAPI) error { return api.Transfer(context.Background(), "dev") },
			method: http.MethodPut,
			path:   "/me/player",
			query:  url.Values{},
			body:   map[string]any{"device_ids": []any{"dev"}, "play": false},
		},
		{
			name: "play",
			call: func(api *WebAPI) error {
				return api.Play(context.Background(), "dev", map[string]any{"uris": []string{"spotify:track:1"}})
			},
			method: http.MethodPut,
			path:   "/me/player/play",
			query:  url.Values{"device_id": {"dev"}},
			body:   map[string]any{"uris": []any{"spotify:track:1"}},
		},
		{
			name:   "pause",
			call:   func(api *WebAPI) error { return api.Pause(context.Background(), "dev") },
			method: http.MethodPut,
			path:   "/me/player/pause",
			query:  url.Values{"device_id": {"dev"}},
		},
		{
			name:   "next",
			call:   func(api *WebAPI) error { return api.Next(context.Background(), "dev") },
			method: http.MethodPost,
			path:   "/me/player/next",
			query:  url.Values{"device_id": {"dev"}},
		},
		{
			name:   "previous",
			call:   func(api *WebAPI) error { return api.Previous(context.Background(), "") },
			method: http.MethodPost,
			path:   "/me/player/previous",
			query:  url.Values{},
		},
		{
			name:   "seek",
			call:   func(api *WebAPI) error { return api.Seek(context.Background(), "dev", 1500) },
			method: http.MethodPut,
			path:   "/me/player/seek",
			query:  url.Values{"device_id": {"dev"}, "position_ms": {"1500"}},
		},
		{
			name:   "volume",
			call:   func(api *WebAPI) error { return api.SetVolume(context.Background(), "", 55) },
			method: http.MethodPut,
			path:   "/me/player/volume",
			query:  url.Values{"volume_percent": {"55"}},
		},
		{
			name:   "shuffle",
			call:   func(api *WebAPI) error { return api.SetShuffle(context.Background(), "dev", true) },
			method: http.MethodPut,
			path:   "/me/player/shuffle",
			query:  url.Values{"device_id": {"dev"}, "state": {"true"}},
		},
		{
			name:   "repeat",
			call:   func(api *WebAPI) error { return api.SetRepeat(context.Background(), "", "track") },
			method: http.MethodPut,
			path:   "/me/player/repeat",
			query:  url.Values{"state": {"track"}},
		},
		{
			name:   "queue",
			call:   func(api *WebAPI) error { return api.AddToQueue(context.Background(), "dev", "spotify:track:2") },
			method: http.MethodPost,
			path:   "/me/player/queue",
			query:  url.Values{"device_id": {"dev"}, "uri": {"spotify:track:2"}},
		},
	}

	for _, status := range []int{http.StatusOK, http.StatusNoContent} {
		for _, command := range commands {
			t.Run(fmt.Sprintf("%s/%d", command.name, status), func(t *testing.T) {
				server, recorder := newAPITestServer(t, func(writer http.ResponseWriter, _ *http.Request, _ []byte) {
					writer.WriteHeader(status)
					if status == http.StatusOK {
						_, _ = writer.Write([]byte("short-receipt-body"))
					}
				})
				api := webAPIForTest(t, server.URL, fakeTokenSource{token: "token", expiry: time.Now().Add(time.Hour)}, nil)
				if err := command.call(api); err != nil {
					t.Fatalf("command error = %v", err)
				}
				requests := recorder.snapshot()
				if len(requests) != 1 {
					t.Fatalf("requests = %d, want 1", len(requests))
				}
				request := requests[0]
				if request.method != command.method || request.path != command.path {
					t.Fatalf("request = %s %s, want %s %s", request.method, request.path, command.method, command.path)
				}
				if !reflect.DeepEqual(request.query, command.query) {
					t.Fatalf("query = %v, want %v", request.query, command.query)
				}
				if request.auth != "Bearer token" {
					t.Fatalf("authorization = %q", request.auth)
				}
				if command.body == nil {
					if len(request.body) != 0 {
						t.Fatalf("body = %s, want empty", request.body)
					}
					return
				}
				var got any
				if err := json.Unmarshal(request.body, &got); err != nil {
					t.Fatalf("decode body: %v", err)
				}
				if !reflect.DeepEqual(got, command.body) {
					t.Fatalf("body = %#v, want %#v", got, command.body)
				}
			})
		}
	}
}

func TestWebAPIGetRateLimitRetries(t *testing.T) {
	var calls int
	server, _ := newAPITestServer(t, func(writer http.ResponseWriter, _ *http.Request, _ []byte) {
		calls++
		if calls == 1 {
			writer.Header().Set("Retry-After", "3")
			writer.WriteHeader(http.StatusTooManyRequests)
			_, _ = writer.Write([]byte(`{"error":{"status":429,"message":"rate limited"}}`))
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"devices":[]}`))
	})
	sleeper := &sleepRecorder{}
	api := webAPIForTest(t, server.URL, fakeTokenSource{token: "token", expiry: time.Now().Add(time.Hour)}, sleeper.sleep)

	if _, err := api.Devices(context.Background()); err != nil {
		t.Fatalf("Devices() error = %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
	durations := sleeper.snapshot()
	if len(durations) != 1 || durations[0] != 3*time.Second {
		t.Fatalf("sleeps = %v, want [3s]", durations)
	}
}

func TestWebAPICommandRateLimitDoesNotRetry(t *testing.T) {
	server, recorder := newAPITestServer(t, func(writer http.ResponseWriter, _ *http.Request, _ []byte) {
		writer.Header().Set("Retry-After", "4")
		writer.WriteHeader(http.StatusTooManyRequests)
		_, _ = writer.Write([]byte(`{"error":{"status":429,"message":"rate limited"}}`))
	})
	sleeper := &sleepRecorder{}
	api := webAPIForTest(t, server.URL, fakeTokenSource{token: "token", expiry: time.Now().Add(time.Hour)}, sleeper.sleep)

	err := api.Pause(context.Background(), "dev")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusTooManyRequests || apiErr.RetryAfter != 4 {
		t.Fatalf("Pause() error = %#v", err)
	}
	if len(recorder.snapshot()) != 1 {
		t.Fatalf("the command was retried: %d requests", len(recorder.snapshot()))
	}
	if len(sleeper.snapshot()) != 0 {
		t.Fatalf("the command slept: %v", sleeper.snapshot())
	}
}

func TestWebAPIGetServerErrorRetries(t *testing.T) {
	var calls int
	server, _ := newAPITestServer(t, func(writer http.ResponseWriter, _ *http.Request, _ []byte) {
		calls++
		if calls == 1 {
			writer.WriteHeader(http.StatusInternalServerError)
			_, _ = writer.Write([]byte(`{"error":{"status":500,"message":"boom"}}`))
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"devices":[]}`))
	})
	sleeper := &sleepRecorder{}
	api := webAPIForTest(t, server.URL, fakeTokenSource{token: "token", expiry: time.Now().Add(time.Hour)}, sleeper.sleep)
	if _, err := api.Devices(context.Background()); err != nil {
		t.Fatalf("Devices() error = %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
	if got := sleeper.snapshot(); len(got) != 1 || got[0] != 250*time.Millisecond {
		t.Fatalf("sleeps = %v, want [250ms]", got)
	}

	hour := 0
	commandServer, _ := newAPITestServer(t, func(writer http.ResponseWriter, _ *http.Request, _ []byte) {
		hour++
		writer.WriteHeader(http.StatusBadGateway)
		_, _ = writer.Write([]byte(`{"error":{"status":502,"message":"bad gateway"}}`))
	})
	commandAPI := webAPIForTest(t, commandServer.URL, fakeTokenSource{token: "token", expiry: time.Now().Add(time.Hour)}, sleeper.sleep)
	err := commandAPI.Pause(context.Background(), "")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusBadGateway {
		t.Fatalf("Pause() error = %#v", err)
	}
	if hour != 1 {
		t.Fatalf("the command was retried: %d requests", hour)
	}
}

func TestWebAPIUnauthorizedRefreshesOnce(t *testing.T) {
	tokens := &fakeRefreshingTokens{token: "old", nextToken: "new"}
	var calls int
	server, _ := newAPITestServer(t, func(writer http.ResponseWriter, request *http.Request, _ []byte) {
		calls++
		if request.Header.Get("Authorization") == "Bearer old" {
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"error":{"status":401,"message":"expired"}}`))
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	})
	api := webAPIForTest(t, server.URL, tokens, nil)
	if err := api.Pause(context.Background(), ""); err != nil {
		t.Fatalf("Pause() error = %v", err)
	}
	if calls != 2 || tokens.refreshes() != 1 {
		t.Fatalf("calls = %d refreshes = %d, want 2/1", calls, tokens.refreshes())
	}
}

func TestWebAPIUnauthorizedWithoutRefresh(t *testing.T) {
	server, _ := newAPITestServer(t, func(writer http.ResponseWriter, _ *http.Request, _ []byte) {
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"error":{"status":401,"message":"expired"}}`))
	})
	api := webAPIForTest(t, server.URL, fakeTokenSource{token: "token", expiry: time.Now().Add(time.Hour)}, nil)
	if err := api.Pause(context.Background(), ""); !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("Pause() error = %v, want ErrLoginRequired", err)
	}
}

func TestWebAPIUnauthorizedPersistent(t *testing.T) {
	tokens := &fakeRefreshingTokens{token: "old", nextToken: "new"}
	server, recorder := newAPITestServer(t, func(writer http.ResponseWriter, _ *http.Request, _ []byte) {
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"error":{"status":401,"message":"expired"}}`))
	})
	api := webAPIForTest(t, server.URL, tokens, nil)
	if err := api.Pause(context.Background(), ""); !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("Pause() error = %v, want ErrLoginRequired", err)
	}
	if len(recorder.snapshot()) != 2 {
		t.Fatalf("requests = %d, want 2", len(recorder.snapshot()))
	}
	if tokens.refreshes() != 1 {
		t.Fatalf("refreshes = %d, want 1", tokens.refreshes())
	}
}

func TestWebAPIErrorMapping(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{
			name:   "premium",
			status: http.StatusForbidden,
			body:   `{"error":{"status":403,"message":"Player command failed: Premium required","reason":"PREMIUM_REQUIRED"}}`,
			want:   ErrPremiumRequired,
		},
		{
			name:   "no active device",
			status: http.StatusNotFound,
			body:   `{"error":{"status":404,"message":"Device not found","reason":"NO_ACTIVE_DEVICE"}}`,
			want:   ErrNoActiveDevice,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server, _ := newAPITestServer(t, func(writer http.ResponseWriter, _ *http.Request, _ []byte) {
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(test.status)
				_, _ = writer.Write([]byte(test.body))
			})
			api := webAPIForTest(t, server.URL, fakeTokenSource{token: "token", expiry: time.Now().Add(time.Hour)}, nil)
			if err := api.Pause(context.Background(), ""); !errors.Is(err, test.want) {
				t.Fatalf("Pause() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestWebAPIErrorsNeverCarryTheToken(t *testing.T) {
	const secret = "BQsecret-token-value"
	server, _ := newAPITestServer(t, func(writer http.ResponseWriter, _ *http.Request, _ []byte) {
		writer.WriteHeader(http.StatusInternalServerError)
		_, _ = writer.Write([]byte(`{"error":{"status":500,"message":"server failed"}}`))
	})
	api := webAPIForTest(t, server.URL, fakeTokenSource{token: secret, expiry: time.Now().Add(time.Hour)}, nil)
	err := api.Pause(context.Background(), "")
	if err == nil {
		t.Fatal("Pause() error = nil")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("the error leaked the access token: %v", err)
	}
}
