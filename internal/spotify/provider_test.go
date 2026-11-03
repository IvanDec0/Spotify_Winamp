package spotify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"cadence/internal/artcache"
	"cadence/internal/player"
)

const playbackJSON = `{
  "is_playing": true,
  "progress_ms": 1000,
  "device": {"id":"device-1","name":"Studio","type":"speaker","is_active":true,"volume_percent":64,"is_restricted":false},
  "item": {"id":"track-1","name":"Track One","uri":"spotify:track:track-1","duration_ms":200000,"artists":[{"name":"Artist One"}],"album":{"name":"Album One","images":[{"url":"https://images.example/cover.jpg"}]}},
  "repeat_state": "context",
  "shuffle_state": true,
  "currently_playing_type": "track",
  "actions": {"disallows": {}}
}`

type providerHarness struct {
	provider *Provider
	server   *httptest.Server
	mu       sync.Mutex
	paths    []string
}

func (h *providerHarness) calls(path string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	count := 0
	for _, item := range h.paths {
		if item == path {
			count++
		}
	}
	return count
}

func newProviderHarness(t *testing.T, handler func(writer http.ResponseWriter, request *http.Request)) (*providerHarness, *artcache.Cache) {
	t.Helper()
	harness := &providerHarness{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		harness.mu.Lock()
		harness.paths = append(harness.paths, request.URL.Path)
		harness.mu.Unlock()
		handler(writer, request)
	}))
	t.Cleanup(server.Close)
	tokenServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"access_token": "refreshed",
			"expires_in":   3600,
		})
	}))
	t.Cleanup(tokenServer.Close)
	tokens := NewTokenSource("client", "refresh", tokenServer.URL, server.Client(), nil)
	tokens.Seed("access", "refresh", time.Now().Add(time.Hour))
	client := NewClientWithBase(tokens, server.URL, server.Client())
	art := artcache.New(server.Client())
	harness.server = server
	harness.provider = NewProvider(client, art, nil)
	return harness, art
}

func writeJSON(writer http.ResponseWriter, status int, body string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(body))
}

func TestProviderMapsPlaybackDevicesAndQueue(t *testing.T) {
	harness, _ := newProviderHarness(t, func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/me/player":
			writeJSON(writer, http.StatusOK, playbackJSON)
		case "/me/player/devices":
			writeJSON(writer, http.StatusOK, `{"devices":[{"id":"device-1","name":"Studio","type":"speaker","is_active":true,"volume_percent":64},{"id":"device-2","name":"Kitchen","type":"speaker","is_active":false,"volume_percent":20}]}`)
		case "/me/player/queue":
			writeJSON(writer, http.StatusOK, `{"currently_playing":{"id":"track-1","name":"Track One","duration_ms":200000,"artists":[{"name":"Artist One"}],"album":{"name":"Album One"}},"queue":[{"id":"track-2","name":"Track Two","duration_ms":120000,"artists":[{"name":"Artist Two"}],"album":{"name":"Album Two"}}]}`)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	})
	now := time.Now()
	harness.provider.SetNow(func() time.Time { return now })

	state, err := harness.provider.State(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.Provider != "spotify" || state.Status != player.StatusPlaying {
		t.Fatalf("state = %#v", state)
	}
	if state.Track.Title != "Track One" || state.Track.Album != "Album One" || len(state.Track.Artists) != 1 {
		t.Fatalf("track = %#v", state.Track)
	}
	if !strings.HasPrefix(state.Track.Cover, "/art/") {
		t.Fatalf("cover = %q", state.Track.Cover)
	}
	if state.Volume != 64 || state.Shuffle != true || state.Repeat != player.RepeatContext {
		t.Fatalf("controls = %#v", state)
	}
	if len(state.Devices) != 2 || !state.Devices[0].Active {
		t.Fatalf("devices = %#v", state.Devices)
	}
	if len(state.Queue) != 2 || !state.Queue[0].IsCurrent || !state.Queue[1].IsNext {
		t.Fatalf("queue = %#v", state.Queue)
	}
	if !state.CanControl || !state.CanSeek || state.Error != nil {
		t.Fatalf("capabilities = %#v", state)
	}

	// The cached state is reused inside the playing refresh interval.
	if _, err := harness.provider.State(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls := harness.calls("/me/player"); calls != 1 {
		t.Fatalf("playback calls = %d, want 1", calls)
	}

	now = now.Add(2 * time.Second)
	if _, err := harness.provider.State(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls := harness.calls("/me/player"); calls != 2 {
		t.Fatalf("playback calls = %d, want 2", calls)
	}

}

func TestProviderNoActiveDevice(t *testing.T) {
	harness, _ := newProviderHarness(t, func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/me/player":
			writer.WriteHeader(http.StatusNoContent)
		case "/me/player/devices":
			writeJSON(writer, http.StatusOK, `{"devices":[{"id":"device-1","name":"Studio","type":"speaker","is_active":false,"volume_percent":50}]}`)
		default:
			writer.WriteHeader(http.StatusNoContent)
		}
	})
	now := time.Now()
	harness.provider.SetNow(func() time.Time { return now })

	state, err := harness.provider.State(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != player.StatusStopped || state.Error == nil || state.Error.Code != "no_active_device" {
		t.Fatalf("state = %#v", state)
	}
	if len(state.Devices) != 1 || state.CanControl {
		t.Fatalf("devices/capabilities = %#v", state)
	}
}

func TestProviderRateLimitAndBackoff(t *testing.T) {
	harness, _ := newProviderHarness(t, func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/me/player" {
			writer.Header().Set("Retry-After", "3")
			writeJSON(writer, http.StatusTooManyRequests, `{"error":{"status":429,"message":"slow down"}}`)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	})
	now := time.Now()
	harness.provider.SetNow(func() time.Time { return now })

	state, err := harness.provider.State(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.Error == nil || state.Error.Code != "rate_limited" || state.Error.RetryAfterSeconds != 3 {
		t.Fatalf("error = %#v", state.Error)
	}
	if calls := harness.calls("/me/player"); calls != 1 {
		t.Fatalf("calls = %d", calls)
	}
	now = now.Add(2 * time.Second)
	if _, err := harness.provider.State(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls := harness.calls("/me/player"); calls != 1 {
		t.Fatalf("calls during backoff = %d", calls)
	}
	now = now.Add(2 * time.Second)
	if _, err := harness.provider.State(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls := harness.calls("/me/player"); calls != 2 {
		t.Fatalf("calls after backoff = %d", calls)
	}
}

func TestProviderAuthFailureNotifiesOnce(t *testing.T) {
	var authNotified atomic.Int32
	harness, _ := newProviderHarness(t, func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/me/player" {
			writeJSON(writer, http.StatusUnauthorized, `{"error":{"status":401,"message":"expired"}}`)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	})
	harness.provider.onAuthProblem = func(error) {
		authNotified.Add(1)
	}
	now := time.Now()
	harness.provider.SetNow(func() time.Time { return now })

	state, err := harness.provider.State(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.Error == nil || state.Error.Code != "auth_expired" {
		t.Fatalf("state error = %#v", state.Error)
	}
	if authNotified.Load() != 1 {
		t.Fatalf("auth notifications = %d", authNotified.Load())
	}
	now = now.Add(time.Second)
	if _, err := harness.provider.State(context.Background()); err != nil {
		t.Fatal(err)
	}
	if authNotified.Load() != 1 {
		t.Fatalf("auth notifications = %d", authNotified.Load())
	}
}

func TestProviderOptimisticActions(t *testing.T) {
	var pauseCalls atomic.Int32
	harness, _ := newProviderHarness(t, func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/me/player":
			writeJSON(writer, http.StatusOK, playbackJSON)
		case "/me/player/pause":
			pauseCalls.Add(1)
			writer.WriteHeader(http.StatusNoContent)
		case "/me/player/seek":
			writer.WriteHeader(http.StatusNoContent)
		case "/me/player/volume":
			writer.WriteHeader(http.StatusNoContent)
		case "/me/player/devices", "/me/player/queue":
			writeJSON(writer, http.StatusOK, `{}`)
		default:
			writer.WriteHeader(http.StatusNoContent)
		}
	})
	now := time.Now()
	harness.provider.SetNow(func() time.Time { return now })
	if _, err := harness.provider.State(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := harness.provider.Pause(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := harness.provider.Seek(context.Background(), 5000); err != nil {
		t.Fatal(err)
	}
	if err := harness.provider.SetVolume(context.Background(), 33); err != nil {
		t.Fatal(err)
	}
	state, err := harness.provider.State(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != player.StatusPaused || state.ProgressMS != 5000 || state.Volume != 33 {
		t.Fatalf("optimistic state = %#v", state)
	}
	if pauseCalls.Load() != 1 {
		t.Fatalf("pause calls = %d", pauseCalls.Load())
	}
}

func TestProviderNetworkFailureUsesBackoff(t *testing.T) {
	tokens := NewTokenSource("client", "refresh", "https://token.invalid", nil, nil)
	tokens.Seed("access", "refresh", time.Now().Add(time.Hour))
	client := NewClientWithBase(tokens, "http://127.0.0.1:1", &http.Client{Timeout: time.Second})
	provider := NewProvider(client, artcache.New(nil), nil)
	now := time.Now()
	provider.SetNow(func() time.Time { return now })

	state, err := provider.State(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.Error == nil || state.Error.Code != "network" || !state.Error.Retryable {
		t.Fatalf("network error state = %#v", state.Error)
	}
	if state.CanControl || state.CanSeek {
		t.Fatalf("capabilities = %#v", state)
	}
}

func TestProviderMapsEpisodesAndEmptyQueue(t *testing.T) {
	harness, _ := newProviderHarness(t, func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/me/player":
			writeJSON(writer, http.StatusOK, `{"is_playing":false,"progress_ms":0,"device":{"id":"d","name":"Phone","type":"smartphone","is_active":true,"volume_percent":10},"item":{"id":"episode","name":"Episode","uri":"spotify:episode:1","duration_ms":60000,"type":"episode","show":{"name":"Podcast","publisher":"Publisher","images":[{"url":"https://images.example/show.jpg"}]}},"repeat_state":"off","shuffle_state":false,"currently_playing_type":"episode","actions":{"disallows":{}}}`)
		case "/me/player/devices":
			writeJSON(writer, http.StatusOK, `{"devices":[]}`)
		case "/me/player/queue":
			writeJSON(writer, http.StatusOK, `{"currently_playing":null,"queue":[]}`)
		}
	})
	harness.provider.SetNow(func() time.Time { return time.Now() })
	state, err := harness.provider.State(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.Track.Album != "Podcast" || len(state.Track.Artists) != 1 || state.Track.Artists[0] != "Publisher" {
		t.Fatalf("episode track = %#v", state.Track)
	}
	if len(state.Queue) != 0 {
		t.Fatalf("queue = %#v", state.Queue)
	}
}
