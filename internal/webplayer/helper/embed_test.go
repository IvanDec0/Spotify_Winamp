package helper

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestPage(t *testing.T) {
	page := Page()
	if len(page) == 0 {
		t.Fatal("Page() is empty")
	}
	text := string(page)
	for _, want := range []string{"/player.js", "sdk.scdn.co"} {
		if !strings.Contains(text, want) {
			t.Fatalf("Page() does not contain %q", want)
		}
	}
	if strings.Contains(text, "Spotify.Player") {
		t.Fatal("Page() must not contain inline glue; it belongs in Script()")
	}
	// Page returns a copy, not a mutable view of the embedded asset.
	page[0] = 'X'
	if strings.HasPrefix(string(Page()), "X") {
		t.Fatal("Page() returned a shared buffer")
	}
}

func TestScript(t *testing.T) {
	script := Script()
	if len(script) == 0 {
		t.Fatal("Script() is empty")
	}
	text := string(script)
	for _, want := range []string{"onSpotifyWebPlaybackSDKReady", "Spotify.Player", `name: "Cadence Web"`, "/token?nonce="} {
		if !strings.Contains(text, want) {
			t.Fatalf("Script() does not contain %q", want)
		}
	}
	script[0] = 'X'
	if strings.HasPrefix(string(Script()), "X") {
		t.Fatal("Script() returned a shared buffer")
	}
}

func TestMaterialize(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "helper")
	if err := Materialize(dir); err != nil {
		t.Fatalf("materialize: %v", err)
	}

	expected := map[string]bool{"package.json": false, "main.js": false, "preload.js": false}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read helper dir: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if _, ok := expected[entry.Name()]; !ok {
			t.Fatalf("unexpected helper file %q", entry.Name())
		}
		expected[entry.Name()] = true
		info, err := entry.Info()
		if err != nil {
			t.Fatalf("stat %s: %v", entry.Name(), err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode = %o, want 600", entry.Name(), info.Mode().Perm())
		}
	}
	for name, found := range expected {
		if !found {
			t.Fatalf("missing helper file %q", name)
		}
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat helper dir: %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("helper dir mode = %o, want 700", info.Mode().Perm())
	}

	before, err := os.Stat(filepath.Join(dir, "main.js"))
	if err != nil {
		t.Fatalf("stat before: %v", err)
	}
	if err := Materialize(dir); err != nil {
		t.Fatalf("second materialize: %v", err)
	}
	after, err := os.Stat(filepath.Join(dir, "main.js"))
	if err != nil {
		t.Fatalf("stat after: %v", err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("Materialize rewrote an identical file instead of skipping it")
	}
}

func TestMaterializeReplacesStaleContent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "helper")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mainPath := filepath.Join(dir, "main.js")
	if err := os.WriteFile(mainPath, []byte("stale"), 0o600); err != nil {
		t.Fatalf("write stale: %v", err)
	}
	if err := Materialize(dir); err != nil {
		t.Fatalf("materialize: %v", err)
	}
	data, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) == "stale" || len(data) == 0 {
		t.Fatalf("stale helper content was not replaced: %q", data)
	}
}

func TestAssetsContainNoTokenLiterals(t *testing.T) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`BQ[A-Za-z0-9_-]{20,}`),
		regexp.MustCompile(`access_token=[^&\s]+`),
		regexp.MustCompile(`"access_token"\s*:\s*"[^"]*"`),
	}
	err := fs.WalkDir(assets, "assets", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(assets, name)
		if err != nil {
			return err
		}
		for _, pattern := range patterns {
			if pattern.Match(data) {
				t.Fatalf("%s contains a token-like literal matching %s", name, pattern)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk assets: %v", err)
	}
}
