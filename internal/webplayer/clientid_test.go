package webplayer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClientIDPath(t *testing.T) {
	got := ClientIDPath("/tmp/cadence")
	if got != filepath.Join("/tmp/cadence", "client-id") {
		t.Fatalf("ClientIDPath() = %q", got)
	}
}

func TestLoadClientID(t *testing.T) {
	valid := "7fe450922d8b4d31ac8bda4a1767d903"
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "valid", content: valid, want: valid},
		{name: "trimmed", content: "  " + valid + "\n", want: valid},
		{name: "uppercase", content: strings.ToUpper(valid), want: ""},
		{name: "too short", content: valid[:31], want: ""},
		{name: "non hex", content: "z" + valid[1:], want: ""},
		{name: "empty", content: "", want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(ClientIDPath(dir), []byte(test.content), 0o600); err != nil {
				t.Fatalf("write client id: %v", err)
			}
			if got := LoadClientID(dir); got != test.want {
				t.Fatalf("LoadClientID() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestLoadClientIDMissingOrInvalidPath(t *testing.T) {
	if got := LoadClientID(""); got != "" {
		t.Fatalf("LoadClientID(\"\") = %q", got)
	}
	if got := LoadClientID(t.TempDir()); got != "" {
		t.Fatalf("LoadClientID(missing file) = %q", got)
	}

	dir := t.TempDir()
	if err := os.Mkdir(ClientIDPath(dir), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if got := LoadClientID(dir); got != "" {
		t.Fatalf("LoadClientID(directory) = %q", got)
	}
}
