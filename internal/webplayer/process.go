package webplayer

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

const stopGrace = 3 * time.Second

// StartOptions configures one helper process.
type StartOptions struct {
	ControlURL string
	PlayerURL  string
	ProfileDir string
	LogPath    string
	ExtraArgs  []string
}

// Process owns one helper process and its redacted log.
type Process struct {
	cmd  *exec.Cmd
	done chan struct{}

	tail *tailBuffer
	log  *redactingWriter

	mu      sync.Mutex
	waitErr error

	stopOnce sync.Once
	stopDone chan struct{}
}

// Start launches the helper for one verified install. The caller owns the
// returned process and must Stop it.
func (r *Runtime) Start(ctx context.Context, install Install, opts StartOptions) (*Process, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	if install.Executable == "" || install.HelperDir == "" {
		return nil, codedError(CodeStartFailed, "the web player runtime is not installed", nil)
	}
	if err := validateControlURL(opts.ControlURL); err != nil {
		return nil, err
	}
	if err := validatePlayerURL(opts.PlayerURL); err != nil {
		return nil, err
	}

	profileDir := opts.ProfileDir
	if profileDir == "" {
		profileDir = filepath.Join(r.root, "profile")
	}
	logPath := opts.LogPath
	if logPath == "" {
		logPath = filepath.Join(r.root, "logs", "helper.log")
	}
	for _, dir := range []string{profileDir, filepath.Dir(logPath)} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, codedError(CodeStartFailed, "cannot create a web player directory", err)
		}
		_ = os.Chmod(dir, 0o700)
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, codedError(CodeStartFailed, "cannot open the web player log", err)
	}
	_ = logFile.Chmod(0o600)

	tail := newTailBuffer(tailLines)
	writer := newRedactingWriter(logFile, tail)

	args := []string{
		install.HelperDir,
		"--cadence-control=" + opts.ControlURL,
		"--cadence-url=" + opts.PlayerURL,
		"--user-data-dir=" + profileDir,
		"--disk-cache-size=67108864",
		"--media-cache-size=33554432",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-sync",
		"--disable-extensions",
		"--disable-background-timer-throttling",
		"--disable-renderer-backgrounding",
		"--disable-backgrounding-occluded-windows",
	}
	// Deliberately absent: --no-sandbox (user namespaces work here and the
	// Chromium sandbox must stay on) and --disable-component-update (the
	// component updater installs the Widevine CDM).
	args = append(args, opts.ExtraArgs...)

	cmd := exec.Command(install.Executable, args...)
	cmd.SysProcAttr = helperSysProcAttr()
	cmd.Stdout = writer
	cmd.Stderr = writer
	if err := cmd.Start(); err != nil {
		_ = writer.Close()
		return nil, codedError(CodeStartFailed, "cannot start the web player helper", err)
	}

	process := &Process{
		cmd:      cmd,
		done:     make(chan struct{}),
		tail:     tail,
		log:      writer,
		stopDone: make(chan struct{}),
	}
	go process.wait(cmd)
	return process, nil
}

func (p *Process) wait(cmd *exec.Cmd) {
	err := cmd.Wait()
	p.mu.Lock()
	p.waitErr = err
	p.mu.Unlock()
	// Closing the writer flushes any unterminated last line and the log file.
	_ = p.log.Close()
	close(p.done)
}

// Wait blocks until the helper exits and returns its wait error.
func (p *Process) Wait() error {
	if p == nil {
		return nil
	}
	<-p.done
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.waitErr
}

// Stop terminates the helper process group: SIGTERM, three seconds, then
// SIGKILL. It is idempotent and safe to call concurrently.
func (p *Process) Stop(ctx context.Context) error {
	if p == nil {
		return nil
	}
	p.stopOnce.Do(func() {
		defer close(p.stopDone)
		p.stop(ctx)
	})
	<-p.stopDone
	return nil
}

func (p *Process) stop(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	// Closing an already-closed writer is a no-op; this covers the unusual
	// case where the process ignores SIGKILL past the wait window.
	defer func() { _ = p.log.Close() }()
	select {
	case <-p.done:
		return
	default:
	}
	if p.cmd == nil || p.cmd.Process == nil {
		return
	}

	_ = signalHelper(p.cmd, false)
	timer := time.NewTimer(stopGrace)
	defer timer.Stop()
	select {
	case <-p.done:
		return
	case <-ctx.Done():
		_ = signalHelper(p.cmd, true)
		waitAfterKill(p.done)
		return
	case <-timer.C:
		_ = signalHelper(p.cmd, true)
		waitAfterKill(p.done)
	}
}

func waitAfterKill(done <-chan struct{}) {
	select {
	case <-done:
	case <-time.After(time.Second):
	}
}

// Tail returns a copy of the last redacted log lines, at most 200.
func (p *Process) Tail() []string {
	if p == nil || p.tail == nil {
		return nil
	}
	return p.tail.snapshot()
}

func validateControlURL(raw string) error {
	return validateLoopbackURL(raw, "ws", "control")
}

func validatePlayerURL(raw string) error {
	return validateLoopbackURL(raw, "http", "player")
}

func validateLoopbackURL(raw, scheme, label string) error {
	target, err := url.Parse(raw)
	if err != nil {
		return codedError(CodeStartFailed, fmt.Sprintf("the web player %s URL is not valid", label), err)
	}
	if target.Scheme != scheme {
		return codedError(CodeStartFailed, fmt.Sprintf("the web player %s URL must use %s", label, scheme), nil)
	}
	if target.Hostname() != "127.0.0.1" {
		return codedError(CodeStartFailed, fmt.Sprintf("the web player %s URL must use loopback", label), nil)
	}
	if target.User != nil {
		return codedError(CodeStartFailed, fmt.Sprintf("the web player %s URL must not embed credentials", label), nil)
	}
	port, err := strconv.Atoi(target.Port())
	if err != nil || port <= 0 || port > 65535 {
		return codedError(CodeStartFailed, fmt.Sprintf("the web player %s URL must have a valid port", label), nil)
	}
	if target.Path == "" {
		return codedError(CodeStartFailed, fmt.Sprintf("the web player %s URL must have a path", label), nil)
	}
	return nil
}
