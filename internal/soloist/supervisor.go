package soloist

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	// expiredExitCode is Soloist's exit status once its build is older than
	// 90 days.
	expiredExitCode = 10
	// defaultStartTimeout bounds the wait for Soloist's WebSocket port.
	defaultStartTimeout = 20 * time.Second
	// defaultStoredLoginTimeout is used when an account is already stored.
	defaultStoredLoginTimeout = 15 * time.Second
	// defaultFreshLoginTimeout is used when no account is stored yet.
	defaultFreshLoginTimeout = 4 * time.Second
	// authWaitTimeout bounds the initial auth_state wait after connecting.
	authWaitTimeout = 5 * time.Second
	// portPollInterval is how often ws.port is checked.
	portPollInterval = 100 * time.Millisecond
	// stopGrace is how long Soloist may take to exit after SIGTERM.
	stopGrace = 3 * time.Second
	// tailLines is how many redacted log lines are kept for diagnostics.
	tailLines = 20
	// tailLineLimit bounds one kept line.
	tailLineLimit = 4096
	// pendingLogLimit bounds an unterminated log line held in memory.
	pendingLogLimit = 64 << 10
	// defaultDeviceName is the Connect device name Soloist advertises.
	defaultDeviceName = "Cadence"
	// autoPairTimeout bounds ZeroConf discovery and the login confirmation.
	autoPairTimeout = 5 * time.Second
	// zeroConfLogPollInterval is how often the Soloist log is checked for the
	// ZeroConf port.
	zeroConfLogPollInterval = 100 * time.Millisecond
)

var zeroConfLogPattern = regexp.MustCompile(`Started ZeroConf service on port (\d+) path (\S+)`)

// SupervisorOptions configures the managed Soloist process.
type SupervisorOptions struct {
	Binary        *Binary
	APIKey        string
	DeviceName    string
	DataDir       string
	CacheDir      string
	LogPath       string
	CacheSizeMB   int
	InitialVolume *int
	StartTimeout  time.Duration
	LoginTimeout  time.Duration
	Emit          func(State)
	// EngineCredentials returns the paired library account and its reusable
	// Spotify credentials. It is an optional best-effort auto-pair source.
	EngineCredentials func() (username string, data []byte, err error)
}

// Supervisor owns the Soloist child process and its WebSocket client.
type Supervisor struct {
	// lifecycleMu serializes whole Start, Stop and Restart sequences so two
	// callers can never spawn two Soloist processes.
	lifecycleMu sync.Mutex

	mu sync.Mutex

	binary        *Binary
	apiKey        string
	deviceName    string
	reportedName  string
	dataDir       string
	cacheDir      string
	logPath       string
	cacheSizeMB   int
	initialVolume *int
	startTimeout  time.Duration
	loginTimeout  time.Duration
	emit          func(State)
	tail          *tailBuffer
	// engineCredentials feeds optional automatic account pairing from the
	// library engine's stored Spotify session.
	engineCredentials func() (username string, data []byte, err error)

	client    *Client
	cmd       *exec.Cmd
	logWriter *redactingWriter
	done      chan struct{}
	running   bool
	stopping  bool
	loggedIn  bool
	needsPair bool
	active    bool
	accounts  []string
	errorText string
	errorCode string
	// startCancel cancels an in-flight Start so Stop can interrupt the port,
	// dial and login waits instead of blocking behind lifecycleMu for them.
	startCancel context.CancelFunc
}

// NewSupervisor returns a supervisor. LoginTimeout 0 is resolved at Start
// from the accounts present in DataDir.
func NewSupervisor(opts SupervisorOptions) *Supervisor {
	if strings.TrimSpace(opts.DeviceName) == "" {
		opts.DeviceName = defaultDeviceName
	}
	if opts.StartTimeout <= 0 {
		opts.StartTimeout = defaultStartTimeout
	}
	initialVolume := opts.InitialVolume
	if initialVolume != nil && (*initialVolume < 0 || *initialVolume > 100) {
		initialVolume = nil
	}
	return &Supervisor{
		binary:            opts.Binary,
		apiKey:            strings.TrimSpace(opts.APIKey),
		deviceName:        opts.DeviceName,
		dataDir:           opts.DataDir,
		cacheDir:          opts.CacheDir,
		logPath:           opts.LogPath,
		cacheSizeMB:       opts.CacheSizeMB,
		initialVolume:     initialVolume,
		startTimeout:      opts.StartTimeout,
		loginTimeout:      opts.LoginTimeout,
		emit:              opts.Emit,
		tail:              newTailBuffer(tailLines),
		accounts:          Accounts(opts.DataDir),
		engineCredentials: opts.EngineCredentials,
	}
}

// Start launches Soloist, waits for its API and authentication state, and
// returns ErrPairingRequired while leaving the process running when no
// account is signed in yet.
func (s *Supervisor) Start(ctx context.Context) error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	return s.startLocked(ctx)
}

func (s *Supervisor) startLocked(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.startCancel = cancel
	binary := s.binary
	apiKey := s.apiKey
	dataDir := s.dataDir
	cacheDir := s.cacheDir
	logPath := s.logPath
	deviceName := s.deviceName
	cacheSizeMB := s.cacheSizeMB
	initialVolume := s.initialVolume
	startTimeout := s.startTimeout
	loginTimeout := s.loginTimeout
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.startCancel = nil
		s.mu.Unlock()
	}()

	// A second Start while Soloist is running is a no-op and must not drop
	// the diagnostic lines of the running process. Only a real new run resets
	// the tail, so an earlier run cannot decide how this one is classified.
	s.tail.reset()

	if binary == nil {
		s.fail(ErrNotInstalled)
		return ErrNotInstalled
	}
	if dataDir == "" {
		err := codedError(CodeStartFailed, "the Spotify Soloist data directory is not configured", nil)
		s.fail(err)
		return err
	}
	if strings.TrimSpace(apiKey) == "" {
		s.fail(ErrAPIKeyMissing)
		return ErrAPIKeyMissing
	}
	if err := ValidateAPIKey(apiKey); err != nil {
		s.fail(err)
		return err
	}
	for _, dir := range []string{dataDir, cacheDir} {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			coded := codedError(CodeStartFailed, "cannot create a Spotify Soloist directory", err)
			s.fail(coded)
			return coded
		}
		_ = os.Chmod(dir, 0o700)
	}
	if logPath != "" {
		if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
			coded := codedError(CodeStartFailed, "cannot create the Spotify Soloist log directory", err)
			s.fail(coded)
			return coded
		}
	}

	cleanupStale(dataDir, binary.Path)

	var logFile *os.File
	if logPath != "" {
		var err error
		logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			coded := codedError(CodeStartFailed, "cannot open the Spotify Soloist log", err)
			s.fail(coded)
			return coded
		}
		_ = logFile.Chmod(0o600)
	}
	var destination io.Writer
	if logFile != nil {
		destination = logFile
	}
	writer := newRedactingWriter(destination, apiKey, s.tail)

	cmd := exec.Command(binary.Path, buildArgs(deviceName, apiKey, dataDir, cacheDir, cacheSizeMB, initialVolume)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGTERM}
	cmd.Stdout = writer
	cmd.Stderr = writer
	if err := cmd.Start(); err != nil {
		_ = writer.Close()
		coded := codedError(CodeStartFailed, "cannot start Spotify Soloist", err)
		s.fail(coded)
		return coded
	}

	done := make(chan struct{})
	s.mu.Lock()
	s.cmd = cmd
	s.logWriter = writer
	s.done = done
	s.running = true
	s.stopping = false
	s.loggedIn = false
	s.needsPair = false
	s.active = false
	s.reportedName = ""
	s.errorText = ""
	s.errorCode = ""
	s.accounts = Accounts(dataDir)
	s.mu.Unlock()
	go s.waitForExit(cmd, done, writer)
	s.emitState()

	port, err := s.waitForPort(ctx, done, dataDir, startTimeout)
	if err != nil {
		_ = s.stopLocked(context.Background())
		s.fail(err)
		return err
	}

	client, err := Dial(ctx, fmt.Sprintf("ws://127.0.0.1:%d", port))
	if err != nil {
		_ = s.stopLocked(context.Background())
		coded := codedError(CodeStartFailed, "cannot connect to Spotify Soloist", err)
		s.fail(coded)
		return coded
	}
	s.mu.Lock()
	s.client = client
	s.mu.Unlock()

	event, err := client.Await(ctx, "auth_state", authWaitTimeout)
	if err != nil {
		_ = s.stopLocked(context.Background())
		coded := codedError(CodeStartFailed, "Spotify Soloist did not report its authentication state", err)
		s.fail(coded)
		return coded
	}
	s.applyAuth(event)

	if s.isRunning() {
		s.mu.Lock()
		loggedIn := s.loggedIn
		s.mu.Unlock()
		if !loggedIn {
			deadline := loginTimeout
			if deadline <= 0 {
				if len(Accounts(dataDir)) > 0 {
					deadline = defaultStoredLoginTimeout
				} else {
					deadline = defaultFreshLoginTimeout
				}
			}
			expires := time.Now().Add(deadline)
			for !loggedIn {
				remaining := time.Until(expires)
				if remaining <= 0 {
					break
				}
				next, err := client.Await(ctx, "auth_state", remaining)
				if err != nil {
					break
				}
				s.applyAuth(next)
				s.mu.Lock()
				loggedIn = s.loggedIn
				s.mu.Unlock()
			}
		}
	}

	if !s.isRunning() {
		return s.startFailure()
	}

	s.mu.Lock()
	loggedIn := s.loggedIn
	s.mu.Unlock()
	if !loggedIn {
		// Best effort: sign in with the library engine's account before
		// falling back to the one-tap pairing state.
		_ = s.tryAutoPair(ctx, client)
	}

	s.mu.Lock()
	loggedIn = s.loggedIn
	if loggedIn {
		s.needsPair = false
		if s.errorCode == CodePairingRequired {
			s.errorCode = ""
			s.errorText = ""
		}
	} else {
		s.needsPair = true
		s.errorCode = CodePairingRequired
		s.errorText = ErrPairingRequired.Message
	}
	client = s.client
	s.mu.Unlock()

	if client != nil {
		go s.observe(client)
	}
	s.emitState()
	if !loggedIn {
		return ErrPairingRequired
	}
	return nil
}

func (s *Supervisor) waitForPort(ctx context.Context, done <-chan struct{}, dataDir string, timeout time.Duration) (int, error) {
	deadline := time.Now().Add(timeout)
	portPath := filepath.Join(dataDir, "ws.port")
	for {
		select {
		case <-done:
			return 0, s.startFailure()
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
		if data, err := os.ReadFile(portPath); err == nil {
			if port, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && port > 0 && port <= 65535 {
				return port, nil
			}
		}
		if time.Now().After(deadline) {
			return 0, codedError(CodeStartFailed, "Spotify Soloist did not open its local API in time", nil)
		}
		select {
		case <-done:
			return 0, s.startFailure()
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(portPollInterval):
		}
	}
}

func (s *Supervisor) startFailure() error {
	s.mu.Lock()
	code := s.errorCode
	text := s.errorText
	s.mu.Unlock()
	if code == "" {
		code = CodeStartFailed
	}
	if text == "" {
		text = "Spotify Soloist exited before it was ready"
	}
	return codedError(code, text, nil)
}

// tryAutoPair signs Soloist in with the reusable Spotify credentials of the
// library engine. It returns an error for every failed step so Start keeps the
// one-tap pairing state; it never logs the credentials.
func (s *Supervisor) tryAutoPair(ctx context.Context, client *Client) error {
	if client == nil {
		return errors.New("the Spotify Soloist client is not connected")
	}
	s.mu.Lock()
	credentials := s.engineCredentials
	binary := s.binary
	running := s.running
	stopping := s.stopping
	deviceName := s.deviceName
	s.mu.Unlock()

	if credentials == nil {
		return errors.New("the Spotify library credentials are not available")
	}
	if !running || stopping {
		return errors.New("Spotify Soloist is not running")
	}
	if binary != nil && binary.Expired(time.Now()) {
		return ErrExpired
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	username, data, err := credentials()
	if err != nil {
		return err
	}
	if strings.TrimSpace(username) == "" || len(data) == 0 {
		return errors.New("the Spotify library credentials are empty")
	}

	port, path, err := s.discoverZeroConf(ctx)
	if err != nil {
		return err
	}
	if err := PairWithCredentials(ctx, port, path, deviceName, username, data); err != nil {
		return err
	}

	deadline := time.Now().Add(autoPairTimeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return errors.New("Spotify Soloist did not sign in with the library account")
		}
		event, err := client.Await(ctx, "auth_state", remaining)
		if err != nil {
			return err
		}
		s.applyAuth(event)
		s.mu.Lock()
		loggedIn := s.loggedIn
		s.mu.Unlock()
		if loggedIn {
			s.mu.Lock()
			s.loggedIn = true
			s.needsPair = false
			if s.errorCode == CodePairingRequired {
				s.errorCode = ""
				s.errorText = ""
			}
			s.mu.Unlock()
			return nil
		}
	}
}

// discoverZeroConf finds the loopback ZeroConf port and path: the Soloist log
// line first, then the listening sockets of the child process.
func (s *Supervisor) discoverZeroConf(ctx context.Context) (int, string, error) {
	s.mu.Lock()
	logPath := s.logPath
	dataDir := s.dataDir
	cmd := s.cmd
	s.mu.Unlock()

	var lastErr error
	if logPath != "" {
		port, path, err := waitForZeroConfLog(ctx, logPath)
		if err == nil {
			return port, path, nil
		}
		lastErr = err
	}

	exclude := readWSPort(filepath.Join(dataDir, "ws.port"))
	pid := 0
	if cmd != nil && cmd.Process != nil {
		pid = cmd.Process.Pid
	}
	if pid <= 0 {
		if lastErr != nil {
			return 0, "", lastErr
		}
		return 0, "", errors.New("the Spotify Soloist process is not available")
	}

	ports := zeroConfPortsFromSS(ctx, pid, exclude)
	if len(ports) == 0 {
		if lastErr != nil {
			return 0, "", lastErr
		}
		return 0, "", errors.New("the Spotify Soloist ZeroConf endpoint was not found")
	}
	deadline := time.Now().Add(autoPairTimeout)
	for _, port := range ports {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		probeCtx, cancel := context.WithTimeout(ctx, remaining)
		path, err := ProbeZeroConf(probeCtx, port)
		cancel()
		if err == nil {
			return port, path, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return 0, "", lastErr
	}
	return 0, "", errors.New("the Spotify Soloist ZeroConf endpoint did not respond")
}

// waitForZeroConfLog polls the current run's log for the ZeroConf port line.
func waitForZeroConfLog(ctx context.Context, logPath string) (int, string, error) {
	deadline := time.Now().Add(autoPairTimeout)
	for {
		if data, err := os.ReadFile(logPath); err == nil {
			if match := zeroConfLogPattern.FindSubmatch(data); len(match) == 3 {
				if port, err := strconv.Atoi(string(match[1])); err == nil && port > 0 && port <= 65535 {
					return port, string(match[2]), nil
				}
			}
		}
		if err := ctx.Err(); err != nil {
			return 0, "", err
		}
		if time.Now().After(deadline) {
			return 0, "", errors.New("the Spotify Soloist ZeroConf port did not appear in its log")
		}
		select {
		case <-ctx.Done():
			return 0, "", ctx.Err()
		case <-time.After(zeroConfLogPollInterval):
		}
	}
}

// zeroConfPortsFromSS lists the local listening ports of the Soloist child via
// ss. A missing ss or a failed call simply yields no ports.
func zeroConfPortsFromSS(ctx context.Context, pid, exclude int) []int {
	probeCtx, cancel := context.WithTimeout(ctx, autoPairTimeout)
	defer cancel()
	output, err := exec.CommandContext(probeCtx, "ss", "-ltnp").Output()
	if err != nil {
		return nil
	}
	seen := make(map[int]bool)
	var ports []int
	for _, line := range strings.Split(string(output), "\n") {
		if !lineHasPID(line, pid) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		local := fields[3]
		index := strings.LastIndexByte(local, ':')
		if index < 0 {
			continue
		}
		port, err := strconv.Atoi(local[index+1:])
		if err != nil || port <= 0 || port > 65535 || port == exclude || seen[port] {
			continue
		}
		seen[port] = true
		ports = append(ports, port)
	}
	return ports
}

func lineHasPID(line string, pid int) bool {
	needle := "pid=" + strconv.Itoa(pid)
	for {
		index := strings.Index(line, needle)
		if index < 0 {
			return false
		}
		rest := line[index+len(needle):]
		if rest == "" || rest[0] == ',' || rest[0] == ')' || rest[0] == ' ' {
			return true
		}
		line = rest
	}
}

func readWSPort(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	port, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || port <= 0 || port > 65535 {
		return 0
	}
	return port
}

func (s *Supervisor) applyAuth(event Event) {
	if event.Auth == nil {
		return
	}
	s.mu.Lock()
	changed := false
	if s.loggedIn != event.Auth.LoggedIn {
		s.loggedIn = event.Auth.LoggedIn
		changed = true
	}
	if s.active != event.Auth.IsActive {
		s.active = event.Auth.IsActive
		changed = true
	}
	if event.Auth.DeviceName != "" && s.reportedName != event.Auth.DeviceName {
		s.reportedName = event.Auth.DeviceName
		changed = true
	}
	if event.Auth.LoggedIn {
		if s.needsPair {
			s.needsPair = false
			changed = true
		}
		if s.errorCode == CodePairingRequired {
			s.errorCode = ""
			s.errorText = ""
			changed = true
		}
		s.accounts = Accounts(s.dataDir)
	}
	s.mu.Unlock()
	if changed {
		s.emitState()
	}
}

func (s *Supervisor) observe(client *Client) {
	events, unsubscribe := client.Subscribe()
	defer unsubscribe()
	// Soloist may have signed in between the startup wait and this
	// subscription; the client keeps the last auth_state for replay.
	if event, err := client.Await(context.Background(), "auth_state", time.Millisecond); err == nil {
		s.applyAuth(event)
	}
	for {
		select {
		case <-client.Done():
			return
		case event := <-events:
			switch event.Type {
			case "auth_state":
				s.applyAuth(event)
			case "playback_state":
				if event.Playback != nil {
					s.applyActive(event.Playback.IsActive, "")
				}
			case "device_changed":
				s.applyActive(derefBool(event.IsActive), event.DeviceName)
			}
		}
	}
}

func (s *Supervisor) applyActive(active bool, device string) {
	s.mu.Lock()
	changed := false
	if s.active != active {
		s.active = active
		changed = true
	}
	if device != "" && s.reportedName != device {
		s.reportedName = device
		changed = true
	}
	s.mu.Unlock()
	if changed {
		s.emitState()
	}
}

func derefBool(value *bool) bool {
	return value != nil && *value
}

func (s *Supervisor) waitForExit(cmd *exec.Cmd, done chan struct{}, writer *redactingWriter) {
	err := cmd.Wait()
	if writer != nil {
		_ = writer.Close()
	}
	s.mu.Lock()
	if s.cmd != cmd {
		s.mu.Unlock()
		close(done)
		return
	}
	stopping := s.stopping
	client := s.client
	s.client = nil
	s.logWriter = nil
	s.running = false
	s.cmd = nil
	if !stopping {
		s.errorCode, s.errorText = s.exitFailureLocked(err)
		s.loggedIn = false
		s.needsPair = false
		s.active = false
	}
	close(done)
	s.mu.Unlock()
	if client != nil {
		_ = client.Close()
	}
	if !stopping {
		s.emitState()
	}
}

func (s *Supervisor) exitFailureLocked(err error) (string, string) {
	code := exitCode(err)
	if code == expiredExitCode {
		return CodeBuildExpired, ErrExpired.Message + ". Install an update from Settings."
	}
	if s.tail.contains("another session is running for data directory") ||
		s.tail.contains("another Spotify Soloist process already using the data directory") {
		return CodeStartFailed, "another Spotify Soloist process is already using its data directory"
	}
	if s.tail.contains("make sure --api-key is valid") || s.tail.contains("--api-key is required") {
		return CodeAPIKeyRejected, "Spotify rejected the Spotify Soloist API key"
	}
	if line := s.tail.last(); line != "" {
		return CodeStartFailed, line
	}
	return CodeStartFailed, fmt.Sprintf("Spotify Soloist stopped unexpectedly (exit code %d)", code)
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

// Stop terminates Soloist and its process group. It is idempotent. An
// in-flight Start is cancelled first so Stop never waits behind the port or
// login timeouts.
func (s *Supervisor) Stop(ctx context.Context) error {
	s.mu.Lock()
	cancel := s.startCancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	return s.stopLocked(ctx)
}

func (s *Supervisor) stopLocked(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	s.mu.Lock()
	cmd := s.cmd
	done := s.done
	client := s.client
	if cmd == nil || cmd.Process == nil || !s.running {
		s.cmd = nil
		s.client = nil
		s.running = false
		s.stopping = false
		s.loggedIn = false
		s.needsPair = false
		s.active = false
		s.errorText = ""
		s.errorCode = ""
		s.mu.Unlock()
		if client != nil {
			_ = client.Close()
		}
		s.removeRuntimeFiles()
		s.emitState()
		return nil
	}
	s.stopping = true
	s.running = false
	s.client = nil
	s.mu.Unlock()

	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			select {
			case <-done:
			case <-time.After(time.Second):
			}
		case <-time.After(stopGrace):
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			select {
			case <-done:
			case <-time.After(time.Second):
			}
		}
	}
	if client != nil {
		_ = client.Close()
	}

	s.mu.Lock()
	if s.cmd == cmd {
		s.cmd = nil
	}
	s.stopping = false
	s.loggedIn = false
	s.needsPair = false
	s.active = false
	s.errorText = ""
	s.errorCode = ""
	s.mu.Unlock()
	s.removeRuntimeFiles()
	s.emitState()
	return nil
}

// Restart stops and starts Soloist.
func (s *Supervisor) Restart(ctx context.Context) error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if err := s.stopLocked(ctx); err != nil {
		return err
	}
	return s.startLocked(ctx)
}

// State returns a copy of the supervisor state with a cloned Accounts slice.
func (s *Supervisor) State() State {
	s.mu.Lock()
	device := s.deviceName
	if s.reportedName != "" {
		device = s.reportedName
	}
	state := State{
		Installed:    s.binary != nil,
		Running:      s.running,
		LoggedIn:     s.loggedIn,
		NeedsPairing: s.needsPair,
		Active:       s.active,
		DeviceName:   device,
		Accounts:     append([]string(nil), s.accounts...),
		Error:        s.errorText,
		ErrorCode:    s.errorCode,
	}
	if s.binary != nil {
		now := time.Now()
		state.BinaryPath = s.binary.Path
		state.Version = s.binary.Version
		state.BuiltAt = s.binary.BuiltAt
		state.ExpiresAt = s.binary.ExpiresAt()
		state.Expired = s.binary.Expired(now)
		state.DaysLeft = s.binary.DaysLeft(now)
	}
	s.mu.Unlock()
	return state
}

// Client returns the WebSocket client while Soloist is running.
func (s *Supervisor) Client() *Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.client
}

// SetEmitter replaces the state emitter.
func (s *Supervisor) SetEmitter(emit func(State)) {
	s.mu.Lock()
	s.emit = emit
	s.mu.Unlock()
}

// setAPIKey replaces the key used by the next start. The running process
// keeps the key it was started with.
func (s *Supervisor) setAPIKey(key string) {
	s.mu.Lock()
	s.apiKey = strings.TrimSpace(key)
	s.mu.Unlock()
}

// apiKeyValue is an unexported test hook for the trimmed key.
func (s *Supervisor) apiKeyValue() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.apiKey
}

// Accounts lists the accounts Soloist has stored under
// <dataDir>/settings/Users, sorted and without the "-user" suffix.
func Accounts(dataDir string) []string {
	entries, err := os.ReadDir(filepath.Join(dataDir, "settings", "Users"))
	if err != nil {
		return nil
	}
	accounts := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name, ok := strings.CutSuffix(entry.Name(), "-user")
		if !ok || name == "" {
			continue
		}
		accounts = append(accounts, name)
	}
	sort.Strings(accounts)
	return accounts
}

func (s *Supervisor) fail(err error) {
	s.mu.Lock()
	s.errorCode = CodeOf(err)
	s.errorText = ""
	if err != nil {
		s.errorText = err.Error()
	}
	if s.errorCode == "" {
		s.errorCode = CodeStartFailed
	}
	s.mu.Unlock()
	s.emitState()
}

func (s *Supervisor) isRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *Supervisor) emitState() {
	s.mu.Lock()
	emit := s.emit
	s.mu.Unlock()
	if emit == nil {
		return
	}
	emit(s.State())
}

func (s *Supervisor) removeRuntimeFiles() {
	if s.dataDir == "" {
		return
	}
	_ = os.Remove(filepath.Join(s.dataDir, "ws.addr"))
	_ = os.Remove(filepath.Join(s.dataDir, "ws.port"))
}

func buildArgs(deviceName, apiKey, dataDir, cacheDir string, cacheSizeMB int, initialVolume *int) []string {
	args := []string{"-n", deviceName, "-k", apiKey, "-D", dataDir, "-C", cacheDir, "-w", "127.0.0.1:0"}
	if cacheSizeMB >= 100 {
		args = append(args, "-z", strconv.Itoa(cacheSizeMB))
	}
	if initialVolume != nil && *initialVolume >= 0 && *initialVolume <= 100 {
		args = append(args, "-i", strconv.Itoa(*initialVolume))
	}
	return args
}

func cleanupStale(dataDir, binaryPath string) {
	if dataDir == "" {
		return
	}
	_ = os.Remove(filepath.Join(dataDir, "ws.addr"))
	_ = os.Remove(filepath.Join(dataDir, "ws.port"))
	pidPath := filepath.Join(dataDir, "soloist.pid")
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		_ = os.Remove(pidPath)
		return
	}
	if !processAlive(pid) {
		_ = os.Remove(pidPath)
		return
	}
	cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		// Cannot verify the process; leave it alone rather than risk killing
		// an unrelated process that reused the pid.
		return
	}
	if !matchesSoloistProcess(cmdline, binaryPath, dataDir) {
		return
	}
	_ = syscall.Kill(pid, syscall.SIGTERM)
	for attempt := 0; attempt < 30; attempt++ {
		time.Sleep(100 * time.Millisecond)
		if !processAlive(pid) {
			_ = os.Remove(pidPath)
			return
		}
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
	time.Sleep(200 * time.Millisecond)
	_ = os.Remove(pidPath)
}

func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// matchesSoloistProcess reports whether a raw /proc/<pid>/cmdline belongs to
// this Soloist binary and data directory. Arguments are NUL-separated, so a
// substring match would let an unrelated command such as
// "tail -f <dataDir>/soloist.log" be killed.
func matchesSoloistProcess(cmdline []byte, binaryPath, dataDir string) bool {
	args := strings.Split(string(cmdline), "\x00")
	for len(args) > 0 && args[len(args)-1] == "" {
		args = args[:len(args)-1]
	}
	if len(args) == 0 {
		return false
	}
	if filepath.Base(args[0]) != filepath.Base(binaryPath) {
		return false
	}
	want := filepath.Clean(dataDir)
	for _, arg := range args[1:] {
		if filepath.Clean(arg) == want {
			return true
		}
	}
	return false
}

type redactingWriter struct {
	mu          sync.Mutex
	secret      string
	destination io.Writer
	tail        *tailBuffer
	pending     []byte
	closed      bool
}

func newRedactingWriter(destination io.Writer, secret string, tail *tailBuffer) *redactingWriter {
	return &redactingWriter{secret: secret, destination: destination, tail: tail}
}

func (w *redactingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return len(p), nil
	}
	w.pending = append(w.pending, p...)
	for {
		index := bytes.IndexByte(w.pending, '\n')
		if index < 0 {
			break
		}
		w.writeLine(w.pending[:index+1])
		w.pending = w.pending[index+1:]
	}
	if len(w.pending) > pendingLogLimit {
		// Keep enough trailing bytes to catch a secret split across the
		// flush boundary instead of logging half of it.
		keep := len(w.secret) - 1
		if keep < 0 {
			keep = 0
		}
		if keep > pendingLogLimit {
			keep = pendingLogLimit
		}
		if keep > len(w.pending) {
			keep = len(w.pending)
		}
		cut := len(w.pending) - keep
		w.writeLine(w.pending[:cut])
		w.pending = append([]byte(nil), w.pending[cut:]...)
	}
	return len(p), nil
}

func (w *redactingWriter) writeLine(line []byte) {
	text := redact(string(line), w.secret)
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	if w.destination != nil {
		_, _ = io.WriteString(w.destination, text)
	}
	trimmed := truncateRunes(strings.TrimRight(text, "\r\n"), tailLineLimit)
	w.tail.add(trimmed)
}

func (w *redactingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	if len(w.pending) > 0 {
		w.writeLine(w.pending)
		w.pending = nil
	}
	if closer, ok := w.destination.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func redact(text, secret string) string {
	if secret == "" {
		return text
	}
	return strings.ReplaceAll(text, secret, "[redacted]")
}

type tailBuffer struct {
	mu    sync.Mutex
	lines []string
	max   int
}

func newTailBuffer(max int) *tailBuffer {
	return &tailBuffer{max: max}
}

func (t *tailBuffer) add(line string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lines = append(t.lines, line)
	if len(t.lines) > t.max {
		t.lines = t.lines[len(t.lines)-t.max:]
	}
}

// reset drops every line from an earlier run.
func (t *tailBuffer) reset() {
	t.mu.Lock()
	t.lines = nil
	t.mu.Unlock()
}

func (t *tailBuffer) last() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	for index := len(t.lines) - 1; index >= 0; index-- {
		if line := strings.TrimSpace(t.lines[index]); line != "" {
			return line
		}
	}
	return ""
}

func (t *tailBuffer) contains(fragment string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, line := range t.lines {
		if strings.Contains(line, fragment) {
			return true
		}
	}
	return false
}
