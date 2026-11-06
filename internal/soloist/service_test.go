package soloist

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func serviceOptions(t *testing.T) Options {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("CADENCE_SOLOIST", "")
	t.Setenv("PATH", filepath.Join(dir, "empty-path"))
	return Options{
		ProgramsDir: filepath.Join(dir, "programs"),
		DataDir:     filepath.Join(dir, "data"),
		CacheDir:    filepath.Join(dir, "cache"),
		LogPath:     filepath.Join(dir, "logs", "soloist.log"),
		APIKeyPath:  filepath.Join(dir, "config", "soloist.key"),
		DeviceName:  "Cadence",
		Now:         time.Now,
	}
}

func TestServiceNotInstalled(t *testing.T) {
	opts := serviceOptions(t)
	var emissions atomic.Int32
	opts.Emit = func(State) { emissions.Add(1) }
	service := NewService(opts)
	if state := service.State(); state.Installed {
		t.Fatalf("state = %+v", state)
	}
	err := service.Start(t.Context())
	if !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("error = %v, want ErrNotInstalled", err)
	}
	state := service.State()
	if state.ErrorCode != CodeNotInstalled {
		t.Fatalf("state error code = %q", state.ErrorCode)
	}
	if emissions.Load() == 0 {
		t.Fatal("emitter was not called")
	}
	if err := service.Stop(t.Context()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if got := service.State().ErrorCode; got != "" {
		t.Fatalf("error code after a clean stop = %q", got)
	}
}

func TestServiceMissingAPIKey(t *testing.T) {
	opts := serviceOptions(t)
	port := 12345
	binary := installFakeBinary(t, opts.ProgramsDir, opts.DataDir, port, time.Now().Add(-time.Hour).Unix(), "secret")
	service := NewService(opts)
	if got := service.Binary(); got == nil || got.Path != binary.Path {
		t.Fatalf("binary = %+v", got)
	}
	err := service.Start(t.Context())
	if !errors.Is(err, ErrAPIKeyMissing) {
		t.Fatalf("error = %v, want ErrAPIKeyMissing", err)
	}
	state := service.State()
	if state.ErrorCode != CodeAPIKeyMissing {
		t.Fatalf("state error code = %q", state.ErrorCode)
	}
}

func TestServiceExpiredBuild(t *testing.T) {
	opts := serviceOptions(t)
	built := time.Now().Add(-120 * 24 * time.Hour).Unix()
	installFakeBinary(t, opts.ProgramsDir, opts.DataDir, 12345, built, "secret")
	service := NewService(opts)
	state := service.State()
	if !state.Expired {
		t.Fatalf("state = %+v, want expired", state)
	}
	err := service.Start(t.Context())
	if !errors.Is(err, ErrExpired) {
		t.Fatalf("error = %v, want ErrExpired", err)
	}
	if got := service.State().ErrorCode; got != CodeBuildExpired {
		t.Fatalf("state error code = %q", got)
	}
}

func TestServiceInstall(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the fake Soloist is a shell script")
	}
	opts := serviceOptions(t)
	built := int64(1792756912)
	archive := tarGzArchive(t, []archiveMember{{Name: "soloist", Body: versionScript("1.3.10.12", built), Mode: 0o755}})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	opts.Now = func() time.Time { return time.Unix(built+86400, 0) }
	installer := NewInstaller(InstallerOptions{
		ProgramsDir:  opts.ProgramsDir,
		HTTPClient:   server.Client(),
		AllowAnyHost: true,
		GOOS:         "linux",
		GOARCH:       "amd64",
		Now:          opts.Now,
	})
	installer.archiveBaseURL = server.URL + "/"
	opts.Installer = installer
	var emissions atomic.Int32
	opts.Emit = func(State) { emissions.Add(1) }

	service := NewService(opts)
	binary, err := service.Install(t.Context(), nil)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if binary.Version != "1.3.10.12" {
		t.Fatalf("binary = %+v", binary)
	}
	state := service.State()
	if !state.Installed || state.Version != "1.3.10.12" || state.BinaryPath != binary.Path {
		t.Fatalf("state = %+v", state)
	}
	if emissions.Load() == 0 {
		t.Fatal("emitter was not called")
	}
}

func TestServiceStartStateMergeAndEmitter(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the fake Soloist is a shell script")
	}
	opts := serviceOptions(t)
	server := newFakeSoloist(t, true)
	built := time.Now().Add(-time.Hour).Unix()
	installFakeBinary(t, opts.ProgramsDir, opts.DataDir, server.port, built, "secret")
	if err := os.MkdirAll(filepath.Join(opts.DataDir, "settings", "Users", "alice-user"), 0o700); err != nil {
		t.Fatalf("mkdir account: %v", err)
	}

	var emissions atomic.Int32
	opts.Emit = func(State) { emissions.Add(1) }
	service := NewService(opts)
	if err := service.SaveAPIKey("secret-key"); err != nil {
		t.Fatalf("save key: %v", err)
	}
	if !service.HasAPIKey() {
		t.Fatal("HasAPIKey = false after SaveAPIKey")
	}

	if err := service.Start(t.Context()); err != nil {
		t.Fatalf("start: %v", err)
	}
	state := service.State()
	if !state.Running || !state.LoggedIn || state.NeedsPairing {
		t.Fatalf("state = %+v", state)
	}
	if !state.Active || state.DeviceName != "Cadence" {
		t.Fatalf("state = %+v", state)
	}
	if strings.Join(state.Accounts, ",") != "alice" {
		t.Fatalf("accounts = %v", state.Accounts)
	}
	if service.Client() == nil {
		t.Fatal("client is nil while running")
	}
	if err := service.Stop(t.Context()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if service.State().Running {
		t.Fatal("state still reports running")
	}
	if service.Client() != nil {
		t.Fatal("client is not nil after Stop")
	}
	if emissions.Load() == 0 {
		t.Fatal("emitter was not called")
	}

	if err := service.ClearAPIKey(); err != nil {
		t.Fatalf("clear key: %v", err)
	}
	if service.HasAPIKey() {
		t.Fatal("HasAPIKey = true after ClearAPIKey")
	}
	// A restart must not reuse the key the supervisor was started with.
	if err := service.Restart(t.Context()); !errors.Is(err, ErrAPIKeyMissing) {
		t.Fatalf("restart after ClearAPIKey = %v, want ErrAPIKeyMissing", err)
	}
}

func TestServiceStateNeverContainsAPIKey(t *testing.T) {
	opts := serviceOptions(t)
	const secret = "top-secret-key"
	service := NewService(opts)
	if err := service.SaveAPIKey(secret); err != nil {
		t.Fatalf("save key: %v", err)
	}
	state := service.State()
	if strings.Contains(state.Error, secret) {
		t.Fatalf("state error leaks the key: %q", state.Error)
	}
}

func TestStateDoesNotBlockOnTheVersionProbe(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the fake Soloist is a shell script")
	}
	opts := serviceOptions(t)
	built := time.Now().Add(-time.Hour).Unix()
	installFakeBinary(t, opts.ProgramsDir, opts.DataDir, 12345, built, "secret")
	service := NewService(opts)
	path := service.Binary().Path
	if path == "" {
		t.Fatal("fake binary was not discovered")
	}

	started := make(chan struct{})
	release := make(chan struct{})
	service.mu.Lock()
	service.inspect = func(ctx context.Context, path string) (*Binary, error) {
		close(started)
		<-release
		return &Binary{Path: path, Version: "1.0.0", BuiltAt: time.Unix(built, 0)}, nil
	}
	service.mu.Unlock()

	future := time.Now().Add(time.Minute)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	done := make(chan struct{})
	go func() {
		_ = service.State()
		close(done)
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("the version probe did not start")
	}

	saved := make(chan error, 1)
	go func() { saved <- service.SaveAPIKey("key") }()
	select {
	case err := <-saved:
		if err != nil {
			t.Fatalf("save key: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("SaveAPIKey blocked on the version probe")
	}

	close(release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("State did not finish")
	}
}

func TestNewServiceDoesNotProbeTheVersion(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the fake Soloist is a shell script")
	}
	// serviceOptions empties PATH, so use the real sleep binary by absolute
	// path; otherwise the fake probe would return instantly.
	sleepPath, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("sleep is not available")
	}
	opts := serviceOptions(t)
	built := time.Now().Add(-time.Hour).Unix()
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "--version" ]; then
  %s 1
fi
echo "soloist 1.0.0 build %d (20260101)"
`, sleepPath, built)
	writeScript(t, filepath.Join(opts.ProgramsDir, fmt.Sprintf("%d", built)), "soloist", script)

	started := time.Now()
	service := NewService(opts)
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("NewService took %s; construction must not run --version", elapsed)
	}

	probeStart := time.Now()
	state := service.State()
	if elapsed := time.Since(probeStart); elapsed < 500*time.Millisecond {
		t.Fatalf("State took %s; the slow --version probe seems to have been skipped", elapsed)
	}
	if !state.Installed || state.Version != "1.0.0" {
		t.Fatalf("state = %+v, want the slow binary discovered on first read", state)
	}
}

func TestStateReportsTheServiceBinaryOverTheSupervisor(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the fake Soloist is a shell script")
	}
	opts := serviceOptions(t)
	now := time.Now()
	oldBuilt := now.Add(-48 * time.Hour).Unix()
	newBuilt := now.Add(-time.Hour).Unix()
	oldBinary := installFakeBinary(t, opts.ProgramsDir, opts.DataDir, 12345, oldBuilt, "secret")
	newBinary := installFakeBinary(t, opts.ProgramsDir, opts.DataDir, 12345, newBuilt, "secret")

	service := NewService(opts)
	service.mu.Lock()
	service.cacheBinaryLocked(newBinary)
	service.discovered = true
	service.supervisor = NewSupervisor(SupervisorOptions{Binary: oldBinary})
	service.mu.Unlock()

	state := service.State()
	if state.BinaryPath != newBinary.Path || !state.BuiltAt.Equal(newBinary.BuiltAt) {
		t.Fatalf("state = %+v, want the service-level binary %+v", state, newBinary)
	}
}

func TestInstallClearsAStoppedSupervisor(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the fake Soloist is a shell script")
	}
	opts := serviceOptions(t)
	built := int64(1792756912)
	archive := tarGzArchive(t, []archiveMember{{Name: "soloist", Body: versionScript("1.3.10.12", built), Mode: 0o755}})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	opts.Now = func() time.Time { return time.Unix(built+86400, 0) }
	installer := NewInstaller(InstallerOptions{
		ProgramsDir:  opts.ProgramsDir,
		HTTPClient:   server.Client(),
		AllowAnyHost: true,
		GOOS:         "linux",
		GOARCH:       "amd64",
		Now:          opts.Now,
	})
	installer.archiveBaseURL = server.URL + "/"
	opts.Installer = installer

	service := NewService(opts)
	oldBinary := installFakeBinary(t, opts.ProgramsDir, opts.DataDir, 12345, built-3600, "secret")
	service.mu.Lock()
	service.supervisor = NewSupervisor(SupervisorOptions{Binary: oldBinary, APIKey: "secret", DataDir: opts.DataDir})
	service.mu.Unlock()

	if _, err := service.Install(t.Context(), nil); err != nil {
		t.Fatalf("install: %v", err)
	}

	service.mu.Lock()
	supervisor := service.supervisor
	service.mu.Unlock()
	if supervisor != nil {
		t.Fatal("a stopped supervisor still points at the old build after Install")
	}
	state := service.State()
	if !state.Installed || state.Version != "1.3.10.12" {
		t.Fatalf("state = %+v, want the installed build", state)
	}
}
