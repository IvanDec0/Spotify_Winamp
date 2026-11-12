package webplayer

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

func TestRedactLog(t *testing.T) {
	secret := "BQ" + strings.Repeat("a", 30)
	tests := []struct {
		name     string
		line     string
		contains string
		absent   string
	}{
		{
			name:     "bearer token",
			line:     "authorization: " + secret,
			contains: "[redacted]",
			absent:   secret,
		},
		{
			name:     "json token",
			line:     `{"access_token":"` + secret + `","token_type":"Bearer"}`,
			contains: `"access_token":"[redacted]"`,
			absent:   secret,
		},
		{
			name:     "query token",
			line:     "GET /token?access_token=" + secret + "&x=1",
			contains: "access_token=[redacted]",
			absent:   secret,
		},
		{
			name:     "soloist key",
			line:     "key: spak_abcdef123456",
			contains: "[redacted]",
			absent:   "spak_abcdef123456",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := redactLog(test.line)
			if !strings.Contains(got, test.contains) {
				t.Fatalf("redacted %q does not contain %q", got, test.contains)
			}
			if strings.Contains(got, test.absent) {
				t.Fatalf("redacted %q still contains the secret", got)
			}
		})
	}

	safe := "the helper is ready"
	if got := redactLog(safe); got != safe {
		t.Fatalf("safe line changed: %q", got)
	}
}

func TestRedactingWriterRedactsAndTails(t *testing.T) {
	var destination bytes.Buffer
	tail := newTailBuffer(tailLines)
	writer := newRedactingWriter(&destination, tail)

	secret := "BQ" + strings.Repeat("z", 40)
	if _, err := writer.Write([]byte("starting\naccess_token=" + secret + "&rest\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if strings.Contains(destination.String(), secret) {
		t.Fatalf("the log destination contains the secret: %s", destination.String())
	}
	lines := tail.snapshot()
	if len(lines) != 2 {
		t.Fatalf("tail = %#v, want 2 lines", lines)
	}
	if lines[0] != "starting" {
		t.Fatalf("first tail line = %q", lines[0])
	}
	if strings.Contains(lines[1], secret) || !strings.Contains(lines[1], "[redacted]") {
		t.Fatalf("second tail line = %q", lines[1])
	}
}

func TestRedactingWriterRedactsSplitToken(t *testing.T) {
	var destination bytes.Buffer
	writer := newRedactingWriter(&destination, newTailBuffer(tailLines))
	secret := "BQ" + strings.Repeat("q", 40)
	// The first write has no newline, so the writer must buffer across the
	// token before redacting it.
	if _, err := writer.Write([]byte("access_token=" + secret[:10])); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := writer.Write([]byte(secret[10:] + "&rest\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if strings.Contains(destination.String(), secret) {
		t.Fatalf("split token was not redacted: %s", destination.String())
	}
	if !strings.Contains(destination.String(), "access_token=[redacted]") {
		t.Fatalf("destination = %q", destination.String())
	}
}

func TestRedactingWriterFlushesUnterminatedLine(t *testing.T) {
	var destination bytes.Buffer
	tail := newTailBuffer(tailLines)
	writer := newRedactingWriter(&destination, tail)
	if _, err := writer.Write([]byte("no newline")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if !strings.Contains(destination.String(), "no newline\n") {
		t.Fatalf("destination = %q", destination.String())
	}
	if lines := tail.snapshot(); len(lines) != 1 || lines[0] != "no newline" {
		t.Fatalf("tail = %#v", lines)
	}
}

func TestTailBufferCap(t *testing.T) {
	tail := newTailBuffer(3)
	for _, line := range []string{"one", "two", "three", "four", "five"} {
		tail.add(line)
	}
	lines := tail.snapshot()
	if len(lines) != 3 {
		t.Fatalf("tail has %d lines, want 3", len(lines))
	}
	if strings.Join(lines, ",") != "three,four,five" {
		t.Fatalf("tail = %#v", lines)
	}
	lines[0] = "mutated"
	if got := tail.snapshot()[0]; got != "three" {
		t.Fatalf("snapshot shares memory with the buffer: %q", got)
	}
}

func TestValidateLoopbackURLs(t *testing.T) {
	valid := []string{
		"ws://127.0.0.1:1234/control?nonce=abc",
		"http://127.0.0.1:1/player?nonce=abc",
	}
	if err := validateControlURL(valid[0]); err != nil {
		t.Fatalf("valid control URL rejected: %v", err)
	}
	if err := validatePlayerURL(valid[1]); err != nil {
		t.Fatalf("valid player URL rejected: %v", err)
	}

	invalidControl := []string{
		"ws://localhost:1234/control",
		"ws://127.0.0.1/control",
		"ws://127.0.0.1:0/control",
		"http://127.0.0.1:1234/control",
		"ws://user:pass@127.0.0.1:1234/control",
		"ws://192.168.1.1:1234/control",
		"ws://127.0.0.1:99999/control",
	}
	for _, raw := range invalidControl {
		if err := validateControlURL(raw); err == nil {
			t.Fatalf("invalid control URL accepted: %s", raw)
		}
	}
	if err := validatePlayerURL("ws://127.0.0.1:1234/player"); err == nil {
		t.Fatal("a ws player URL was accepted")
	}
}

func TestAppendTuningArgs(t *testing.T) {
	got := appendTuningArgs([]string{"base"}, HelperTuning{
		LowEndMode:      true,
		JSHeapMB:        64,
		DisableGPU:      true,
		DisableFeatures: []string{"MediaRouter", "Translate"},
	})
	want := []string{
		"base",
		"--enable-low-end-device-mode",
		"--js-flags=--max-old-space-size=64",
		"--disable-gpu",
		"--disable-software-rasterizer",
		"--disable-features=MediaRouter,Translate",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("appendTuningArgs() = %v, want %v", got, want)
	}
}
