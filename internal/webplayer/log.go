package webplayer

import (
	"bytes"
	"io"
	"regexp"
	"strings"
	"sync"
)

const (
	// tailLines is how many redacted log lines are kept for diagnostics.
	tailLines = 200
	// tailLineLimit bounds one kept line, in runes.
	tailLineLimit = 4096
	// pendingLogLimit bounds an unterminated log line held in memory.
	pendingLogLimit = 64 << 10
	// redactionOverlap keeps enough trailing bytes when flushing a huge
	// unterminated line that a secret split across writes is still caught.
	redactionOverlap = 4096
)

var (
	// bearerTokenPattern matches Spotify access tokens (they start with BQ).
	bearerTokenPattern = regexp.MustCompile(`BQ[A-Za-z0-9_-]{20,}`)
	// jsonTokenPattern matches an access_token field in JSON output.
	jsonTokenPattern = regexp.MustCompile(`"access_token"\s*:\s*"[^"]*"`)
	// queryTokenPattern matches an access_token query parameter.
	queryTokenPattern = regexp.MustCompile(`access_token=[^&\s]+`)
)

// redactLog removes token-shaped secrets from one log line. It never logs the
// replacement back as a secret.
func redactLog(line string) string {
	line = jsonTokenPattern.ReplaceAllString(line, `"access_token":"[redacted]"`)
	line = bearerTokenPattern.ReplaceAllString(line, "[redacted]")
	line = queryTokenPattern.ReplaceAllString(line, "access_token=[redacted]")
	return line
}

// redactingWriter forwards child process output to a destination after
// redacting token-shaped strings, and keeps a bounded tail for diagnostics.
type redactingWriter struct {
	mu          sync.Mutex
	destination io.Writer
	tail        *tailBuffer
	pending     []byte
	closed      bool
}

func newRedactingWriter(destination io.Writer, tail *tailBuffer) *redactingWriter {
	return &redactingWriter{destination: destination, tail: tail}
}

func (w *redactingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return len(p), nil
	}
	w.pending = append(w.pending, p...)
	for {
		index := bytes.IndexByte(w.pending, '\n')
		if index < 0 {
			break
		}
		w.writeLine(w.pending[:index+1])
		w.pending = w.pending[index+1:]
	}
	if len(w.pending) > pendingLogLimit {
		keep := redactionOverlap
		if keep > pendingLogLimit {
			keep = pendingLogLimit
		}
		if keep > len(w.pending) {
			keep = len(w.pending)
		}
		cut := len(w.pending) - keep
		w.writeLine(w.pending[:cut])
		w.pending = append([]byte(nil), w.pending[cut:]...)
	}
	return len(p), nil
}

func (w *redactingWriter) writeLine(line []byte) {
	text := redactLog(string(line))
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	if w.destination != nil {
		_, _ = io.WriteString(w.destination, text)
	}
	if w.tail != nil {
		w.tail.add(truncateRunes(strings.TrimRight(text, "\r\n"), tailLineLimit))
	}
}

func (w *redactingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	if len(w.pending) > 0 {
		w.writeLine(w.pending)
		w.pending = nil
	}
	if closer, ok := w.destination.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func truncateRunes(text string, limit int) string {
	if limit <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit])
}

// tailBuffer keeps the last max lines in memory.
type tailBuffer struct {
	mu    sync.Mutex
	lines []string
	max   int
}

func newTailBuffer(max int) *tailBuffer {
	return &tailBuffer{max: max}
}

func (t *tailBuffer) add(line string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lines = append(t.lines, line)
	if t.max > 0 && len(t.lines) > t.max {
		t.lines = t.lines[len(t.lines)-t.max:]
	}
}

func (t *tailBuffer) reset() {
	t.mu.Lock()
	t.lines = nil
	t.mu.Unlock()
}

// snapshot returns a copy so callers cannot mutate the buffer.
func (t *tailBuffer) snapshot() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.lines...)
}
