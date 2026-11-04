package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/coder/websocket"
)

// eventReadLimit bounds one engine event frame. Engine events are small; the
// limit only protects the client from an unexpected payload.
const eventReadLimit = 1 << 20

// Event is one envelope from the engine's /events stream. Data keeps the
// engine-specific payload untouched until a caller needs it.
type Event struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// WatchEvents connects to the engine event stream and calls handler for every
// valid event frame, in order. Malformed frames are skipped. It blocks until
// ctx is cancelled or the connection fails, and always closes the connection
// before returning.
func (c *Client) WatchEvents(ctx context.Context, handler func(Event)) error {
	endpoint, err := eventURL(c.baseURL)
	if err != nil {
		return err
	}
	conn, _, err := websocket.Dial(ctx, endpoint, nil)
	if err != nil {
		return fmt.Errorf("connect to the go-librespot event stream: %w", err)
	}
	defer func() {
		if ctx.Err() != nil {
			_ = conn.CloseNow()
			return
		}
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}()
	conn.SetReadLimit(eventReadLimit)

	for {
		messageType, payload, err := conn.Read(ctx)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			return fmt.Errorf("read go-librespot event: %w", err)
		}
		if messageType != websocket.MessageText && messageType != websocket.MessageBinary {
			continue
		}
		var event Event
		if err := json.Unmarshal(payload, &event); err != nil || event.Type == "" {
			continue
		}
		if handler != nil {
			handler(event)
		}
	}
}

// eventURL turns the loopback API base URL into its WebSocket /events endpoint.
func eventURL(baseURL string) (string, error) {
	endpoint, err := url.Parse(baseURL)
	if err != nil {
		return "", errors.New("invalid go-librespot API address")
	}
	switch endpoint.Scheme {
	case "http", "ws":
		endpoint.Scheme = "ws"
	case "https", "wss":
		endpoint.Scheme = "wss"
	default:
		return "", fmt.Errorf("unsupported go-librespot API scheme %q", endpoint.Scheme)
	}
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/events"
	return endpoint.String(), nil
}
