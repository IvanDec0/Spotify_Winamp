package webplayer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"cadence/internal/artwork"
	"cadence/internal/playback"
)

type fakeProviderBackend struct {
	mu          sync.Mutex
	api         *WebAPI
	deviceID    string
	running     bool
	loggedIn    bool
	installed   bool
	ensureCalls int
	ensureErr   error
}

func (f *fakeProviderBackend) API() *WebAPI {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.api
}

func (f *fakeProviderBackend) DeviceID() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.deviceID
}

func (f *fakeProviderBackend) Running() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.running
}

func (f *fakeProviderBackend) LoggedIn() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.loggedIn
}

func (f *fakeProviderBackend) RuntimeInstalled() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.installed
}

func (f *fakeProviderBackend) EnsureStarted(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ensureCalls++
	if f.ensureErr != nil {
		return f.ensureErr
	}
	f.running = true
	if f.deviceID == "" {
		f.deviceID = "sdk-dev"
	}
	return nil
}

func (f *fakeProviderBackend) setLoggedIn(value bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.loggedIn = value
}

func (f *fakeProviderBackend) setRunning(value bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.running = value
}

func (f *fakeProviderBackend) ensureCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ensureCalls
}

func newProviderForTest(t *testing.T, backend *fakeProviderBackend, serverURL string, sleep func(context.Context, time.Duration) error) *Provider {
	t.Helper()
	backend.api = webAPIForTest(t, serverURL, fakeTokenSource{token: "token", expiry: time.Now().Add(time.Hour)}, sleep)
	return NewProvider(backend, artwork.New(http.DefaultClient))
}

func waitForProviderState(t *testing.T, states <-chan playback.PlayerState) playback.PlayerState {
	t.Helper()
	select {
	case state := <-states:
		return state
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for a provider state")
		return playback.PlayerState{}
	}
}

const testPlayerBody = `{
	"device":{"id":"player-dev","is_active":true,"is_restricted":false,"name":"Laptop","type":"Computer","volume_percent":37},
	"repeat_state":"track","shuffle_state":true,"is_playing":true,"progress_ms":500,
	"item":{"id":"track-1","name":"Song","uri":"spotify:track:1","duration_ms":300000,
		"artists":[{"name":"Artist One"},{"name":"Artist Two"}],
		"album":{"name":"Album","images":[{"url":""},{"url":"https://i.scdn.co/image/cover"}]}}
}`

const testDevicesBody = `{"devices":[
	{"id":"sdk-dev","is_active":true,"is_restricted":false,"name":"Cadence Web","type":"Computer","volume_percent":61},
	{"id":"phone","is_active":false,"is_restricted":true,"name":"Phone","type":"Smartphone","volume_percent":80}
]}`

func TestProviderStateMapping(t *testing.T) {
	server, _ := newAPITestServer(t, func(writer http.ResponseWriter, request *http.Request, _ []byte) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/me/player":
			_, _ = writer.Write([]byte(testPlayerBody))
		case "/me/player/devices":
			_, _ = writer.Write([]byte(testDevicesBody))
		default:
			http.NotFound(writer, request)
		}
	})
	backend := &fakeProviderBackend{deviceID: "sdk-dev", running: true, loggedIn: true, installed: true}
	provider := newProviderForTest(t, backend, server.URL, nil)

	state, err := provider.State(context.Background())
	if err != nil {
		t.Fatalf("State() error = %v", err)
	}
	if state.Provider != providerName || state.Status != playback.StatusPlaying {
		t.Fatalf("state = %#v", state)
	}
	if state.Track.ID != "track-1" || state.Track.Title != "Song" || state.Track.URI != "spotify:track:1" {
		t.Fatalf("track = %#v", state.Track)
	}
	if len(state.Track.Artists) != 2 || state.Track.Artists[0] != "Artist One" {
		t.Fatalf("artists = %#v", state.Track.Artists)
	}
	if state.Track.Album != "Album" || !strings.HasPrefix(state.Track.Cover, "/art/") {
		t.Fatalf("album/cover = %q/%q", state.Track.Album, state.Track.Cover)
	}
	if state.Track.DurationMS != 300000 || state.DurationMS != 300000 || state.ProgressMS != 500 {
		t.Fatalf("duration/progress = %d/%d/%d", state.Track.DurationMS, state.DurationMS, state.ProgressMS)
	}
	if state.Volume != 37 || !state.Shuffle || state.Repeat != playback.RepeatTrack {
		t.Fatalf("volume/shuffle/repeat = %d/%v/%q", state.Volume, state.Shuffle, state.Repeat)
	}
	if state.Device.ID != "sdk-dev" || state.Device.Name != "Cadence Web" {
		t.Fatalf("device = %#v", state.Device)
	}
	if len(state.Devices) != 3 {
		t.Fatalf("devices = %#v, want the API list plus the player device", state.Devices)
	}
	var phone playback.Device
	for _, device := range state.Devices {
		if device.ID == "phone" {
			phone = device
		}
	}
	if !phone.Restricted || phone.VolumePercent != 80 || phone.Type != "smartphone" {
		t.Fatalf("phone = %#v", phone)
	}
	if !state.CanSeek || !state.CanControl {
		t.Fatalf("canSeek/canControl = %v/%v", state.CanSeek, state.CanControl)
	}
	if state.Revision != 1 || state.CapturedAt == 0 {
		t.Fatalf("revision/capturedAt = %d/%d", state.Revision, state.CapturedAt)
	}
}

func TestProviderStatePausedAndStopped(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		status playback.Status
	}{
		{
			name:   "paused",
			body:   `{"is_playing":false,"progress_ms":100,"item":{"id":"t","name":"Song","uri":"spotify:track:1","duration_ms":1000,"artists":[],"album":{"name":"","images":[]}}}`,
			status: playback.StatusPaused,
		},
		{
			name:   "stopped",
			body:   ``,
			status: playback.StatusStopped,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server, _ := newAPITestServer(t, func(writer http.ResponseWriter, request *http.Request, _ []byte) {
				if request.URL.Path == "/me/player" {
					if test.body == "" {
						writer.WriteHeader(http.StatusNoContent)
						return
					}
					writer.Header().Set("Content-Type", "application/json")
					_, _ = writer.Write([]byte(test.body))
					return
				}
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(`{"devices":[{"id":"active","is_active":true,"name":"Speaker","type":"Speaker","volume_percent":20}]}`))
			})
			backend := &fakeProviderBackend{loggedIn: true, installed: true}
			provider := newProviderForTest(t, backend, server.URL, nil)
			state, err := provider.State(context.Background())
			if err != nil {
				t.Fatalf("State() error = %v", err)
			}
			if state.Status != test.status {
				t.Fatalf("status = %q, want %q", state.Status, test.status)
			}
			if state.Device.ID != "active" || state.Volume != 20 {
				t.Fatalf("device/volume = %#v/%d", state.Device, state.Volume)
			}
			if test.status == playback.StatusStopped && state.CanSeek {
				t.Fatal("CanSeek = true for a stopped player")
			}
		})
	}
}

func TestProviderLoginRequiredState(t *testing.T) {
	server, recorder := newAPITestServer(t, func(writer http.ResponseWriter, _ *http.Request, _ []byte) {
		writer.WriteHeader(http.StatusNoContent)
	})
	backend := &fakeProviderBackend{loggedIn: false, installed: true}
	provider := newProviderForTest(t, backend, server.URL, nil)
	state, err := provider.State(context.Background())
	if err != nil {
		t.Fatalf("State() error = %v", err)
	}
	if state.Error == nil || state.Error.Code != CodeLoginRequired {
		t.Fatalf("error = %#v, want %q", state.Error, CodeLoginRequired)
	}
	if len(recorder.snapshot()) != 0 {
		t.Fatal("the API was queried while logged out")
	}
}

func TestProviderSyntheticDevice(t *testing.T) {
	server, _ := newAPITestServer(t, func(writer http.ResponseWriter, request *http.Request, _ []byte) {
		if request.URL.Path == "/me/player" {
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		_, _ = writer.Write([]byte(`{"devices":[]}`))
	})
	backend := &fakeProviderBackend{loggedIn: true, installed: true}
	provider := newProviderForTest(t, backend, server.URL, nil)
	state, err := provider.State(context.Background())
	if err != nil {
		t.Fatalf("State() error = %v", err)
	}
	if state.Device.ID != "" || state.Device.Name != defaultWebDeviceName || state.Device.Type != webDeviceType {
		t.Fatalf("device = %#v", state.Device)
	}
	if len(state.Devices) != 1 {
		t.Fatalf("devices = %#v", state.Devices)
	}
	if state.Status != playback.StatusStopped || state.CanSeek {
		t.Fatalf("state = %#v", state)
	}
}

func TestProviderRateLimitErrorState(t *testing.T) {
	server, _ := newAPITestServer(t, func(writer http.ResponseWriter, _ *http.Request, _ []byte) {
		writer.Header().Set("Retry-After", "7")
		writer.WriteHeader(http.StatusTooManyRequests)
		_, _ = writer.Write([]byte(`{"error":{"status":429,"message":"rate limited"}}`))
	})
	backend := &fakeProviderBackend{loggedIn: true, installed: true}
	provider := newProviderForTest(t, backend, server.URL, (&sleepRecorder{}).sleep)
	state, err := provider.State(context.Background())
	if err != nil {
		t.Fatalf("State() error = %v", err)
	}
	if state.Error == nil || state.Error.Code != CodeRateLimited {
		t.Fatalf("error = %#v", state.Error)
	}
	if !state.Error.Retryable || state.Error.RetryAfterSeconds != 7 {
		t.Fatalf("retryable/retryAfter = %v/%d", state.Error.Retryable, state.Error.RetryAfterSeconds)
	}
}

func TestProviderQueue(t *testing.T) {
	var body strings.Builder
	body.WriteString(`{"currently_playing":{"id":"current","name":"Current","uri":"spotify:track:current","duration_ms":1000,"artists":[{"name":"A"}],"album":{"name":"","images":[]}},"queue":[`)
	for index := 0; index < 60; index++ {
		if index > 0 {
			body.WriteString(",")
		}
		fmt.Fprintf(&body, `{"id":"track-%d","name":"Track %d","uri":"spotify:track:%d","duration_ms":1000,"artists":[],"album":{"name":"","images":[]}}`, index, index, index)
	}
	body.WriteString(`]}`)
	server, _ := newAPITestServer(t, func(writer http.ResponseWriter, _ *http.Request, _ []byte) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(body.String()))
	})
	backend := &fakeProviderBackend{loggedIn: true, installed: true}
	provider := newProviderForTest(t, backend, server.URL, nil)
	items, err := provider.Queue(context.Background())
	if err != nil {
		t.Fatalf("Queue() error = %v", err)
	}
	if len(items) != 50 {
		t.Fatalf("queue length = %d, want 50", len(items))
	}
	if !items[0].IsCurrent || items[0].Track.ID != "current" {
		t.Fatalf("first item = %#v", items[0])
	}
	if !items[1].IsNext || items[1].Track.ID != "track-0" {
		t.Fatalf("second item = %#v", items[1])
	}
	for index, item := range items {
		if item.Index != index {
			t.Fatalf("item %d index = %d", index, item.Index)
		}
	}
	if items[2].IsCurrent || items[2].IsNext {
		t.Fatalf("third item has queue flags: %#v", items[2])
	}
}

func TestProviderPlayStartsAndRetries(t *testing.T) {
	calls := 0
	server, recorder := newAPITestServer(t, func(writer http.ResponseWriter, request *http.Request, _ []byte) {
		if request.URL.Path != "/me/player/play" {
			http.NotFound(writer, request)
			return
		}
		calls++
		if calls == 1 {
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte(`{"error":{"status":404,"message":"Device not found","reason":"NO_ACTIVE_DEVICE"}}`))
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	})
	backend := &fakeProviderBackend{loggedIn: true, installed: true}
	provider := newProviderForTest(t, backend, server.URL, nil)
	previous := webDeviceRetryInterval
	webDeviceRetryInterval = time.Millisecond
	defer func() { webDeviceRetryInterval = previous }()

	if err := provider.Play(context.Background()); err != nil {
		t.Fatalf("Play() error = %v", err)
	}
	if backend.ensureCount() != 1 {
		t.Fatalf("EnsureStarted calls = %d, want 1", backend.ensureCount())
	}
	requests := recorder.snapshot()
	if len(requests) != 2 {
		t.Fatalf("play requests = %d, want 2", len(requests))
	}
	if requests[0].query.Get("device_id") != "sdk-dev" {
		t.Fatalf("device_id = %q", requests[0].query.Get("device_id"))
	}
}

func TestProviderPlayRequiresRuntime(t *testing.T) {
	server, _ := newAPITestServer(t, func(writer http.ResponseWriter, _ *http.Request, _ []byte) {
		writer.WriteHeader(http.StatusNoContent)
	})
	backend := &fakeProviderBackend{loggedIn: true, installed: false}
	provider := newProviderForTest(t, backend, server.URL, nil)
	if err := provider.Play(context.Background()); !errors.Is(err, ErrRuntimeNotInstalled) {
		t.Fatalf("Play() error = %v, want ErrRuntimeNotInstalled", err)
	}
	if backend.ensureCount() != 0 {
		t.Fatalf("EnsureStarted calls = %d, want 0", backend.ensureCount())
	}
}

func TestProviderPlayContextBodies(t *testing.T) {
	server, recorder := newAPITestServer(t, func(writer http.ResponseWriter, request *http.Request, _ []byte) {
		if request.URL.Path != "/me/player/play" {
			http.NotFound(writer, request)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	})
	backend := &fakeProviderBackend{loggedIn: true, installed: true}
	provider := newProviderForTest(t, backend, server.URL, nil)

	if err := provider.PlayContext(context.Background(), "", ""); err == nil || CodeOf(err) != CodeInvalidArgument {
		t.Fatalf("PlayContext(empty) error = %v", err)
	}
	if err := provider.PlayContext(context.Background(), "spotify:track:1", ""); err != nil {
		t.Fatalf("PlayContext(track) error = %v", err)
	}
	if err := provider.PlayContext(context.Background(), "spotify:album:1", "spotify:track:2"); err != nil {
		t.Fatalf("PlayContext(album) error = %v", err)
	}
	requests := recorder.snapshot()
	if len(requests) != 2 {
		t.Fatalf("play requests = %d, want 2", len(requests))
	}

	var trackBody map[string]any
	if err := json.Unmarshal(requests[0].body, &trackBody); err != nil {
		t.Fatalf("decode track body: %v", err)
	}
	if !jsonEqual(trackBody, map[string]any{"uris": []any{"spotify:track:1"}}) {
		t.Fatalf("track body = %#v", trackBody)
	}

	var albumBody map[string]any
	if err := json.Unmarshal(requests[1].body, &albumBody); err != nil {
		t.Fatalf("decode album body: %v", err)
	}
	wantAlbum := map[string]any{
		"context_uri": "spotify:album:1",
		"offset":      map[string]any{"uri": "spotify:track:2"},
	}
	if !jsonEqual(albumBody, wantAlbum) {
		t.Fatalf("album body = %#v", albumBody)
	}
}

func TestProviderAddToQueueDoesNotStart(t *testing.T) {
	server, recorder := newAPITestServer(t, func(writer http.ResponseWriter, _ *http.Request, _ []byte) {
		writer.WriteHeader(http.StatusNoContent)
	})
	backend := &fakeProviderBackend{loggedIn: true, installed: false, running: false}
	provider := newProviderForTest(t, backend, server.URL, nil)
	if err := provider.AddToQueue(context.Background(), "spotify:track:9"); err != nil {
		t.Fatalf("AddToQueue() error = %v", err)
	}
	if backend.ensureCount() != 0 {
		t.Fatalf("EnsureStarted calls = %d, want 0", backend.ensureCount())
	}
	requests := recorder.snapshot()
	if len(requests) != 1 {
		t.Fatalf("requests = %d", len(requests))
	}
	if requests[0].query.Get("device_id") != "" || requests[0].query.Get("uri") != "spotify:track:9" {
		t.Fatalf("query = %v", requests[0].query)
	}
}

func TestProviderToggleAndTransfer(t *testing.T) {
	t.Run("pause", func(t *testing.T) {
		server, recorder := newAPITestServer(t, func(writer http.ResponseWriter, request *http.Request, _ []byte) {
			switch request.URL.Path {
			case "/me/player":
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(`{"is_playing":true}`))
			case "/me/player/pause":
				writer.WriteHeader(http.StatusNoContent)
			default:
				http.NotFound(writer, request)
			}
		})
		backend := &fakeProviderBackend{deviceID: "sdk-dev", running: true, loggedIn: true, installed: true}
		provider := newProviderForTest(t, backend, server.URL, nil)
		if err := provider.Toggle(context.Background()); err != nil {
			t.Fatalf("Toggle() error = %v", err)
		}
		requests := recorder.snapshot()
		if len(requests) != 2 || requests[1].path != "/me/player/pause" {
			t.Fatalf("requests = %#v", requests)
		}
		if requests[1].query.Get("device_id") != "sdk-dev" {
			t.Fatalf("pause query = %v", requests[1].query)
		}
	})

	t.Run("resume", func(t *testing.T) {
		server, recorder := newAPITestServer(t, func(writer http.ResponseWriter, request *http.Request, _ []byte) {
			switch request.URL.Path {
			case "/me/player":
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(`{"is_playing":false}`))
			case "/me/player/play":
				writer.WriteHeader(http.StatusNoContent)
			default:
				http.NotFound(writer, request)
			}
		})
		backend := &fakeProviderBackend{loggedIn: true, installed: true}
		provider := newProviderForTest(t, backend, server.URL, nil)
		if err := provider.Toggle(context.Background()); err != nil {
			t.Fatalf("Toggle() error = %v", err)
		}
		requests := recorder.snapshot()
		if len(requests) != 2 || requests[1].path != "/me/player/play" {
			t.Fatalf("requests = %#v", requests)
		}
		if backend.ensureCount() != 1 {
			t.Fatalf("EnsureStarted calls = %d, want 1", backend.ensureCount())
		}
	})

	t.Run("transfer sdk", func(t *testing.T) {
		server, recorder := newAPITestServer(t, func(writer http.ResponseWriter, _ *http.Request, _ []byte) {
			writer.WriteHeader(http.StatusNoContent)
		})
		backend := &fakeProviderBackend{loggedIn: true, installed: true}
		provider := newProviderForTest(t, backend, server.URL, nil)
		if err := provider.Transfer(context.Background(), ""); err != nil {
			t.Fatalf("Transfer() error = %v", err)
		}
		if backend.ensureCount() != 1 {
			t.Fatalf("EnsureStarted calls = %d, want 1", backend.ensureCount())
		}
		requests := recorder.snapshot()
		if len(requests) != 1 || requests[0].path != "/me/player" {
			t.Fatalf("requests = %#v", requests)
		}
		var body map[string]any
		if err := json.Unmarshal(requests[0].body, &body); err != nil {
			t.Fatalf("decode transfer body: %v", err)
		}
		if !jsonEqual(body, map[string]any{"device_ids": []any{"sdk-dev"}, "play": false}) {
			t.Fatalf("transfer body = %#v", body)
		}
	})

	t.Run("transfer explicit", func(t *testing.T) {
		server, _ := newAPITestServer(t, func(writer http.ResponseWriter, _ *http.Request, _ []byte) {
			writer.WriteHeader(http.StatusNoContent)
		})
		backend := &fakeProviderBackend{loggedIn: true, installed: true}
		provider := newProviderForTest(t, backend, server.URL, nil)
		if err := provider.Transfer(context.Background(), "other"); err != nil {
			t.Fatalf("Transfer() error = %v", err)
		}
		if backend.ensureCount() != 0 {
			t.Fatalf("EnsureStarted calls = %d, want 0", backend.ensureCount())
		}
	})
}

func TestProviderRevisionOnlyChangesOnStateChange(t *testing.T) {
	var volume = 20
	var mu sync.Mutex
	server, _ := newAPITestServer(t, func(writer http.ResponseWriter, request *http.Request, _ []byte) {
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/me/player/devices" {
			_, _ = writer.Write([]byte(`{"devices":[]}`))
			return
		}
		mu.Lock()
		current := volume
		mu.Unlock()
		_, _ = fmt.Fprintf(writer, `{"is_playing":false,"shuffle_state":false,"repeat_state":"off","item":{"id":"t","name":"Song","uri":"spotify:track:1","duration_ms":1000,"artists":[],"album":{"name":"","images":[]}},"device":{"id":"dev","is_active":true,"volume_percent":%d}}`, current)
	})
	backend := &fakeProviderBackend{loggedIn: true, installed: true}
	provider := newProviderForTest(t, backend, server.URL, nil)

	first, err := provider.State(context.Background())
	if err != nil {
		t.Fatalf("first State() error = %v", err)
	}
	second, err := provider.State(context.Background())
	if err != nil {
		t.Fatalf("second State() error = %v", err)
	}
	if second.Revision != first.Revision || second.CapturedAt != first.CapturedAt {
		t.Fatalf("unchanged state revision/capturedAt = %d/%d, want %d/%d", second.Revision, second.CapturedAt, first.Revision, first.CapturedAt)
	}

	mu.Lock()
	volume = 70
	mu.Unlock()
	third, err := provider.State(context.Background())
	if err != nil {
		t.Fatalf("third State() error = %v", err)
	}
	if third.Revision != first.Revision+1 {
		t.Fatalf("revision = %d, want %d", third.Revision, first.Revision+1)
	}
	if third.Volume != 70 {
		t.Fatalf("volume = %d, want 70", third.Volume)
	}
}

func TestProviderStreamEmitsOnChangeAndSurvivesLogin(t *testing.T) {
	previousPlayer, previousDevices := webStreamPlayerInterval, webStreamDevicesInterval
	previousLogin, previousError := webStreamLoginInterval, webStreamErrorInterval
	webStreamPlayerInterval = 5 * time.Millisecond
	webStreamDevicesInterval = 10 * time.Millisecond
	webStreamLoginInterval = 5 * time.Millisecond
	webStreamErrorInterval = 10 * time.Millisecond
	defer func() {
		webStreamPlayerInterval, webStreamDevicesInterval = previousPlayer, previousDevices
		webStreamLoginInterval, webStreamErrorInterval = previousLogin, previousError
	}()

	var mu sync.Mutex
	volume := 20
	server, _ := newAPITestServer(t, func(writer http.ResponseWriter, request *http.Request, _ []byte) {
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/me/player/devices" {
			_, _ = writer.Write([]byte(`{"devices":[]}`))
			return
		}
		mu.Lock()
		current := volume
		mu.Unlock()
		_, _ = fmt.Fprintf(writer, `{"is_playing":false,"item":{"id":"t","name":"Song","uri":"spotify:track:1","duration_ms":1000,"artists":[],"album":{"name":"","images":[]}},"device":{"id":"dev","is_active":true,"volume_percent":%d}}`, current)
	})
	backend := &fakeProviderBackend{loggedIn: true, installed: true}
	provider := newProviderForTest(t, backend, server.URL, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	states := make(chan playback.PlayerState, 32)
	streamErr := make(chan error, 1)
	go func() {
		streamErr <- provider.Stream(ctx, func(state playback.PlayerState) {
			states <- state
		})
	}()

	initial := waitForProviderState(t, states)
	if initial.Error != nil || initial.Volume != 20 {
		t.Fatalf("initial state = %#v", initial)
	}

	backend.setLoggedIn(false)
	login := waitForProviderState(t, states)
	if login.Error == nil || login.Error.Code != CodeLoginRequired {
		t.Fatalf("login state = %#v", login)
	}

	backend.setLoggedIn(true)
	mu.Lock()
	volume = 40
	mu.Unlock()
	changed := waitForProviderState(t, states)
	if changed.Error != nil || changed.Volume != 40 {
		t.Fatalf("changed state = %#v", changed)
	}
	if changed.Revision <= initial.Revision {
		t.Fatalf("revision did not increase: %d -> %d", initial.Revision, changed.Revision)
	}
	if backend.ensureCount() != 0 {
		t.Fatalf("the stream started the helper: %d calls", backend.ensureCount())
	}

	cancel()
	select {
	case err := <-streamErr:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Stream() error = %v, want context.Canceled", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Stream() did not return after cancellation")
	}
}

func jsonEqual(left, right any) bool {
	leftJSON, err := json.Marshal(left)
	if err != nil {
		return false
	}
	rightJSON, err := json.Marshal(right)
	if err != nil {
		return false
	}
	return string(leftJSON) == string(rightJSON)
}
