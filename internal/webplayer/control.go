package webplayer

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const (
	controlEventsBuffer = 16
	controlReadLimit    = 1 << 20
	controlHeaderLimit  = 5 * time.Second
)

// Message is one control-socket message. Type is "hello", "ready", "error",
// "show", "hide" or "shutdown" depending on the direction.
type Message struct {
	Type     string `json:"type"`
	Version  string `json:"version,omitempty"`
	DeviceID string `json:"deviceId,omitempty"`
	Code     string `json:"code,omitempty"`
	Message  string `json:"message,omitempty"`
}

// Control is the loopback WebSocket endpoint the helper connects to. It
// accepts one connection at a time; every connection must authenticate with
// the per-run nonce and start with a hello message.
type Control struct {
	nonce    string
	listener net.Listener
	http     *http.Server
	events   chan Message

	connMu sync.Mutex
	busy   bool
	conn   *websocket.Conn

	writeMu sync.Mutex

	eventsMu     sync.Mutex
	eventsClosed bool

	closeOnce sync.Once
	closeErr  error
}

// NewControl binds 127.0.0.1:0 and starts accepting control connections.
func NewControl() (*Control, error) {
	nonce, err := generateNonce()
	if err != nil {
		return nil, fmt.Errorf("webplayer: cannot generate a control nonce: %w", err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("webplayer: cannot bind the control socket: %w", err)
	}
	control := &Control{
		nonce:    nonce,
		listener: listener,
		events:   make(chan Message, controlEventsBuffer),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/control", control.handle)
	control.http = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: controlHeaderLimit,
	}
	go func() {
		_ = control.http.Serve(listener)
	}()
	return control, nil
}

// URL is the WebSocket URL the helper must connect to.
func (c *Control) URL() string {
	target := url.URL{
		Scheme: "ws",
		Host:   c.listener.Addr().String(),
		Path:   "/control",
	}
	query := target.Query()
	query.Set("nonce", c.nonce)
	target.RawQuery = query.Encode()
	return target.String()
}

// Events returns the buffered event stream. It is closed by Close. Events are
// dropped rather than blocking the helper when the consumer falls behind.
func (c *Control) Events() <-chan Message {
	return c.events
}

// Send writes one message to the connected helper. It fails with
// ErrNotConnected when no helper is connected.
func (c *Control) Send(ctx context.Context, message Message) error {
	if ctx == nil {
		ctx = context.Background()
	}
	c.connMu.Lock()
	conn := c.conn
	c.connMu.Unlock()
	if conn == nil {
		return ErrNotConnected
	}
	// One writer at a time: the WebSocket package allows concurrent control
	// frames but not concurrent data frames without external serialization.
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return wsjson.Write(ctx, conn, message)
}

// Close stops accepting connections, closes the active connection with a
// normal closure and closes Events. It is idempotent.
func (c *Control) Close() error {
	c.closeOnce.Do(func() {
		c.connMu.Lock()
		conn := c.conn
		c.conn = nil
		c.busy = false
		c.connMu.Unlock()
		if conn != nil {
			_ = conn.Close(websocket.StatusNormalClosure, "")
		}
		c.closeErr = c.http.Close()

		c.eventsMu.Lock()
		c.eventsClosed = true
		close(c.events)
		c.eventsMu.Unlock()
	})
	return c.closeErr
}

func (c *Control) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	got := r.URL.Query().Get("nonce")
	if subtle.ConstantTimeCompare([]byte(got), []byte(c.nonce)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	c.connMu.Lock()
	if c.busy {
		c.connMu.Unlock()
		http.Error(w, "a helper is already connected", http.StatusConflict)
		return
	}
	c.busy = true
	c.connMu.Unlock()

	// The nonce authenticates this endpoint and it is loopback-only, so the
	// Origin check adds no security here. It would also reject the Node
	// WebSocket client when it sends an Origin the helper did not choose.
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		c.connMu.Lock()
		c.busy = false
		c.connMu.Unlock()
		return
	}
	conn.SetReadLimit(controlReadLimit)

	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()

	c.readLoop(conn)

	c.connMu.Lock()
	if c.conn == conn {
		c.conn = nil
	}
	c.busy = false
	c.connMu.Unlock()
	_ = conn.Close(websocket.StatusNormalClosure, "")
}

func (c *Control) readLoop(conn *websocket.Conn) {
	ctx := context.Background()
	helloSeen := false
	for {
		var message Message
		if err := wsjson.Read(ctx, conn, &message); err != nil {
			return
		}
		if !helloSeen {
			if message.Type != "hello" {
				c.emit(Message{
					Type:    "error",
					Code:    "protocol_error",
					Message: "the first control message must be hello",
				})
				_ = conn.Close(websocket.StatusPolicyViolation, "hello required")
				return
			}
			helloSeen = true
			continue
		}
		switch message.Type {
		case "ready", "error":
			c.emit(message)
		}
	}
}

func (c *Control) emit(message Message) {
	c.eventsMu.Lock()
	defer c.eventsMu.Unlock()
	if c.eventsClosed {
		return
	}
	select {
	case c.events <- message:
	default:
		// Keep the read loop moving; a stalled consumer loses diagnostics,
		// never the socket.
	}
}
