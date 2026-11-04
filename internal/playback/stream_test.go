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

func TestEngineProviderStreamRequiresClient(t *testing.T) {
	provider := NewEngineProvider(engine.NewSupervisor(engine.Options{}), nil)
	err := provider.Stream(context.Background(), func(PlayerState) {})
	if err == nil {
		t.Fatal("Stream() error = nil, want an unavailable engine error")
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
