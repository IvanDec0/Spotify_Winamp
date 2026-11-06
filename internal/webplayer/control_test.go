package webplayer

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func dialControl(t *testing.T, rawURL string) (*websocket.Conn, *http.Response) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, response, err := websocket.Dial(ctx, rawURL, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", rawURL, err)
	}
	// Tests do not run a reader on the client side, so a graceful close would
	// wait for a close handshake that nobody reads. The server's graceful
	// Path is covered by TestControlCloseIsIdempotent.
	t.Cleanup(func() { _ = conn.CloseNow() })
	return conn, response
}

func controlURLWithNonce(t *testing.T, rawURL, nonce string) string {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse control URL: %v", err)
	}
	query := parsed.Query()
	query.Set("nonce", nonce)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func httpControlURL(t *testing.T, rawURL string) string {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse control URL: %v", err)
	}
	parsed.Scheme = "http"
	return parsed.String()
}

func waitForEvent(t *testing.T, control *Control) Message {
	t.Helper()
	select {
	case message, ok := <-control.Events():
		if !ok {
			t.Fatal("events channel closed")
		}
		return message
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for a control event")
	}
	return Message{}
}

func TestControlRejectsWrongNonce(t *testing.T) {
	control, err := NewControl()
	if err != nil {
		t.Fatalf("new control: %v", err)
	}
	t.Cleanup(func() { _ = control.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, response, err := websocket.Dial(ctx, controlURLWithNonce(t, control.URL(), "wrong"), nil)
	if err == nil {
		_ = conn.Close(websocket.StatusNormalClosure, "")
		t.Fatal("a wrong nonce was accepted")
	}
	if response == nil {
		t.Fatalf("dial error has no HTTP response: %v", err)
	}
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.StatusCode)
	}
}

func TestControlRequiresHello(t *testing.T) {
	control, err := NewControl()
	if err != nil {
		t.Fatalf("new control: %v", err)
	}
	t.Cleanup(func() { _ = control.Close() })

	conn, _ := dialControl(t, control.URL())
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := wsjson.Write(ctx, conn, Message{Type: "ready", DeviceID: "device"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	message := waitForEvent(t, control)
	if message.Type != "error" || message.Code != "protocol_error" {
		t.Fatalf("event = %+v, want a protocol error", message)
	}
}

func TestControlEvents(t *testing.T) {
	control, err := NewControl()
	if err != nil {
		t.Fatalf("new control: %v", err)
	}
	t.Cleanup(func() { _ = control.Close() })

	conn, _ := dialControl(t, control.URL())
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := wsjson.Write(ctx, conn, Message{Type: "hello", Version: "1"}); err != nil {
		t.Fatalf("hello: %v", err)
	}
	if err := wsjson.Write(ctx, conn, Message{Type: "ready", DeviceID: "device-1"}); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := wsjson.Write(ctx, conn, Message{Type: "error", Code: "widevine", Message: "missing"}); err != nil {
		t.Fatalf("error: %v", err)
	}

	ready := waitForEvent(t, control)
	if ready.Type != "ready" || ready.DeviceID != "device-1" {
		t.Fatalf("first event = %+v", ready)
	}
	failure := waitForEvent(t, control)
	if failure.Type != "error" || failure.Code != "widevine" || failure.Message != "missing" {
		t.Fatalf("second event = %+v", failure)
	}
}

func TestControlSendRoundTrip(t *testing.T) {
	control, err := NewControl()
	if err != nil {
		t.Fatalf("new control: %v", err)
	}
	t.Cleanup(func() { _ = control.Close() })

	if err := control.Send(context.Background(), Message{Type: "show"}); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("Send before connect = %v, want ErrNotConnected", err)
	}

	conn, _ := dialControl(t, control.URL())
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := wsjson.Write(ctx, conn, Message{Type: "hello", Version: "1"}); err != nil {
		t.Fatalf("hello: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		err := control.Send(ctx, Message{Type: "show"})
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Send never succeeded: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	var received Message
	if err := wsjson.Read(ctx, conn, &received); err != nil {
		t.Fatalf("read: %v", err)
	}
	if received.Type != "show" {
		t.Fatalf("received = %+v, want show", received)
	}
}

func TestControlRejectsSecondConnection(t *testing.T) {
	control, err := NewControl()
	if err != nil {
		t.Fatalf("new control: %v", err)
	}
	t.Cleanup(func() { _ = control.Close() })

	first, _ := dialControl(t, control.URL())
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := wsjson.Write(ctx, first, Message{Type: "hello", Version: "1"}); err != nil {
		t.Fatalf("hello: %v", err)
	}
	// Wait until the server has registered the first connection.
	deadline := time.Now().Add(2 * time.Second)
	for {
		err := control.Send(ctx, Message{Type: "show"})
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the first connection never registered: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	secondCtx, secondCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer secondCancel()
	conn, response, err := websocket.Dial(secondCtx, control.URL(), nil)
	if err == nil {
		_ = conn.Close(websocket.StatusNormalClosure, "")
		t.Fatal("a second connection was accepted")
	}
	if response == nil || response.StatusCode != http.StatusConflict {
		t.Fatalf("second connection status = %v, want 409", response)
	}
}

func TestControlRejectsNonGet(t *testing.T) {
	control, err := NewControl()
	if err != nil {
		t.Fatalf("new control: %v", err)
	}
	t.Cleanup(func() { _ = control.Close() })

	response, err := http.Post(httpControlURL(t, control.URL()), "text/plain", strings.NewReader(""))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", response.StatusCode)
	}
}

func TestControlCloseIsIdempotent(t *testing.T) {
	control, err := NewControl()
	if err != nil {
		t.Fatalf("new control: %v", err)
	}
	if err := control.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := control.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
	if _, ok := <-control.Events(); ok {
		t.Fatal("events channel is still open after Close")
	}
	if err := control.Send(context.Background(), Message{Type: "show"}); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("Send after Close = %v, want ErrNotConnected", err)
	}
}
