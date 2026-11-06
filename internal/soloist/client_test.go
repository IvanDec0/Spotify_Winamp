package soloist

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func testServer(t *testing.T, handler func(conn *websocket.Conn)) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		handler(conn)
	}))
	t.Cleanup(server.Close)
	return "ws" + strings.TrimPrefix(server.URL, "http")
}

func readCommand(t *testing.T, conn *websocket.Conn) Command {
	t.Helper()
	_, data, err := conn.Read(context.Background())
	if err != nil {
		t.Fatalf("server read: %v", err)
	}
	var command Command
	if err := json.Unmarshal(data, &command); err != nil {
		t.Fatalf("server decode: %v", err)
	}
	return command
}

func writeEvent(t *testing.T, conn *websocket.Conn, payload string) {
	t.Helper()
	if err := conn.Write(context.Background(), websocket.MessageText, []byte(payload)); err != nil {
		t.Fatalf("server write: %v", err)
	}
}

func drain(conn *websocket.Conn) {
	for {
		if _, _, err := conn.Read(context.Background()); err != nil {
			return
		}
	}
}

func TestDoQueryReturnsTypedEvent(t *testing.T) {
	url := testServer(t, func(conn *websocket.Conn) {
		command := readCommand(t, conn)
		if command.Command != "get_state" {
			t.Errorf("command = %q, want get_state", command.Command)
		}
		writeEvent(t, conn, `{"type":"playback_state","status":"paused","volume":42,"options":{"shuffle":true,"repeat":"context"},"item":{"uri":"spotify:track:x","entity_type":"track","decorations":{"identity":{"name":"Song"},"playback":{"duration_ms":1000}}}}`)
		drain(conn)
	})

	client, err := Dial(context.Background(), url)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	event, err := client.Do(context.Background(), GetState(), "playback_state", time.Second)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	if event.Playback == nil || event.Playback.Status != "paused" || event.Playback.Volume != 42 {
		t.Fatalf("playback = %+v", event.Playback)
	}
	if !event.Playback.Options.Shuffle || event.Playback.Options.Repeat != "context" {
		t.Fatalf("options = %+v", event.Playback.Options)
	}
	if event.Playback.Item == nil || event.Playback.Item.Decorations.Identity.Name != "Song" {
		t.Fatalf("item = %+v", event.Playback.Item)
	}
}

func TestDoDoesNotLoseImmediateReply(t *testing.T) {
	url := testServer(t, func(conn *websocket.Conn) {
		for {
			_, data, err := conn.Read(context.Background())
			if err != nil {
				return
			}
			var command Command
			if err := json.Unmarshal(data, &command); err != nil {
				return
			}
			payload, _ := json.Marshal(map[string]string{"type": "command_result", "command": command.Command})
			if err := conn.Write(context.Background(), websocket.MessageText, payload); err != nil {
				return
			}
		}
	})

	client, err := Dial(context.Background(), url)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	for iteration := 0; iteration < 25; iteration++ {
		event, err := client.Do(context.Background(), Pause(), "command_result", time.Second)
		if err != nil {
			t.Fatalf("iteration %d: %v", iteration, err)
		}
		if event.Command != "pause" {
			t.Fatalf("iteration %d: command = %q", iteration, event.Command)
		}
	}
}

func TestErrorEventFailsWaiterAndKeepsConnection(t *testing.T) {
	url := testServer(t, func(conn *websocket.Conn) {
		readCommand(t, conn)
		writeEvent(t, conn, `{"type":"error","message":"not logged in"}`)
		readCommand(t, conn)
		writeEvent(t, conn, `{"type":"command_result","command":"pause"}`)
		drain(conn)
	})

	client, err := Dial(context.Background(), url)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	_, err = client.Do(context.Background(), Pause(), "command_result", time.Second)
	if err == nil {
		t.Fatal("expected the error event to fail the command")
	}
	if CodeOf(err) != CodeProtocol {
		t.Fatalf("code = %q, want %q", CodeOf(err), CodeProtocol)
	}
	if !strings.Contains(err.Error(), "not logged in") {
		t.Fatalf("error = %v", err)
	}

	event, err := client.Do(context.Background(), Pause(), "command_result", time.Second)
	if err != nil {
		t.Fatalf("connection did not stay open: %v", err)
	}
	if event.Command != "pause" {
		t.Fatalf("command = %q", event.Command)
	}
}

func TestSubscribeReceivesEventsAndUnsubscribes(t *testing.T) {
	url := testServer(t, func(conn *websocket.Conn) {
		readCommand(t, conn)
		writeEvent(t, conn, `{"type":"auth_state","logged_in":true,"is_active":true,"device_name":"Cadence"}`)
		writeEvent(t, conn, `{"type":"playback_state","status":"playing","volume":10}`)
		writeEvent(t, conn, `{"type":"position_sync","position":{"position_ms":500,"timestamp_ms":1000,"speed":1}}`)
		readCommand(t, conn)
		writeEvent(t, conn, `{"type":"auth_state","logged_in":false}`)
		drain(conn)
	})

	client, err := Dial(context.Background(), url)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	events, unsubscribe := client.Subscribe()
	if err := client.Send(context.Background(), Pause()); err != nil {
		t.Fatalf("send: %v", err)
	}

	want := []string{"auth_state", "playback_state", "position_sync"}
	for _, wantType := range want {
		select {
		case event := <-events:
			if event.Type != wantType {
				t.Fatalf("event type = %q, want %q", event.Type, wantType)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for %q", wantType)
		}
	}

	unsubscribe()
	unsubscribe()
	if err := client.Send(context.Background(), Pause()); err != nil {
		t.Fatalf("send after unsubscribe: %v", err)
	}
	select {
	case event := <-events:
		t.Fatalf("received %q after unsubscribe", event.Type)
	case <-time.After(150 * time.Millisecond):
	}
}

func TestAwaitReplaysInitialAuthState(t *testing.T) {
	url := testServer(t, func(conn *websocket.Conn) {
		writeEvent(t, conn, `{"type":"auth_state","logged_in":false,"is_active":false,"device_name":"Cadence"}`)
		drain(conn)
	})

	client, err := Dial(context.Background(), url)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	time.Sleep(50 * time.Millisecond)
	event, err := client.Await(context.Background(), "auth_state", time.Second)
	if err != nil {
		t.Fatalf("await: %v", err)
	}
	if event.Auth == nil || event.Auth.LoggedIn || event.Auth.DeviceName != "Cadence" {
		t.Fatalf("auth = %+v", event.Auth)
	}
}

func TestDoneClosesOnServerClose(t *testing.T) {
	url := testServer(t, func(conn *websocket.Conn) {
		_ = conn.Close(websocket.StatusNormalClosure, "")
	})

	client, err := Dial(context.Background(), url)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	select {
	case <-client.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("Done did not close")
	}
	if err := client.Err(); err != nil {
		t.Fatalf("Err = %v, want nil for a clean close", err)
	}
}
