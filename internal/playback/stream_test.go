package playback

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"cadence/internal/artwork"
	"cadence/internal/engine"

	"github.com/coder/websocket"
)

func waitForState(t *testing.T, states <-chan PlayerState) PlayerState {
	t.Helper()
	select {
	case state := <-states:
		return state
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for a streamed state")
		return PlayerState{}
	}
}

func waitForCondition(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("timed out waiting for a condition")
}

func TestEngineProviderStreamPublishesOnlyChanges(t *testing.T) {
	previousRefresh, previousReconcile := streamRefreshInterval, streamReconcileInterval
	streamRefreshInterval = 5 * time.Millisecond
	streamReconcileInterval = time.Hour
	defer func() {
		streamRefreshInterval = previousRefresh
		streamReconcileInterval = previousReconcile
	}()

	var volume atomic.Int64
	volume.Store(20)
	var statusCalls atomic.Int64
	connected := make(chan struct{}, 1)
	sendEvent := make(chan struct{}, 1)
	release := make(chan struct{})
	defer close(release)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/status":
			statusCalls.Add(1)
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(fmt.Sprintf(`{
				"username":"user","device_id":"dev","device_name":"Cadence","device_type":"COMPUTER",
				"stopped":false,"paused":true,"volume":%d,"volume_steps":100,
				"track":{"uri":"spotify:track:1","name":"Song","duration":1000,"position":500,"artist_names":["Artist"],"album_name":"Album"}
			}`, volume.Load())))
		case "/events":
			conn, err := websocket.Accept(writer, request, nil)
			if err != nil {
				return
			}
			defer func() {
				_ = conn.Close(websocket.StatusNormalClosure, "")
			}()
			connected <- struct{}{}
			for {
				select {
				case <-sendEvent:
					_ = conn.Write(context.Background(), websocket.MessageText, []byte(`{"type":"volume","data":{"value":30}}`))
				case <-release:
					return
				}
			}
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	runtime := engine.NewSupervisor(engine.Options{Client: engine.NewClient(server.URL, server.Client())})
	provider := NewEngineProvider(runtime, artwork.New(server.Client()))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	states := make(chan PlayerState, 8)
	streamErr := make(chan error, 1)
	go func() {
		streamErr <- provider.Stream(ctx, func(state PlayerState) {
			states <- state
		})
	}()

	initial := waitForState(t, states)
	if initial.Volume != 20 || initial.Status != StatusPaused {
		t.Fatalf("initial state = %#v", initial)
	}
	if initial.Revision != 1 || initial.CapturedAt == 0 {
		t.Fatalf("initial revision/capturedAt = %d/%d", initial.Revision, initial.CapturedAt)
	}
	select {
	case <-connected:
	case <-time.After(3 * time.Second):
		t.Fatal("event stream was not connected")
	}

	// An idle stream must not poll: no event and no reconciliation due means
	// no /status request at all.
	idleCalls := statusCalls.Load()
	time.Sleep(300 * time.Millisecond)
	if got := statusCalls.Load(); got != idleCalls {
		t.Fatalf("idle stream made %d extra /status requests", got-idleCalls)
	}

	volume.Store(40)
	sendEvent <- struct{}{}
	changed := waitForState(t, states)
	if changed.Volume != 40 || changed.Revision != 2 {
		t.Fatalf("changed state = %#v", changed)
	}
	if changed.CapturedAt < initial.CapturedAt {
		t.Fatalf("capturedAt went backwards: %d -> %d", initial.CapturedAt, changed.CapturedAt)
	}

	// A refresh triggered by an event with identical status must not emit.
	callsBefore := statusCalls.Load()
	sendEvent <- struct{}{}
	waitForCondition(t, func() bool { return statusCalls.Load() > callsBefore })
	select {
	case duplicate := <-states:
		t.Fatalf("unexpected emit for an unchanged refresh: %#v", duplicate)
	case <-time.After(250 * time.Millisecond):
	}

	cancel()
	select {
	case err := <-streamErr:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Stream() error = %v, want context.Canceled", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Stream did not return after cancellation")
	}
}

func TestEngineProviderStreamEmitsUnavailableWithoutClient(t *testing.T) {
	provider := NewEngineProvider(engine.NewSupervisor(engine.Options{}), nil)
	var states []PlayerState
	err := provider.Stream(context.Background(), func(state PlayerState) {
		states = append(states, state)
	})
	if err == nil {
		t.Fatal("Stream() error = nil, want an unavailable engine error")
	}
	if len(states) != 1 {
		t.Fatalf("emitted states = %d, want exactly 1", len(states))
	}
	state := states[0]
	if state.Status == StatusPlaying {
		t.Fatalf("status = %q, want a stopped state", state.Status)
	}
	if state.Error == nil || state.Error.Code != "audio_unavailable" {
		t.Fatalf("error = %#v, want audio_unavailable", state.Error)
	}
	if state.Error.Message != "The local audio engine is not running." {
		t.Fatalf("error message = %q", state.Error.Message)
	}
}

func TestEngineProviderIgnoresOutOfOrderFetches(t *testing.T) {
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	var calls atomic.Int64

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/status" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		if calls.Add(1) == 1 {
			close(firstStarted)
			<-releaseFirst
			_, _ = writer.Write([]byte(`{
				"username":"user","device_id":"dev","device_name":"Cadence","device_type":"COMPUTER",
				"stopped":false,"paused":false,"volume":20,"volume_steps":100,
				"track":{"uri":"spotify:track:1","name":"Old","duration":1000,"position":100,"artist_names":["Artist"],"album_name":"Album"}
			}`))
			return
		}
		_, _ = writer.Write([]byte(`{
			"username":"user","device_id":"dev","device_name":"Cadence","device_type":"COMPUTER",
			"stopped":false,"paused":true,"volume":40,"volume_steps":100,
			"track":{"uri":"spotify:track:1","name":"New","duration":1000,"position":100,"artist_names":["Artist"],"album_name":"Album"}
		}`))
	}))
	defer server.Close()

	runtime := engine.NewSupervisor(engine.Options{Client: engine.NewClient(server.URL, server.Client())})
	provider := NewEngineProvider(runtime, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	states := make(chan PlayerState, 4)
	errs := make(chan error, 2)
	var stale PlayerState
	go func() {
		state, err := provider.refresh(ctx, func(state PlayerState) {
			states <- state
		})
		stale = state
		errs <- err
	}()

	select {
	case <-firstStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("the first status request did not start")
	}

	newer, err := provider.refresh(ctx, func(state PlayerState) {
		states <- state
	})
	if err != nil {
		t.Fatal(err)
	}
	if newer.Volume != 40 || newer.Revision != 1 {
		t.Fatalf("newer state = %#v", newer)
	}
	emitted := waitForState(t, states)
	if emitted.Volume != 40 || emitted.Revision != 1 {
		t.Fatalf("emitted state = %#v", emitted)
	}

	close(releaseFirst)
	if err := <-errs; err != nil {
		t.Fatal(err)
	}
	if stale.Volume != 40 || stale.Revision != 1 {
		t.Fatalf("stale fetch returned %#v, want the newer snapshot", stale)
	}
	select {
	case extra := <-states:
		t.Fatalf("stale fetch emitted %#v", extra)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestDemoProviderQueue(t *testing.T) {
	provider := NewDemoProvider()
	queue, err := provider.Queue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(queue) == 0 || !queue[0].IsCurrent || !queue[1].IsNext {
		t.Fatalf("queue = %#v", queue)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := provider.Queue(cancelled); err == nil {
		t.Fatal("Queue() with a cancelled context returned no error")
	}
}
