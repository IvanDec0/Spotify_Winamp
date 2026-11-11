//go:build linux

package webplayer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestProcessUsesAppDirAndOmitsEmptyPlayerURL verifies the developer helper
// path: a custom app directory and no --cadence-url argument.
func TestProcessUsesAppDirAndOmitsEmptyPlayerURL(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "developer-helper")
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		t.Fatalf("mkdir app: %v", err)
	}
	scriptPath := filepath.Join(root, "helper-bin")
	script := "#!/bin/sh\nprintf 'args: %s\\n' \"$*\"\nsleep 30\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	runtime := NewRuntime(RuntimeOptions{Root: root, GOOS: "linux", GOARCH: "amd64"})
	process, err := runtime.Start(context.Background(), Install{
		Version:    "test",
		Dir:        root,
		Executable: scriptPath,
		HelperDir:  filepath.Join(root, "embedded-helper"),
	}, StartOptions{
		ControlURL: "ws://127.0.0.1:4321/control?nonce=control-nonce",
		AppDir:     appDir,
		LogPath:    filepath.Join(root, "logs", "developer.log"),
		ExtraArgs:  []string{"--cadence-action=web-app"},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = process.Stop(ctx)
	})

	deadline := time.Now().Add(5 * time.Second)
	var lines []string
	for time.Now().Before(deadline) {
		lines = process.Tail()
		if len(lines) >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(lines) < 1 {
		t.Fatalf("tail did not capture the helper output: %#v", lines)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, appDir) {
		t.Fatalf("args do not start with the app dir: %s", joined)
	}
	if strings.Contains(joined, filepath.Join(root, "embedded-helper")) {
		t.Fatalf("args use the embedded helper dir instead of AppDir: %s", joined)
	}
	if strings.Contains(joined, "--cadence-url=") {
		t.Fatalf("args contain --cadence-url without a player URL: %s", joined)
	}
	if !strings.Contains(joined, "--cadence-action=web-app") {
		t.Fatalf("extra args were not appended: %s", joined)
	}
}

// TestProcessLifecycle exercises Start, Tail, Stop and log redaction with a
// plain shell process. No Electron or network is involved.
func TestProcessLifecycle(t *testing.T) {
	root := t.TempDir()
	helperDir := filepath.Join(root, "helper")
	if err := os.MkdirAll(helperDir, 0o700); err != nil {
		t.Fatalf("mkdir helper: %v", err)
	}
	secret := "BQ" + strings.Repeat("x", 40)
	scriptPath := filepath.Join(root, "helper-bin")
	script := fmt.Sprintf("#!/bin/sh\nprintf 'args: %%s\\n' \"$*\"\nprintf 'access_token=%s\\n'\nsleep 30\n", secret)
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	runtime := NewRuntime(RuntimeOptions{Root: root, GOOS: "linux", GOARCH: "amd64"})
	controlURL := "ws://127.0.0.1:4321/control?nonce=control-nonce"
	playerURL := "http://127.0.0.1:4322/player?nonce=player-nonce"
	process, err := runtime.Start(context.Background(), Install{
		Version:    "test",
		Dir:        root,
		Executable: scriptPath,
		HelperDir:  helperDir,
	}, StartOptions{ControlURL: controlURL, PlayerURL: playerURL})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	var lines []string
	for time.Now().Before(deadline) {
		lines = process.Tail()
		if len(lines) >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(lines) < 2 {
		t.Fatalf("tail did not capture the helper output: %#v", lines)
	}
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"--cadence-control=" + controlURL, "--cadence-url=" + playerURL, "--user-data-dir=" + filepath.Join(root, "profile")} {
		if !strings.Contains(joined, want) {
			t.Fatalf("tail does not contain %q: %s", want, joined)
		}
	}
	if strings.Contains(joined, secret) {
		t.Fatalf("tail contains the unredacted token: %s", joined)
	}
	if !strings.Contains(joined, "access_token=[redacted]") {
		t.Fatalf("tail does not show the redacted token: %s", joined)
	}

	output, err := os.ReadFile(filepath.Join(root, "logs", "helper.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if strings.Contains(string(output), secret) {
		t.Fatalf("log contains the unredacted token: %s", output)
	}
	info, err := os.Stat(filepath.Join(root, "logs", "helper.log"))
	if err != nil {
		t.Fatalf("stat log: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("log mode = %o, want 600", info.Mode().Perm())
	}
	profile, err := os.Stat(filepath.Join(root, "profile"))
	if err != nil {
		t.Fatalf("stat profile: %v", err)
	}
	if profile.Mode().Perm() != 0o700 {
		t.Fatalf("profile mode = %o, want 700", profile.Mode().Perm())
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := process.Stop(stopCtx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if err := process.Stop(stopCtx); err != nil {
		t.Fatalf("second stop: %v", err)
	}
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer waitCancel()
	waited := make(chan error, 1)
	go func() { waited <- process.Wait() }()
	select {
	case <-waited:
	case <-waitCtx.Done():
		t.Fatal("Wait did not return after Stop")
	}
}
