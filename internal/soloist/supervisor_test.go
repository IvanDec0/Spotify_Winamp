package soloist

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type fakeSoloist struct {
	server *httptest.Server
	port   int
	mu     sync.Mutex
	hits   int
}

func newFakeSoloist(t *testing.T, loggedIn bool) *fakeSoloist {
	t.Helper()
	fake := &fakeSoloist{}
	fake.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		fake.mu.Lock()
		fake.hits++
		fake.mu.Unlock()
		ctx := context.Background()
		auth := map[string]any{
			"type":        "auth_state",
			"logged_in":   loggedIn,
			"is_active":   loggedIn,
			"device_name": "Cadence",
		}
		payload, _ := json.Marshal(auth)
		if err := conn.Write(ctx, websocket.MessageText, payload); err != nil {
			return
		}
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			var command Command
			if err := json.Unmarshal(data, &command); err != nil {
				continue
			}
			result, _ := json.Marshal(map[string]string{"type": "command_result", "command": command.Command})
			if err := conn.Write(ctx, websocket.MessageText, result); err != nil {
				return
			}
		}
	}))
	t.Cleanup(fake.server.Close)
	fake.port = fake.server.Listener.Addr().(*net.TCPAddr).Port
	return fake
}

func fakeSoloistScript(dataDir string, port int, built int64, secret string) string {
	return fmt.Sprintf(`#!/bin/sh
if [ "$1" = "--version" ]; then
  echo "soloist 1.0.0 build %d (20260101)"
  exit 0
fi
echo "soloist starting with %s"
echo %d > "%s"
trap 'exit 0' TERM INT
while :; do sleep 0.05; done
`, built, secret, port, filepath.Join(dataDir, "ws.port"))
}

func installFakeBinary(t *testing.T, programsDir, dataDir string, port int, built int64, secret string) *Binary {
	t.Helper()
	path := filepath.Join(programsDir, fmt.Sprintf("%d", built), "soloist")
	writeScript(t, filepath.Dir(path), filepath.Base(path), fakeSoloistScript(dataDir, port, built, secret))
	binary, err := InspectBinary(t.Context(), path)
	if err != nil {
		t.Fatalf("inspect fake binary: %v", err)
	}
	return binary
}

func TestBuildArgs(t *testing.T) {
	volume := 55
	tests := []struct {
		name        string
		cacheSizeMB int
		volume      *int
		want        []string
	}{
		{
			name: "cache omitted",
			want: []string{"-n", "Cadence", "-k", "key", "-D", "/data", "-C", "/cache", "-w", "127.0.0.1:0"},
		},
		{
			name:        "cache included",
			cacheSizeMB: 100,
			want:        []string{"-n", "Cadence", "-k", "key", "-D", "/data", "-C", "/cache", "-w", "127.0.0.1:0", "-z", "100"},
		},
		{
			name:        "cache and volume",
			cacheSizeMB: 256,
			volume:      &volume,
			want:        []string{"-n", "Cadence", "-k", "key", "-D", "/data", "-C", "/cache", "-w", "127.0.0.1:0", "-z", "256", "-i", "55"},
		},
		{
			name:        "small cache omitted",
			cacheSizeMB: 99,
			want:        []string{"-n", "Cadence", "-k", "key", "-D", "/data", "-C", "/cache", "-w", "127.0.0.1:0"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := buildArgs("Cadence", "key", "/data", "/cache", test.cacheSizeMB, test.volume)
			if strings.Join(got, "\x00") != strings.Join(test.want, "\x00") {
				t.Fatalf("args = %v, want %v", got, test.want)
			}
		})
	}
}

func TestCleanupStaleRemovesDeadPidFiles(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("stale pid cleanup uses /proc")
	}
	dataDir := t.TempDir()
	for _, name := range []string{"ws.addr", "ws.port", "soloist.pid"} {
		if err := os.WriteFile(filepath.Join(dataDir, name), []byte("999999999"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	cleanupStale(dataDir, "/usr/bin/soloist")
	for _, name := range []string{"ws.addr", "ws.port", "soloist.pid"} {
		if _, err := os.Stat(filepath.Join(dataDir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s still exists", name)
		}
	}
}

func TestCleanupStaleLeavesUnrelatedProcess(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("stale pid cleanup uses /proc")
	}
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()
	dataDir := t.TempDir()
	pidText := fmt.Sprintf("%d", cmd.Process.Pid)
	if err := os.WriteFile(filepath.Join(dataDir, "soloist.pid"), []byte(pidText), 0o600); err != nil {
		t.Fatalf("write pid: %v", err)
	}
	_ = os.WriteFile(filepath.Join(dataDir, "ws.port"), []byte("1234"), 0o600)
	cleanupStale(dataDir, "/usr/bin/soloist")
	if err := syscall.Kill(cmd.Process.Pid, 0); err != nil {
		t.Fatalf("unrelated process was killed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "soloist.pid")); err != nil {
		t.Fatalf("pid file removed for an unrelated process: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "ws.port")); !os.IsNotExist(err) {
		t.Fatal("runtime files were not removed")
	}
}

func TestStartExitCodeTenIsExpired(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the fake Soloist is a shell script")
	}
	dir := t.TempDir()
	path := writeScript(t, dir, "soloist", "#!/bin/sh\nexit 10\n")
	binary := &Binary{Path: path, Version: "1.0.0", BuiltAt: time.Now().Add(-time.Hour)}
	supervisor := NewSupervisor(SupervisorOptions{
		Binary:       binary,
		APIKey:       "secret",
		DataDir:      t.TempDir(),
		CacheDir:     t.TempDir(),
		LogPath:      filepath.Join(t.TempDir(), "soloist.log"),
		StartTimeout: 2 * time.Second,
	})
	err := supervisor.Start(t.Context())
	if CodeOf(err) != CodeBuildExpired {
		t.Fatalf("error = %v, code = %q, want %q", err, CodeOf(err), CodeBuildExpired)
	}
	state := supervisor.State()
	if state.Running {
		t.Fatal("state still reports running")
	}
	if state.ErrorCode != CodeBuildExpired {
		t.Fatalf("state error code = %q", state.ErrorCode)
	}
}

func TestStartRequiresBinaryAndKey(t *testing.T) {
	dataDir := t.TempDir()
	supervisor := NewSupervisor(SupervisorOptions{DataDir: dataDir})
	if err := supervisor.Start(t.Context()); !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("error = %v, want ErrNotInstalled", err)
	}
	supervisor = NewSupervisor(SupervisorOptions{
		Binary:  &Binary{Path: "/nonexistent/soloist", Version: "1", BuiltAt: time.Now()},
		DataDir: dataDir,
	})
	if err := supervisor.Start(t.Context()); !errors.Is(err, ErrAPIKeyMissing) {
		t.Fatalf("error = %v, want ErrAPIKeyMissing", err)
	}
}

func TestStartFullLifecycle(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the fake Soloist is a shell script")
	}
	const secret = "test-key-secret"
	server := newFakeSoloist(t, true)
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "settings", "Users", "alice-user"), 0o700); err != nil {
		t.Fatalf("mkdir account: %v", err)
	}
	programs := t.TempDir()
	binary := installFakeBinary(t, programs, dataDir, server.port, time.Now().Add(-time.Hour).Unix(), secret)
	logPath := filepath.Join(t.TempDir(), "soloist.log")

	var mu sync.Mutex
	var emissions []State
	supervisor := NewSupervisor(SupervisorOptions{
		Binary:       binary,
		APIKey:       secret,
		DataDir:      dataDir,
		CacheDir:     t.TempDir(),
		LogPath:      logPath,
		StartTimeout: 5 * time.Second,
		LoginTimeout: 2 * time.Second,
		Emit: func(state State) {
			mu.Lock()
			emissions = append(emissions, state)
			mu.Unlock()
		},
	})

	if err := supervisor.Start(t.Context()); err != nil {
		t.Fatalf("start: %v", err)
	}
	state := supervisor.State()
	if !state.Running || !state.LoggedIn || state.NeedsPairing {
		t.Fatalf("state = %+v", state)
	}
	if !state.Active || state.DeviceName != "Cadence" {
		t.Fatalf("state = %+v", state)
	}
	if len(state.Accounts) != 1 || state.Accounts[0] != "alice" {
		t.Fatalf("accounts = %v", state.Accounts)
	}
	if supervisor.Client() == nil {
		t.Fatal("client is nil while running")
	}
	if err := supervisor.Stop(t.Context()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if supervisor.State().Running {
		t.Fatal("state still reports running after Stop")
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	logText := string(logData)
	if strings.Contains(logText, secret) {
		t.Fatalf("log leaks the API key: %q", logText)
	}
	if !strings.Contains(logText, "[redacted]") {
		t.Fatalf("log was not redacted: %q", logText)
	}
	if supervisor.tail.contains(secret) {
		t.Fatal("diagnostic tail leaks the API key")
	}
	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("stat log: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("log mode = %o, want 600", info.Mode().Perm())
	}
	mu.Lock()
	count := len(emissions)
	mu.Unlock()
	if count == 0 {
		t.Fatal("emitter was never called")
	}
	if !strings.Contains(server.hitsString(), "1") {
		t.Fatalf("server connections = %s", server.hitsString())
	}
}

func (f *fakeSoloist) hitsString() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return fmt.Sprintf("%d", f.hits)
}

func TestStartPairingRequiredKeepsRunning(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the fake Soloist is a shell script")
	}
	server := newFakeSoloist(t, false)
	dataDir := t.TempDir()
	programs := t.TempDir()
	binary := installFakeBinary(t, programs, dataDir, server.port, time.Now().Add(-time.Hour).Unix(), "secret")
	supervisor := NewSupervisor(SupervisorOptions{
		Binary:       binary,
		APIKey:       "secret",
		DataDir:      dataDir,
		CacheDir:     t.TempDir(),
		LogPath:      filepath.Join(t.TempDir(), "soloist.log"),
		StartTimeout: 5 * time.Second,
		LoginTimeout: 200 * time.Millisecond,
	})
	err := supervisor.Start(t.Context())
	if !errors.Is(err, ErrPairingRequired) {
		t.Fatalf("error = %v, want ErrPairingRequired", err)
	}
	state := supervisor.State()
	if !state.Running || state.LoggedIn || !state.NeedsPairing {
		t.Fatalf("state = %+v", state)
	}
	if state.ErrorCode != CodePairingRequired {
		t.Fatalf("error code = %q", state.ErrorCode)
	}
	if err := supervisor.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
}

func TestAccounts(t *testing.T) {
	dataDir := t.TempDir()
	users := filepath.Join(dataDir, "settings", "Users")
	for _, name := range []string{"alice-user", "bob-user", "carol"} {
		if err := os.MkdirAll(filepath.Join(users, name), 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(users, "dave-user"), []byte("not a dir"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	accounts := Accounts(dataDir)
	if strings.Join(accounts, ",") != "alice,bob" {
		t.Fatalf("accounts = %v", accounts)
	}
	if got := Accounts(filepath.Join(dataDir, "missing")); got != nil {
		t.Fatalf("missing accounts = %v", got)
	}
}

func TestRedactingWriterHandlesSplitSecret(t *testing.T) {
	tail := newTailBuffer(tailLines)
	var output strings.Builder
	writer := newRedactingWriter(&output, "supersecret", tail)
	_, _ = writer.Write([]byte("starting superse"))
	_, _ = writer.Write([]byte("cret now\n"))
	if err := writer.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if strings.Contains(output.String(), "supersecret") {
		t.Fatalf("output leaks the secret: %q", output.String())
	}
	if !strings.Contains(output.String(), "[redacted]") {
		t.Fatalf("output was not redacted: %q", output.String())
	}
	if tail.contains("supersecret") {
		t.Fatal("tail leaks the secret")
	}
}

func TestRedactingWriterProtectsFlushBoundary(t *testing.T) {
	const secret = "supersecretvalue"
	tail := newTailBuffer(tailLines)
	var output strings.Builder
	writer := newRedactingWriter(&output, secret, tail)

	// A long unterminated line is flushed in chunks. The first chunk ends in
	// the middle of the secret, so a naive flush would log half of it.
	prefix := strings.Repeat("a", pendingLogLimit)
	halves := len(secret) / 2
	_, _ = writer.Write([]byte(prefix + secret[:halves]))
	_, _ = writer.Write([]byte(secret[halves:] + "done\n"))
	if err := writer.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if strings.Contains(output.String(), secret[:halves]) {
		t.Fatal("output leaks a secret fragment across the flush boundary")
	}
	if !strings.Contains(output.String(), "[redacted]") {
		t.Fatal("output was not redacted")
	}
	if tail.contains(secret) || tail.contains(secret[:halves]) {
		t.Fatal("tail leaks the secret")
	}
}
