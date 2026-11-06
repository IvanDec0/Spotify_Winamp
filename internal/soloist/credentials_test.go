package soloist

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAPIKeyRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "soloist.key")
	if err := SaveAPIKey(path, "  abc123def456  "); err != nil {
		t.Fatalf("save: %v", err)
	}
	key, err := ReadAPIKey(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if key != "abc123def456" {
		t.Fatalf("key = %q", key)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", info.Mode().Perm())
	}
}

func TestAPIKeyOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "soloist.key")
	if err := SaveAPIKey(path, "first"); err != nil {
		t.Fatalf("save first: %v", err)
	}
	if err := SaveAPIKey(path, "second"); err != nil {
		t.Fatalf("save second: %v", err)
	}
	key, err := ReadAPIKey(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if key != "second" {
		t.Fatalf("key = %q", key)
	}
}

func TestClearAPIKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "soloist.key")
	if err := SaveAPIKey(path, "secret"); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := ClearAPIKey(path); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if _, err := ReadAPIKey(path); !errors.Is(err, ErrAPIKeyMissing) {
		t.Fatalf("read after clear = %v, want ErrAPIKeyMissing", err)
	}
	if err := ClearAPIKey(path); err != nil {
		t.Fatalf("clear missing: %v", err)
	}
}

func TestReadAPIKeyMissing(t *testing.T) {
	_, err := ReadAPIKey(filepath.Join(t.TempDir(), "missing"))
	if !errors.Is(err, ErrAPIKeyMissing) {
		t.Fatalf("error = %v", err)
	}
}

func TestReadAPIKeyEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty")
	if err := os.WriteFile(path, []byte("  \n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := ReadAPIKey(path); !errors.Is(err, ErrAPIKeyMissing) {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateAPIKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{name: "valid", key: "abc123def456"},
		{name: "trimmed", key: "  abc123  "},
		{name: "empty", key: "", wantErr: true},
		{name: "whitespace", key: "   ", wantErr: true},
		{name: "inner space", key: "abc 123", wantErr: true},
		{name: "tab", key: "abc\t123", wantErr: true},
		{name: "newline", key: "abc\n123", wantErr: true},
		{name: "control", key: "abc\x01123", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateAPIKey(test.key)
			if test.wantErr {
				if err == nil || CodeOf(err) != CodeAPIKeyInvalid {
					t.Fatalf("error = %v, want CodeAPIKeyInvalid", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestAPIKeyErrorsNeverContainTheKey(t *testing.T) {
	const secret = "abc123def456"
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	err := SaveAPIKey(filepath.Join(blocker, "soloist.key"), secret)
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error leaks the key: %v", err)
	}
	if err := ValidateAPIKey(secret + " " + secret); err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("validation error = %v", err)
	}
}
