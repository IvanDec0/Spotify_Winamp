package soloist

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const (
	// defaultCommandTimeout is used when Do or Await is given a zero or
	// negative timeout.
	defaultCommandTimeout = 5 * time.Second
	// maxMessageBytes bounds one WebSocket frame. Playback snapshots with
	// many covers stay well below it.
	maxMessageBytes = 8 << 20
	// subscriberBuffer is how many events a subscriber may fall behind
	// before the oldest queued event is dropped.
	subscriberBuffer = 128
)

// waitResult is delivered once to a waiter.
type waitResult struct {
	event Event
	err   error
}

// waiter is one pending Await or Do registration.
type waiter struct {
	want    string
	command string
	out     chan waitResult
}

// Client is one loopback WebSocket connection to Soloist.
//
// Soloist has no request ids: a query is answered by the next event of the
// requested type. The one read goroutine decodes frames, resolves waiters and
// fans events out to subscribers; it never blocks on a subscriber.
type Client struct {
	conn *websocket.Conn

	// writeMu serializes writes: coder/websocket permits one writer.
	writeMu sync.Mutex
	// cmdMu serializes Do so its waiter is registered before its write.
	cmdMu sync.Mutex

	mu          sync.Mutex
	waiters     []*waiter
	pendingAuth *Event
	subscribers map[int]chan Event
	nextSub     int
	err         error
	finishOnce  sync.Once
	closeOnce   sync.Once
	closeErr    error
	done        chan struct{}
}

// Dial opens a WebSocket connection to Soloist, normally
// ws://127.0.0.1:PORT. The one read goroutine starts before Dial returns.
func Dial(ctx context.Context, rawURL string) (*Client, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	conn, _, err := websocket.Dial(ctx, rawURL, nil)
	if err != nil {
		return nil, codedError(CodeStartFailed, "cannot connect to Spotify Soloist", err)
	}
	conn.SetReadLimit(maxMessageBytes)
	client := &Client{
		conn:        conn,
		subscribers: make(map[int]chan Event),
		done:        make(chan struct{}),
	}
	go client.readLoop()
	return client, nil
}

// Do sends a command and waits for the next event of type want. Registering
// the waiter before the write means an immediate reply is never lost.
//
// Do holds an internal command mutex, so concurrent calls serialize. Use
// "command_result" for control commands and the query's event type for
// queries.
//
// A command_result only means Soloist accepted the command; it does not prove
// that playback changed. Callers that need the new state must wait for the
// matching state event as well.
func (c *Client) Do(ctx context.Context, cmd Command, want string, timeout time.Duration) (Event, error) {
	if want == "" {
		want = "command_result"
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := commandContext(ctx, timeout)
	defer cancel()

	c.cmdMu.Lock()
	defer c.cmdMu.Unlock()

	filter := ""
	if want == "command_result" {
		filter = cmd.Command
	}
	w, replayed := c.register(want, filter, false)
	if replayed != nil {
		return *replayed, nil
	}
	defer c.removeWaiter(w)

	if err := c.Send(ctx, cmd); err != nil {
		return Event{}, err
	}
	select {
	case result := <-w.out:
		return result.event, result.err
	case <-ctx.Done():
		return Event{}, codedError(CodeProtocol, "Spotify Soloist did not answer in time", ctx.Err())
	case <-c.done:
		return Event{}, c.closedError()
	}
}

// Await waits for the next event of type want. auth_state is replayed from
// the connection handshake when it arrived before Await was called, because
// Soloist sends it immediately on connect.
func (c *Client) Await(ctx context.Context, want string, timeout time.Duration) (Event, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := commandContext(ctx, timeout)
	defer cancel()

	w, replayed := c.register(want, "", true)
	if replayed != nil {
		return *replayed, nil
	}
	defer c.removeWaiter(w)

	select {
	case result := <-w.out:
		return result.event, result.err
	case <-ctx.Done():
		return Event{}, codedError(CodeProtocol, "Spotify Soloist did not answer in time", ctx.Err())
	case <-c.done:
		return Event{}, c.closedError()
	}
}

// Send writes one command without waiting for a reply.
func (c *Client) Send(ctx context.Context, cmd Command) error {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-c.done:
		return c.closedError()
	default:
	}
	data, err := json.Marshal(cmd)
	if err != nil {
		return codedError(CodeProtocol, "cannot encode a Spotify Soloist command", err)
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := c.conn.Write(ctx, websocket.MessageText, data); err != nil {
		return codedError(CodeProtocol, "cannot send a Spotify Soloist command", err)
	}
	return nil
}

// Subscribe returns a buffered event stream. The reader never blocks on it:
// when the buffer is full the oldest queued event is dropped. The returned
// unsubscribe function is idempotent and safe to call at any time.
func (c *Client) Subscribe() (<-chan Event, func()) {
	events := make(chan Event, subscriberBuffer)
	c.mu.Lock()
	id := c.nextSub
	c.nextSub++
	c.subscribers[id] = events
	c.mu.Unlock()
	var once sync.Once
	return events, func() {
		once.Do(func() {
			c.mu.Lock()
			delete(c.subscribers, id)
			c.mu.Unlock()
		})
	}
}

// Done closes when the connection ends or Close is called.
func (c *Client) Done() <-chan struct{} {
	return c.done
}

// Err reports why the connection ended. It is nil after a clean close.
func (c *Client) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

// Close closes the underlying connection. It is safe to call more than once.
func (c *Client) Close() error {
	c.closeOnce.Do(func() {
		err := c.conn.Close(websocket.StatusNormalClosure, "")
		c.finish(nil)
		c.closeErr = normalizeCloseError(err)
	})
	return c.closeErr
}

func (c *Client) readLoop() {
	for {
		_, data, err := c.conn.Read(context.Background())
		if err != nil {
			c.finish(err)
			return
		}
		event, err := decodeEvent(data)
		if err != nil {
			// A malformed frame must not take the connection down; the
			// documented protocol is the only thing this client consumes.
			continue
		}
		c.dispatch(event)
	}
}

func (c *Client) dispatch(event Event) {
	c.mu.Lock()
	if event.Type == "error" {
		failure := codedError(CodeProtocol, event.Message, nil)
		for _, w := range c.waiters {
			w.out <- waitResult{err: failure}
		}
		c.waiters = nil
	} else {
		matched := false
		for index, w := range c.waiters {
			if w.want != event.Type {
				continue
			}
			if w.command != "" && event.Command != "" && w.command != event.Command {
				continue
			}
			w.out <- waitResult{event: event}
			c.waiters = append(c.waiters[:index], c.waiters[index+1:]...)
			matched = true
			break
		}
		if !matched && event.Type == "auth_state" {
			stored := event
			c.pendingAuth = &stored
		}
	}
	subscribers := make([]chan Event, 0, len(c.subscribers))
	for _, events := range c.subscribers {
		subscribers = append(subscribers, events)
	}
	c.mu.Unlock()

	for _, events := range subscribers {
		sendLatest(events, event)
	}
}

func (c *Client) register(want, filter string, replayAuth bool) (*waiter, *Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if replayAuth && want == "auth_state" && c.pendingAuth != nil {
		event := *c.pendingAuth
		c.pendingAuth = nil
		return nil, &event
	}
	w := &waiter{want: want, command: filter, out: make(chan waitResult, 1)}
	c.waiters = append(c.waiters, w)
	return w, nil
}

func (c *Client) removeWaiter(w *waiter) {
	if w == nil {
		return
	}
	c.mu.Lock()
	for index, current := range c.waiters {
		if current == w {
			c.waiters = append(c.waiters[:index], c.waiters[index+1:]...)
			break
		}
	}
	c.mu.Unlock()
}

func (c *Client) finish(err error) {
	c.finishOnce.Do(func() {
		clean := normalizeCloseError(err)
		c.mu.Lock()
		c.err = clean
		failure := codedError(CodeProtocol, "the Spotify Soloist connection closed", clean)
		for _, w := range c.waiters {
			w.out <- waitResult{err: failure}
		}
		c.waiters = nil
		c.mu.Unlock()
		close(c.done)
	})
}

func (c *Client) closedError() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return codedError(CodeProtocol, "the Spotify Soloist connection is closed", c.err)
}

func commandContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = defaultCommandTimeout
	}
	return context.WithTimeout(ctx, timeout)
}

func sendLatest(events chan Event, event Event) {
	select {
	case events <- event:
		return
	default:
	}
	select {
	case <-events:
	default:
	}
	select {
	case events <- event:
	default:
	}
}

func normalizeCloseError(err error) error {
	if err == nil {
		return nil
	}
	status := websocket.CloseStatus(err)
	if status == websocket.StatusNormalClosure || status == websocket.StatusGoingAway {
		return nil
	}
	if errors.Is(err, net.ErrClosed) || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
