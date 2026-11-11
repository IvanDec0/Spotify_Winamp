package helper

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestMaterialize(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "helper")
	if err := Materialize(dir); err != nil {
		t.Fatalf("materialize: %v", err)
	}

	for _, name := range []string{"package.json", "main.js"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode = %o, want 600", name, info.Mode().Perm())
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
		t.Fatal("Materialize rewrote an identical file")
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

func TestMaterializeRejectsEmptyDir(t *testing.T) {
	if err := Materialize(""); err == nil {
		t.Fatal("Materialize(\"\") error = nil")
	}
}

func TestAssetsContainNoSecrets(t *testing.T) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`BQ[A-Za-z0-9_-]{20,}`),
		regexp.MustCompile(`spak_[A-Za-z0-9]+`),
		regexp.MustCompile(`"client_secret"\s*:`),
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
				t.Fatalf("%s contains a secret-like literal matching %s", name, pattern)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk assets: %v", err)
	}
}

func TestAssetsMentionHeadlessAndActions(t *testing.T) {
	data, err := assets.ReadFile("assets/app/main.js")
	if err != nil {
		t.Fatalf("read main.js: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"show: false",
		"backgroundThrottling: false",
		"disable-background-timer-throttling",
		"disable-renderer-backgrounding",
		"disable-backgrounding-occluded-windows",
		`action !== "web-app" && action !== "soloist-key"`,
		"onBeforeSendHeaders",
		"X-Cadence-Developer",
		"login_required",
		"premium_required",
		"app_limit",
		"termsAccepted",
		"apis-used-1",
		"apis-used-3",
		"v1/consent",
		"v1/key",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("main.js does not contain %q", want)
		}
	}
}
