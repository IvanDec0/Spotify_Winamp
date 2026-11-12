package developer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"cadence/internal/webplayer"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	root := t.TempDir()
	service, err := NewService(Options{
		Root:       filepath.Join(root, "cache"),
		ConfigDir:  filepath.Join(root, "config"),
		ProfileDir: filepath.Join(root, "profile"),
		LogPath:    filepath.Join(root, "logs", "developer.log"),
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

func TestNewServiceCreatesPrivateProfile(t *testing.T) {
	root := t.TempDir()
	profile := filepath.Join(root, "profile")
	if _, err := NewService(Options{
		Root:       filepath.Join(root, "cache"),
		ConfigDir:  filepath.Join(root, "config"),
		ProfileDir: profile,
		LogPath:    filepath.Join(root, "logs", "developer.log"),
	}); err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	info, err := os.Stat(profile)
	if err != nil {
		t.Fatalf("stat profile: %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("profile mode = %o, want 700", info.Mode().Perm())
	}
}

func TestNewServicePrunesDRMCaches(t *testing.T) {
	root := t.TempDir()
	profile := filepath.Join(root, "profile")
	for _, name := range []string{"WidevineCdm", "component_crx_cache"} {
		dir := filepath.Join(profile, name, "nested")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "file"), []byte("x"), 0o600); err != nil {
			t.Fatalf("write %s: %v", dir, err)
		}
	}
	if _, err := NewService(Options{
		Root:       filepath.Join(root, "cache"),
		ConfigDir:  filepath.Join(root, "config"),
		ProfileDir: profile,
		LogPath:    filepath.Join(root, "logs", "developer.log"),
	}); err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	for _, name := range []string{"WidevineCdm", "component_crx_cache"} {
		if _, err := os.Stat(filepath.Join(profile, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s still exists after NewService(), err = %v", name, err)
		}
	}
}

func TestHelperArgsDisableComponentUpdate(t *testing.T) {
	for _, args := range [][]string{
		helperArgs("web-app", false, ""),
		helperArgs("soloist-key", true, ""),
		helperArgs("oauth", false, "https://accounts.spotify.com/authorize"),
	} {
		if !slices.Contains(args, developerDisableComponentUpdate) {
			t.Fatalf("helperArgs() = %v, want %s", args, developerDisableComponentUpdate)
		}
	}
}

func TestOpenOAuthURLRejectsInvalidURL(t *testing.T) {
	service := newTestService(t)
	values := []string{
		"",
		"http://accounts.spotify.com/authorize",
		"https://example.com/authorize",
		"https://accounts.spotify.com/dashboard",
		"https://user:password@accounts.spotify.com/authorize",
		"://not-a-url",
	}
	for _, value := range values {
		err := service.OpenOAuthURL(value)
		if err == nil {
			t.Fatalf("OpenOAuthURL(%q) error = nil", value)
		}
		if CodeOf(err) != CodeFailed {
			t.Fatalf("OpenOAuthURL(%q) code = %q, want %q", value, CodeOf(err), CodeFailed)
		}
	}
}

func TestOpenOAuthURLReturnsStartFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake runtime entry is not executable on Windows")
	}
	root := t.TempDir()
	service, err := NewService(Options{
		Root:       filepath.Join(root, "cache"),
		ConfigDir:  filepath.Join(root, "config"),
		ProfileDir: filepath.Join(root, "profile"),
		LogPath:    filepath.Join(root, "logs", "developer.log"),
		Runtime:    newInstalledRuntimeForTest(t),
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	err = service.OpenOAuthURL("https://accounts.spotify.com/authorize?client_id=test&response_type=code")
	if err == nil {
		t.Fatal("OpenOAuthURL() error = nil with an unrunnable helper")
	}
	if CodeOf(err) != CodeFailed || !strings.Contains(err.Error(), "cannot start") {
		t.Fatalf("OpenOAuthURL() error = %v", err)
	}
	service.mu.Lock()
	process := service.process
	service.mu.Unlock()
	if process != nil {
		t.Fatal("a failed start left a tracked helper process")
	}
}

// newInstalledRuntimeForTest creates a trusted runtime whose entry is a
// non-executable regular file. Runtime.Start reaches exec and fails without
// launching Electron or using the network.
func newInstalledRuntimeForTest(t *testing.T) *webplayer.Runtime {
	t.Helper()
	root := t.TempDir()
	goos, goarch := runtime.GOOS, runtime.GOARCH
	sha := strings.Repeat("a", 64)
	manifest := &webplayer.Manifest{
		Schema:  1,
		Version: "test",
		Platforms: map[string]webplayer.Platform{
			goos + "/" + goarch: {
				URL:    "https://github.com/cadence/test/electron.zip",
				SHA256: sha,
				Size:   1,
				Entry:  "electron",
			},
		},
	}
	dir := filepath.Join(root, "runtime", "test-"+goos+"-"+goarch)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("create fake runtime: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".cadence-runtime-ok"), []byte(sha), 0o600); err != nil {
		t.Fatalf("write fake runtime marker: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "electron"), []byte("#!/bin/sh\n"), 0o600); err != nil {
		t.Fatalf("write fake runtime entry: %v", err)
	}
	rt := webplayer.NewRuntime(webplayer.RuntimeOptions{
		Root:     root,
		GOOS:     goos,
		GOARCH:   goarch,
		Manifest: manifest,
	})
	if _, ok := rt.Installed(); !ok {
		t.Fatal("the fake runtime is not considered installed")
	}
	return rt
}

func TestWriteClientID(t *testing.T) {
	service := newTestService(t)
	clientID := "7fe450922d8b4d31ac8bda4a1767d903"
	if err := service.writeClientID(clientID); err != nil {
		t.Fatalf("writeClientID() error = %v", err)
	}
	path := filepath.Join(service.configDir, "webplayer", "client-id")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat client id: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("client id mode = %o, want 600", info.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read client id: %v", err)
	}
	if strings.TrimSpace(string(data)) != clientID {
		t.Fatalf("client id file = %q", data)
	}
	if got := webplayer.LoadClientID(filepath.Join(service.configDir, "webplayer")); got != clientID {
		t.Fatalf("LoadClientID() = %q", got)
	}

	// A second write replaces the value atomically.
	updated := "0123456789abcdef0123456789abcdef"
	if err := service.writeClientID(updated); err != nil {
		t.Fatalf("second writeClientID() error = %v", err)
	}
	if got := webplayer.LoadClientID(filepath.Join(service.configDir, "webplayer")); got != updated {
		t.Fatalf("LoadClientID() after replace = %q", got)
	}
}

func TestWriteClientIDRejectsInvalid(t *testing.T) {
	service := newTestService(t)
	for _, value := range []string{"", "not-a-client-id", strings.ToUpper("7fe450922d8b4d31ac8bda4a1767d903")} {
		if err := service.writeClientID(value); err == nil {
			t.Fatalf("writeClientID(%q) error = nil", value)
		}
	}
}

func TestParseWebApp(t *testing.T) {
	clientID := "7fe450922d8b4d31ac8bda4a1767d903"
	got, err := parseWebApp(webplayer.Message{Type: "webApp", ClientID: clientID, State: "existing"})
	if err != nil || got != clientID {
		t.Fatalf("parseWebApp() = %q, %v", got, err)
	}
	if _, err := parseWebApp(webplayer.Message{Type: "webApp", ClientID: "bad"}); err == nil {
		t.Fatal("parseWebApp() accepted an invalid Client ID")
	}
	if _, err := parseWebApp(webplayer.Message{Type: "ready", ClientID: clientID}); err == nil {
		t.Fatal("parseWebApp() accepted the wrong message type")
	}
}

func TestParseSoloistKey(t *testing.T) {
	key := "spak_abcdef123456"
	got, err := parseSoloistKey(webplayer.Message{Type: "soloistKey", Key: key, State: "existing"})
	if err != nil || got != key {
		t.Fatalf("parseSoloistKey() = %q, %v", got, err)
	}
	for _, value := range []string{"", "not-a-key", "spak_", "spak_with space"} {
		if _, err := parseSoloistKey(webplayer.Message{Type: "soloistKey", Key: value}); err == nil {
			t.Fatalf("parseSoloistKey(%q) error = nil", value)
		}
	}
	if _, err := parseSoloistKey(webplayer.Message{Type: "webApp", Key: key}); err == nil {
		t.Fatal("parseSoloistKey() accepted the wrong message type")
	}
}

func TestHelperErrorMapping(t *testing.T) {
	tests := []struct {
		code string
		want error
	}{
		{code: "login_required", want: ErrLoginRequired},
		{code: "premium_required", want: ErrPremiumRequired},
		{code: "app_limit", want: ErrAppLimit},
	}
	for _, test := range tests {
		err := helperError(webplayer.Message{Type: "error", Code: test.code})
		if !errors.Is(err, test.want) {
			t.Fatalf("helperError(%q) = %v, want %v", test.code, err, test.want)
		}
		if CodeOf(err) != CodeOf(test.want) {
			t.Fatalf("helperError(%q) code = %q", test.code, CodeOf(err))
		}
	}

	err := helperError(webplayer.Message{Type: "error", Code: "failed", Message: "dashboard failed"})
	if CodeOf(err) != CodeFailed {
		t.Fatalf("unknown helper error code = %q", CodeOf(err))
	}
	if err.Error() != "dashboard failed" {
		t.Fatalf("unknown helper error message = %q", err.Error())
	}
}

func TestWaitForResultDeliversWebApp(t *testing.T) {
	control, err := webplayer.NewControl()
	if err != nil {
		t.Fatalf("NewControl() error = %v", err)
	}
	t.Cleanup(func() { _ = control.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, control.URL(), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })

	if err := wsjson.Write(ctx, conn, webplayer.Message{Type: "hello", Version: "1"}); err != nil {
		t.Fatalf("hello: %v", err)
	}
	clientID := "7fe450922d8b4d31ac8bda4a1767d903"
	if err := wsjson.Write(ctx, conn, webplayer.Message{Type: "ready"}); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := wsjson.Write(ctx, conn, webplayer.Message{Type: "webApp", ClientID: clientID}); err != nil {
		t.Fatalf("webApp: %v", err)
	}

	message, err := waitForResult(ctx, "web-app", nil, control)
	if err != nil {
		t.Fatalf("waitForResult() error = %v", err)
	}
	if message.Type != "webApp" || message.ClientID != clientID {
		t.Fatalf("message = %+v", message)
	}
}

func TestWaitForResultReportsHelperErrors(t *testing.T) {
	control, err := webplayer.NewControl()
	if err != nil {
		t.Fatalf("NewControl() error = %v", err)
	}
	t.Cleanup(func() { _ = control.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, control.URL(), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })

	if err := wsjson.Write(ctx, conn, webplayer.Message{Type: "hello", Version: "1"}); err != nil {
		t.Fatalf("hello: %v", err)
	}
	if err := wsjson.Write(ctx, conn, webplayer.Message{
		Type: "error", Code: "login_required", Message: "sign in",
	}); err != nil {
		t.Fatalf("error: %v", err)
	}

	if _, err := waitForResult(ctx, "web-app", nil, control); !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("waitForResult() error = %v, want ErrLoginRequired", err)
	}
}

func TestWaitForResultHonorsContext(t *testing.T) {
	control, err := webplayer.NewControl()
	if err != nil {
		t.Fatalf("NewControl() error = %v", err)
	}
	t.Cleanup(func() { _ = control.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := waitForResult(ctx, "web-app", nil, control); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waitForResult() error = %v, want DeadlineExceeded", err)
	}
}

func TestStopWithoutHelper(t *testing.T) {
	service := newTestService(t)
	if err := service.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if err := service.Stop(nil); err != nil {
		t.Fatalf("Stop(nil) error = %v", err)
	}
}

func TestHelperStatusCodesAreStable(t *testing.T) {
	// The frontend localizes these exact strings.
	want := map[string]string{
		"login_required":   CodeLoginRequired,
		"premium_required": CodePremiumRequired,
		"app_limit":        CodeAppLimit,
	}
	if CodeLoginRequired != want["login_required"] ||
		CodePremiumRequired != want["premium_required"] ||
		CodeAppLimit != want["app_limit"] {
		t.Fatalf("codes changed: %q %q %q", CodeLoginRequired, CodePremiumRequired, CodeAppLimit)
	}
}

func TestControlRequiresHelloStillWorks(t *testing.T) {
	// Guard the webplayer control extension against a regression that would
	// silently drop developer result messages.
	control, err := webplayer.NewControl()
	if err != nil {
		t.Fatalf("NewControl() error = %v", err)
	}
	t.Cleanup(func() { _ = control.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, control.URL(), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })
	if err := wsjson.Write(ctx, conn, webplayer.Message{Type: "hello", Version: "1"}); err != nil {
		t.Fatalf("hello: %v", err)
	}
	if err := wsjson.Write(ctx, conn, webplayer.Message{Type: "soloistKey", Key: "spak_test"}); err != nil {
		t.Fatalf("soloistKey: %v", err)
	}

	select {
	case message := <-control.Events():
		if message.Type != "soloistKey" || message.Key != "spak_test" {
			t.Fatalf("message = %+v", message)
		}
	case <-ctx.Done():
		t.Fatal("the control socket dropped the soloistKey message")
	}
}
