package soloist

import (
	"bytes"
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
	"strconv"
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
	conns  map[*websocket.Conn]struct{}
}

func newFakeSoloist(t *testing.T, loggedIn bool) *fakeSoloist {
	t.Helper()
	fake := &fakeSoloist{conns: make(map[*websocket.Conn]struct{})}
	fake.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		fake.mu.Lock()
		fake.hits++
		fake.conns[conn] = struct{}{}
		fake.mu.Unlock()
		defer func() {
			fake.mu.Lock()
			delete(fake.conns, conn)
			fake.mu.Unlock()
		}()
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

// sendAuth pushes an auth_state event to every connected client.
func (f *fakeSoloist) sendAuth(loggedIn bool) {
	payload, _ := json.Marshal(map[string]any{
		"type":        "auth_state",
		"logged_in":   loggedIn,
		"is_active":   loggedIn,
		"device_name": "Cadence",
	})
	f.mu.Lock()
	conns := make([]*websocket.Conn, 0, len(f.conns))
	for conn := range f.conns {
		conns = append(conns, conn)
	}
	f.mu.Unlock()
	for _, conn := range conns {
		_ = conn.Write(context.Background(), websocket.MessageText, payload)
	}
}

func fakeSoloistScript(dataDir string, port int, built int64, secret string, zeroConfPort ...int) string {
	zeroConfLine := ""
	if len(zeroConfPort) > 0 && zeroConfPort[0] > 0 {
		zeroConfLine = fmt.Sprintf("echo \"Started ZeroConf service on port %d path /zc\"\n", zeroConfPort[0])
	}
	return fmt.Sprintf(`#!/bin/sh
if [ "$1" = "--version" ]; then
  echo "soloist 1.0.0 build %d (20260101)"
  exit 0
fi
echo "soloist starting with %s"
%s echo %d > "%s"
trap 'exit 0' TERM INT
while :; do sleep 0.05; done
`, built, secret, zeroConfLine, port, filepath.Join(dataDir, "ws.port"))
}

func installFakeBinary(t *testing.T, programsDir, dataDir string, port int, built int64, secret string) *Binary {
	t.Helper()
	return installFakeBinaryWithZeroConf(t, programsDir, dataDir, port, 0, built, secret)
}

func installFakeBinaryWithZeroConf(t *testing.T, programsDir, dataDir string, port, zeroConfPort int, built int64, secret string) *Binary {
	t.Helper()
	path := filepath.Join(programsDir, fmt.Sprintf("%d", built), "soloist")
	writeScript(t, filepath.Dir(path), filepath.Base(path), fakeSoloistScript(dataDir, port, built, secret, zeroConfPort))
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
	if err := supervisor.Stop(t.Context()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if got := supervisor.State().ErrorCode; got != "" {
		t.Fatalf("error code after a clean stop = %q", got)
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

func fakeSoloistPidScript(dataDir, pidFile string, port int, built int64) string {
	return fmt.Sprintf(`#!/bin/sh
if [ "$1" = "--version" ]; then
  echo "soloist 1.0.0 build %d (20260101)"
  exit 0
fi
echo $$ >> "%s"
echo %d > "%s"
trap 'exit 0' TERM INT
while :; do sleep 0.05; done
`, built, pidFile, port, filepath.Join(dataDir, "ws.port"))
}

func readSpawnedPids(t *testing.T, path string) []int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pids: %v", err)
	}
	var pids []int
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil {
			t.Fatalf("pid %q: %v", line, err)
		}
		pids = append(pids, pid)
	}
	return pids
}

func TestConcurrentStartSpawnsOneProcess(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the fake Soloist is a shell script")
	}
	server := newFakeSoloist(t, true)
	dataDir := t.TempDir()
	pidFile := filepath.Join(t.TempDir(), "spawns")
	programs := t.TempDir()
	built := time.Now().Add(-time.Hour).Unix()
	path := writeScript(t, filepath.Join(programs, strconv.FormatInt(built, 10)), "soloist", fakeSoloistPidScript(dataDir, pidFile, server.port, built))
	binary, err := InspectBinary(t.Context(), path)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	supervisor := NewSupervisor(SupervisorOptions{
		Binary:       binary,
		APIKey:       "secret",
		DataDir:      dataDir,
		CacheDir:     t.TempDir(),
		LogPath:      filepath.Join(t.TempDir(), "soloist.log"),
		StartTimeout: 5 * time.Second,
		LoginTimeout: 2 * time.Second,
	})

	start := make(chan struct{})
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for index := range errs {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			errs[index] = supervisor.Start(t.Context())
		}(index)
	}
	close(start)
	wg.Wait()
	for index, err := range errs {
		if err != nil {
			t.Fatalf("start %d: %v", index, err)
		}
	}
	pids := readSpawnedPids(t, pidFile)
	if len(pids) != 1 {
		t.Fatalf("spawns = %d (%v), want exactly one", len(pids), pids)
	}
	if err := supervisor.Stop(t.Context()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if err := syscall.Kill(pids[0], 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("process %d is still alive after Stop: %v", pids[0], err)
	}
}

func TestCleanupStaleIgnoresTailWatchingTheLog(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("stale pid cleanup uses /proc")
	}
	tailPath, err := exec.LookPath("tail")
	if err != nil {
		t.Skip("tail is not available")
	}
	dataDir := t.TempDir()
	logPath := filepath.Join(dataDir, "soloist.log")
	if err := os.WriteFile(logPath, []byte("line\n"), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}
	cmd := exec.Command(tailPath, "-f", logPath)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start tail: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()
	pidPath := filepath.Join(dataDir, "soloist.pid")
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0o600); err != nil {
		t.Fatalf("write pid: %v", err)
	}
	cleanupStale(dataDir, "/usr/bin/soloist")
	if err := syscall.Kill(cmd.Process.Pid, 0); err != nil {
		t.Fatalf("tail watching the log was killed: %v", err)
	}
	if _, err := os.Stat(pidPath); err != nil {
		t.Fatalf("pid file removed for an unrelated process: %v", err)
	}
}

func TestAPIKeyIsTrimmedForTheNextRun(t *testing.T) {
	supervisor := NewSupervisor(SupervisorOptions{APIKey: "  spaced-key  "})
	if got := supervisor.apiKeyValue(); got != "spaced-key" {
		t.Fatalf("key = %q, want trimmed", got)
	}
	supervisor.setAPIKey("  other-key  ")
	if got := supervisor.apiKeyValue(); got != "other-key" {
		t.Fatalf("key = %q, want trimmed", got)
	}
	args := buildArgs("Cadence", supervisor.apiKeyValue(), "/data", "/cache", 0, nil)
	if strings.Join(args, "\x00") != "-n\x00Cadence\x00-k\x00other-key\x00-D\x00/data\x00-C\x00/cache\x00-w\x00127.0.0.1:0" {
		t.Fatalf("args = %v", args)
	}

	opts := serviceOptions(t)
	service := NewService(opts)
	service.mu.Lock()
	service.supervisor = NewSupervisor(SupervisorOptions{APIKey: "old-key"})
	service.mu.Unlock()
	if err := service.SaveAPIKey("  spaced-key  "); err != nil {
		t.Fatalf("save key: %v", err)
	}
	service.mu.Lock()
	got := service.supervisor.apiKeyValue()
	service.mu.Unlock()
	if got != "spaced-key" {
		t.Fatalf("supervisor key = %q, want trimmed", got)
	}
}

func TestDiagnosticTailDoesNotLeakAcrossRuns(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the fake Soloist is a shell script")
	}
	dir := t.TempDir()
	path := writeScript(t, dir, "soloist", "#!/bin/sh\necho 'make sure --api-key is valid' >&2\nexit 1\n")
	binary := &Binary{Path: path, Version: "1.0.0", BuiltAt: time.Now().Add(-time.Hour)}
	supervisor := NewSupervisor(SupervisorOptions{
		Binary:       binary,
		APIKey:       "secret",
		DataDir:      t.TempDir(),
		CacheDir:     t.TempDir(),
		LogPath:      filepath.Join(t.TempDir(), "soloist.log"),
		StartTimeout: time.Second,
	})
	if err := supervisor.Start(t.Context()); CodeOf(err) != CodeAPIKeyRejected {
		t.Fatalf("first run error = %v, code = %q, want %q", err, CodeOf(err), CodeAPIKeyRejected)
	}

	if err := os.WriteFile(path, []byte("#!/bin/sh\necho 'unexpected boom' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("rewrite script: %v", err)
	}
	err := supervisor.Start(t.Context())
	if CodeOf(err) != CodeStartFailed {
		t.Fatalf("second run error = %v, code = %q, want %q", err, CodeOf(err), CodeStartFailed)
	}
	if !strings.Contains(err.Error(), "unexpected boom") {
		t.Fatalf("second run error = %v, want the current run's line", err)
	}
}

func TestDataDirectoryRefusalIsMapped(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the fake Soloist is a shell script")
	}
	dir := t.TempDir()
	path := writeScript(t, dir, "soloist", "#!/bin/sh\necho 'another Spotify Soloist process already using the data directory' >&2\nexit 1\n")
	supervisor := NewSupervisor(SupervisorOptions{
		Binary:       &Binary{Path: path, Version: "1.0.0", BuiltAt: time.Now().Add(-time.Hour)},
		APIKey:       "secret",
		DataDir:      t.TempDir(),
		CacheDir:     t.TempDir(),
		LogPath:      filepath.Join(t.TempDir(), "soloist.log"),
		StartTimeout: time.Second,
	})
	err := supervisor.Start(t.Context())
	if CodeOf(err) != CodeStartFailed || !strings.Contains(err.Error(), "data directory") {
		t.Fatalf("error = %v, code = %q, want a data-directory start failure", err, CodeOf(err))
	}
}

func TestInitialVolumeIsValidated(t *testing.T) {
	invalid := []int{-1, 101}
	for _, value := range invalid {
		volume := value
		supervisor := NewSupervisor(SupervisorOptions{InitialVolume: &volume})
		if supervisor.initialVolume != nil {
			t.Fatalf("InitialVolume %d was not dropped", value)
		}
		if args := buildArgs("Cadence", "key", "/data", "/cache", 0, &volume); strings.Contains(strings.Join(args, " "), "-i") {
			t.Fatalf("InitialVolume %d reached argv: %v", value, args)
		}
	}
	good := 55
	supervisor := NewSupervisor(SupervisorOptions{InitialVolume: &good})
	if supervisor.initialVolume == nil || *supervisor.initialVolume != good {
		t.Fatalf("InitialVolume = %v, want 55", supervisor.initialVolume)
	}
}

func TestStopCancelsAnInFlightStart(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the fake Soloist is a shell script")
	}
	dataDir := t.TempDir()
	pidFile := filepath.Join(t.TempDir(), "pids")
	script := fmt.Sprintf(`#!/bin/sh
echo $$ >> "%s"
trap 'exit 0' TERM INT
while :; do sleep 0.05; done
`, pidFile)
	path := writeScript(t, t.TempDir(), "soloist", script)
	supervisor := NewSupervisor(SupervisorOptions{
		Binary:       &Binary{Path: path, Version: "1.0.0", BuiltAt: time.Now().Add(-time.Hour)},
		APIKey:       "secret",
		DataDir:      dataDir,
		CacheDir:     t.TempDir(),
		LogPath:      filepath.Join(t.TempDir(), "soloist.log"),
		StartTimeout: defaultStartTimeout,
	})

	started := make(chan error, 1)
	go func() { started <- supervisor.Start(context.Background()) }()

	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(pidFile); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the fake Soloist did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	pids := readSpawnedPids(t, pidFile)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	stopped := make(chan error, 1)
	begin := time.Now()
	go func() { stopped <- supervisor.Stop(ctx) }()

	select {
	case err := <-stopped:
		if err != nil {
			t.Fatalf("stop: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Stop blocked behind the in-flight Start")
	}
	if elapsed := time.Since(begin); elapsed >= defaultStartTimeout {
		t.Fatalf("Stop took %s; it must not wait for the start timeout", elapsed)
	}

	select {
	case err := <-started:
		if err == nil {
			t.Fatal("Start succeeded although the fake never wrote ws.port")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Start did not return after cancellation")
	}

	if len(pids) == 0 {
		t.Fatal("no spawned pid recorded")
	}
	if err := syscall.Kill(pids[0], 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("process %d is still alive after Stop: %v", pids[0], err)
	}
}

func TestStartAutoPairsWithLibraryCredentials(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the fake Soloist is a shell script")
	}
	credentials := bytes.Repeat([]byte{0xab}, 260)
	ws := newFakeSoloist(t, false)
	zeroConf := newZeroConfTestServer(t, "device-abc", "alice", credentials)
	zeroConf.onAddUser = func() { ws.sendAuth(true) }
	dataDir := t.TempDir()
	programs := t.TempDir()
	binary := installFakeBinaryWithZeroConf(t, programs, dataDir, ws.port, zeroConf.port(), time.Now().Add(-time.Hour).Unix(), "secret")

	supervisor := NewSupervisor(SupervisorOptions{
		Binary:       binary,
		APIKey:       "secret",
		DataDir:      dataDir,
		CacheDir:     t.TempDir(),
		LogPath:      filepath.Join(t.TempDir(), "soloist.log"),
		StartTimeout: 5 * time.Second,
		LoginTimeout: 100 * time.Millisecond,
		EngineCredentials: func() (string, []byte, error) {
			return "alice", credentials, nil
		},
	})

	if err := supervisor.Start(t.Context()); err != nil {
		t.Fatalf("start: %v", err)
	}
	state := supervisor.State()
	if !state.Running || !state.LoggedIn || state.NeedsPairing {
		t.Fatalf("state = %+v", state)
	}
	infoCalls, addUserCalls := zeroConf.calls()
	if infoCalls != 1 || addUserCalls != 1 {
		t.Fatalf("ZeroConf calls: getInfo=%d addUser=%d", infoCalls, addUserCalls)
	}
	if err := supervisor.Stop(t.Context()); err != nil {
		t.Fatalf("stop: %v", err)
	}
}

func TestStartAutoPairFailureFallsBack(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the fake Soloist is a shell script")
	}
	credentials := bytes.Repeat([]byte{0xcd}, 260)
	ws := newFakeSoloist(t, false)
	zeroConf := newZeroConfTestServer(t, "device-def", "alice", credentials)
	zeroConf.addUserHTTP = http.StatusForbidden
	dataDir := t.TempDir()
	programs := t.TempDir()
	binary := installFakeBinaryWithZeroConf(t, programs, dataDir, ws.port, zeroConf.port(), time.Now().Add(-time.Hour).Unix(), "secret")

	supervisor := NewSupervisor(SupervisorOptions{
		Binary:       binary,
		APIKey:       "secret",
		DataDir:      dataDir,
		CacheDir:     t.TempDir(),
		LogPath:      filepath.Join(t.TempDir(), "soloist.log"),
		StartTimeout: 5 * time.Second,
		LoginTimeout: 50 * time.Millisecond,
		EngineCredentials: func() (string, []byte, error) {
			return "alice", credentials, nil
		},
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
	if err := supervisor.Stop(t.Context()); err != nil {
		t.Fatalf("stop: %v", err)
	}
}

func TestStartWithoutLibraryCredentialsSkipsAutoPair(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the fake Soloist is a shell script")
	}
	ws := newFakeSoloist(t, false)
	zeroConf := newZeroConfTestServer(t, "device-ghi", "alice", []byte("unused"))
	dataDir := t.TempDir()
	programs := t.TempDir()
	binary := installFakeBinaryWithZeroConf(t, programs, dataDir, ws.port, zeroConf.port(), time.Now().Add(-time.Hour).Unix(), "secret")

	supervisor := NewSupervisor(SupervisorOptions{
		Binary:       binary,
		APIKey:       "secret",
		DataDir:      dataDir,
		CacheDir:     t.TempDir(),
		LogPath:      filepath.Join(t.TempDir(), "soloist.log"),
		StartTimeout: 5 * time.Second,
		LoginTimeout: 50 * time.Millisecond,
		EngineCredentials: func() (string, []byte, error) {
			return "", nil, errors.New("the Spotify library account is not paired")
		},
	})

	err := supervisor.Start(t.Context())
	if !errors.Is(err, ErrPairingRequired) {
		t.Fatalf("error = %v, want ErrPairingRequired", err)
	}
	infoCalls, addUserCalls := zeroConf.calls()
	if infoCalls != 0 || addUserCalls != 0 {
		t.Fatalf("ZeroConf calls: getInfo=%d addUser=%d, want none", infoCalls, addUserCalls)
	}
	if err := supervisor.Stop(t.Context()); err != nil {
		t.Fatalf("stop: %v", err)
	}
}
