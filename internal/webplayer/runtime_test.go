package webplayer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// testRuntime serves one zip archive from a TLS test server and returns a
// runtime wired to it.
type testRuntime struct {
	runtime  *Runtime
	manifest *Manifest
	root     string
	requests *atomic.Int64
}

func newTestRuntime(t *testing.T, archive []byte, mutate func(*Platform)) *testRuntime {
	t.Helper()
	var requests atomic.Int64
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/runtime.zip" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(archive)
	}))
	t.Cleanup(server.Close)

	sum := sha256.Sum256(archive)
	platform := Platform{
		URL:    server.URL + "/runtime.zip",
		SHA256: hex.EncodeToString(sum[:]),
		Size:   int64(len(archive)),
		Entry:  "electron",
	}
	if mutate != nil {
		mutate(&platform)
	}
	manifest := &Manifest{
		Schema:  manifestSchema,
		Version: "1.2.3-test",
		Platforms: map[string]Platform{
			"linux/amd64": platform,
		},
	}
	root := t.TempDir()
	return &testRuntime{
		runtime: NewRuntime(RuntimeOptions{
			Root:             root,
			HTTPClient:       server.Client(),
			AllowAnyHost:     true,
			GOOS:             "linux",
			GOARCH:           "amd64",
			Manifest:         manifest,
			MaxDownloadBytes: 64 << 20,
			MaxExtractBytes:  64 << 20,
			MaxFiles:         100,
			Now:              func() time.Time { return time.Unix(1_700_000_000, 0) },
		}),
		manifest: manifest,
		root:     root,
		requests: &requests,
	}
}

func testRuntimeArchive(t *testing.T) []byte {
	t.Helper()
	return readTestFile(t, writeTestZip(t, []testZipEntry{
		{Name: "electron", Body: "runtime-binary", Mode: 0o755},
		{Name: "resources/app.asar", Body: "asar"},
		{Name: "locales/en-US.pak", Body: "pak"},
	}))
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func TestRuntimeEnsureInstalls(t *testing.T) {
	archive := testRuntimeArchive(t)
	fixture := newTestRuntime(t, archive, nil)

	install, err := fixture.runtime.Ensure(context.Background())
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}

	wantDir := filepath.Join(fixture.root, "runtime", "1.2.3-test-linux-amd64")
	if install.Version != "1.2.3-test" {
		t.Fatalf("version = %q", install.Version)
	}
	if install.Dir != wantDir {
		t.Fatalf("dir = %q, want %q", install.Dir, wantDir)
	}
	if install.Executable != filepath.Join(wantDir, "electron") {
		t.Fatalf("executable = %q", install.Executable)
	}
	if install.HelperDir != filepath.Join(fixture.root, "helper") {
		t.Fatalf("helper dir = %q", install.HelperDir)
	}

	assertFileContent(t, install.Executable, "runtime-binary")
	assertMode(t, install.Executable, 0o755)

	marker := readTestFile(t, filepath.Join(wantDir, runtimeMarkerName))
	if strings.TrimSpace(string(marker)) != fixture.manifest.Platforms["linux/amd64"].SHA256 {
		t.Fatalf("marker = %q", marker)
	}

	for _, name := range []string{"package.json", "main.js", "preload.js"} {
		info, err := os.Stat(filepath.Join(install.HelperDir, name))
		if err != nil {
			t.Fatalf("helper file %s: %v", name, err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("helper file %s mode = %o, want 600", name, info.Mode().Perm())
		}
	}

	installed, ok := fixture.runtime.Installed()
	if !ok {
		t.Fatal("Installed() = false after a successful install")
	}
	if installed.Dir != wantDir {
		t.Fatalf("Installed().Dir = %q", installed.Dir)
	}

	before := fixture.requests.Load()
	if _, err := fixture.runtime.Ensure(context.Background()); err != nil {
		t.Fatalf("second ensure: %v", err)
	}
	if after := fixture.requests.Load(); after != before {
		t.Fatalf("second Ensure made %d network requests", after-before)
	}
}

func TestRuntimeInstallProgress(t *testing.T) {
	archive := testRuntimeArchive(t)
	fixture := newTestRuntime(t, archive, nil)

	var stages []string
	_, err := fixture.runtime.Install(context.Background(), func(progress Progress) {
		stages = append(stages, progress.Stage)
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	for _, want := range []string{"downloading", "extracting", "installing"} {
		found := false
		for _, stage := range stages {
			if stage == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("progress stages %v do not include %q", stages, want)
		}
	}
}

func TestRuntimeChecksumMismatchLeavesNoInstall(t *testing.T) {
	archive := testRuntimeArchive(t)
	fixture := newTestRuntime(t, archive, func(platform *Platform) {
		platform.SHA256 = strings.Repeat("0", 64)
	})

	_, err := fixture.runtime.Ensure(context.Background())
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("error = %v, want ErrChecksumMismatch", err)
	}
	if _, ok := fixture.runtime.Installed(); ok {
		t.Fatal("Installed() = true after a checksum failure")
	}
	entries, readErr := os.ReadDir(filepath.Join(fixture.root, "tmp"))
	if readErr != nil {
		t.Fatalf("read staging dir: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("staging directory was not cleaned: %v", entries)
	}
}

func TestRuntimeRejectsWrongDownloadSize(t *testing.T) {
	archive := testRuntimeArchive(t)
	tests := []struct {
		name string
		body []byte
		size int64
	}{
		{
			name: "too large",
			body: append(append([]byte(nil), archive...), make([]byte, downloadSizeTolerance+1024)...),
			size: int64(len(archive)),
		},
		{
			name: "too small",
			body: archive,
			size: int64(len(archive)) + downloadSizeTolerance + 1024,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write(test.body)
			}))
			defer server.Close()
			sum := sha256.Sum256(test.body)
			manifest := &Manifest{
				Schema:  manifestSchema,
				Version: "1.2.3-test",
				Platforms: map[string]Platform{
					"linux/amd64": {
						URL:    server.URL + "/runtime.zip",
						SHA256: hex.EncodeToString(sum[:]),
						Size:   test.size,
						Entry:  "electron",
					},
				},
			}
			runtime := NewRuntime(RuntimeOptions{
				Root:         t.TempDir(),
				HTTPClient:   server.Client(),
				AllowAnyHost: true,
				GOOS:         "linux",
				GOARCH:       "amd64",
				Manifest:     manifest,
			})
			_, err := runtime.Install(context.Background(), nil)
			if err == nil || CodeOf(err) != CodeDownloadFailed {
				t.Fatalf("error = %v, want a download size failure", err)
			}
			if _, ok := runtime.Installed(); ok {
				t.Fatal("Installed() = true after a size failure")
			}
		})
	}
}

func TestCheckDownloadURL(t *testing.T) {
	allowed := []string{
		"https://github.com/castlabs/electron-releases/releases/download/x.zip",
		"https://objects.githubusercontent.com/x.zip",
		"https://githubusercontent.com/x.zip",
		"https://api.github.com/repos/x",
	}
	for _, raw := range allowed {
		target, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("parse %s: %v", raw, err)
		}
		if err := checkDownloadURL(target); err != nil {
			t.Fatalf("%s rejected: %v", raw, err)
		}
	}
	rejected := []string{
		"http://github.com/x.zip",
		"https://evil.com/x.zip",
		"https://github.com.evil.com/x.zip",
		"https://notgithub.com/x.zip",
		"https://user:secret@github.com/x.zip",
	}
	for _, raw := range rejected {
		target, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("parse %s: %v", raw, err)
		}
		if err := checkDownloadURL(target); err == nil {
			t.Fatalf("%s was accepted", raw)
		}
	}
}

func TestRuntimeReplacesInvalidInstall(t *testing.T) {
	archive := testRuntimeArchive(t)
	fixture := newTestRuntime(t, archive, nil)

	install, err := fixture.runtime.Ensure(context.Background())
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if err := os.Remove(install.Executable); err != nil {
		t.Fatalf("remove executable: %v", err)
	}
	if _, ok := fixture.runtime.Installed(); ok {
		t.Fatal("Installed() = true without the entry executable")
	}

	before := fixture.requests.Load()
	install, err = fixture.runtime.Ensure(context.Background())
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if after := fixture.requests.Load(); after != before+1 {
		t.Fatalf("replacement made %d requests, want 1", after-before)
	}
	assertFileContent(t, install.Executable, "runtime-binary")
}

func TestRuntimeRejectsNonGitHubHostByDefault(t *testing.T) {
	archive := testRuntimeArchive(t)
	fixture := newTestRuntime(t, archive, nil)
	runtime := NewRuntime(RuntimeOptions{
		Root:     t.TempDir(),
		GOOS:     "linux",
		GOARCH:   "amd64",
		Manifest: fixture.manifest,
	})

	_, err := runtime.Install(context.Background(), nil)
	if err == nil || CodeOf(err) != CodeDownloadFailed {
		t.Fatalf("error = %v, want a download failure", err)
	}
	if fixture.requests.Load() != 0 {
		t.Fatal("the allowlist rejection still hit the network")
	}
}

func TestRuntimeUnsupportedPlatform(t *testing.T) {
	archive := testRuntimeArchive(t)
	fixture := newTestRuntime(t, archive, nil)
	runtime := NewRuntime(RuntimeOptions{
		Root:     t.TempDir(),
		GOOS:     "darwin",
		GOARCH:   "arm64",
		Manifest: fixture.manifest,
	})
	if _, err := runtime.Install(context.Background(), nil); !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("error = %v, want ErrUnsupportedPlatform", err)
	}
}

func TestRuntimePrune(t *testing.T) {
	runtime := NewRuntime(RuntimeOptions{Root: t.TempDir(), GOOS: "linux", GOARCH: "amd64"})
	runtimeDir := filepath.Join(runtime.root, "runtime")
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	now := time.Now()
	dirs := map[string]time.Duration{
		"keep-me": -5 * time.Hour,
		"middle":  -2 * time.Hour,
		"newest":  -1 * time.Hour,
		"older":   -3 * time.Hour,
	}
	for name, age := range dirs {
		path := filepath.Join(runtimeDir, name)
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
		stamp := now.Add(age)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatalf("chtimes %s: %v", name, err)
		}
	}
	stray := filepath.Join(runtimeDir, "stray.txt")
	if err := os.WriteFile(stray, []byte("keep me"), 0o600); err != nil {
		t.Fatalf("write stray: %v", err)
	}

	if err := runtime.Prune("keep-me"); err != nil {
		t.Fatalf("prune: %v", err)
	}
	for _, name := range []string{"keep-me", "newest"} {
		if _, err := os.Stat(filepath.Join(runtimeDir, name)); err != nil {
			t.Fatalf("%s should have been kept: %v", name, err)
		}
	}
	for _, name := range []string{"middle", "older"} {
		if _, err := os.Stat(filepath.Join(runtimeDir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s should have been pruned: %v", name, err)
		}
	}
	if _, err := os.Stat(stray); err != nil {
		t.Fatalf("Prune removed a non-directory: %v", err)
	}
}

func TestRuntimePruneKeepsNewestWhenKeepIsUnknown(t *testing.T) {
	runtime := NewRuntime(RuntimeOptions{Root: t.TempDir(), GOOS: "linux", GOARCH: "amd64"})
	runtimeDir := filepath.Join(runtime.root, "runtime")
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	now := time.Now()
	for name, age := range map[string]time.Duration{"old": -2 * time.Hour, "new": -time.Hour} {
		path := filepath.Join(runtimeDir, name)
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
		stamp := now.Add(age)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatalf("chtimes %s: %v", name, err)
		}
	}
	if err := runtime.Prune(""); err != nil {
		t.Fatalf("prune: %v", err)
	}
	if _, err := os.Stat(filepath.Join(runtimeDir, "new")); err != nil {
		t.Fatalf("newest was removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(runtimeDir, "old")); !os.IsNotExist(err) {
		t.Fatalf("oldest was kept: %v", err)
	}
}
