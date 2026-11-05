package engine

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func nextEvent(t *testing.T, events <-chan Event) Event {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for an engine event")
		return Event{}
	}
}

func TestWatchEventsDeliversOrderedEvents(t *testing.T) {
	received := make(chan Event, 4)
	release := make(chan struct{})
	defer close(release)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/events" {
			http.NotFound(writer, request)
			return
		}
		conn, err := websocket.Accept(writer, request, nil)
		if err != nil {
			return
		}
		defer func() {
			_ = conn.Close(websocket.StatusNormalClosure, "")
		}()
		ctx := context.Background()
		_ = conn.Write(ctx, websocket.MessageText, []byte(`{"type":"playing","data":{"uri":"spotify:track:1"}}`))
		_ = conn.Write(ctx, websocket.MessageText, []byte(`not a json envelope`))
		_ = conn.Write(ctx, websocket.MessageBinary, []byte(`{"type":"volume","data":{"value":30}}`))
		<-release
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- client.WatchEvents(ctx, func(event Event) {
			received <- event
		})
	}()

	first := nextEvent(t, received)
	second := nextEvent(t, received)
	if first.Type != "playing" || second.Type != "volume" {
		t.Fatalf("event order = %q, %q; want playing, volume", first.Type, second.Type)
	}
	var data struct {
		Value int `json:"value"`
	}
	if err := json.Unmarshal(second.Data, &data); err != nil || data.Value != 30 {
		t.Fatalf("volume payload = %s (err %v)", second.Data, err)
	}

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("WatchEvents() error = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("WatchEvents did not return after cancellation")
	}
}

func TestWatchEventsRejectsUnreachableEngine(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.NotFound(writer, request)
	}))
	server.Close()

	client := NewClient(server.URL, server.Client())
	err := client.WatchEvents(context.Background(), func(Event) {})
	if err == nil {
		t.Fatal("WatchEvents() error = nil, want a dial error")
	}
}

func TestEventURL(t *testing.T) {
	tests := []struct {
		base string
		want string
	}{
		{base: "http://127.0.0.1:1234", want: "ws://127.0.0.1:1234/events"},
		{base: "http://127.0.0.1:1234/", want: "ws://127.0.0.1:1234/events"},
		{base: "https://127.0.0.1:1234", want: "wss://127.0.0.1:1234/events"},
	}
	for _, test := range tests {
		got, err := eventURL(test.base)
		if err != nil {
			t.Fatalf("eventURL(%q) error = %v", test.base, err)
		}
		if got != test.want {
			t.Fatalf("eventURL(%q) = %q, want %q", test.base, got, test.want)
		}
	}
}
