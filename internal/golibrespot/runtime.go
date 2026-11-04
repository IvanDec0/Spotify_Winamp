package golibrespot

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	engineName    = "go-librespot"
	defaultDevice = "Cadence"
)

// RuntimeOptions configures the managed go-librespot process.
type RuntimeOptions struct {
	BinaryPath   string
	ConfigDir    string
	LogPath      string
	DeviceName   string
	AudioBackend string
	AudioDevice  string
	Bitrate      int
	HTTPClient   *http.Client
	Emit         func(AudioState)
}

// Runtime owns the go-librespot sidecar and its loopback API client.
type Runtime struct {
	mu sync.Mutex

	binaryPath   string
	configDir    string
	logPath      string
	deviceName   string
	audioBackend string
	audioDevice  string
	bitrate      int
	httpClient   *http.Client
	emit         func(AudioState)

	client    *Client
	cmd       *exec.Cmd
	running   bool
	stopping  bool
	available bool
	lastError string
	done      chan struct{}
}

func NewRuntime(options RuntimeOptions) *Runtime {
	binary := findBinary(options.BinaryPath)
	deviceName := strings.TrimSpace(options.DeviceName)
	if deviceName == "" {
		deviceName = defaultDevice
	}
	backend := options.AudioBackend
	if backend == "" {
		backend = "pulseaudio"
	}
	device := options.AudioDevice
	if device == "" {
		device = "default"
	}
	bitrate := options.Bitrate
	if bitrate != 96 && bitrate != 160 && bitrate != 320 {
		bitrate = 320
	}
	return &Runtime{
		binaryPath:   binary,
		configDir:    options.ConfigDir,
		logPath:      options.LogPath,
		deviceName:   deviceName,
		audioBackend: backend,
		audioDevice:  device,
		bitrate:      bitrate,
		httpClient:   options.HTTPClient,
		emit:         options.Emit,
		available:    binary != "",
	}
}

func findBinary(explicit string) string {
	candidates := []string{}
	if explicit != "" {
		candidates = append(candidates, explicit)
	}
	if env := strings.TrimSpace(os.Getenv("CADENCE_GO_LIBRESPOT")); env != "" {
		candidates = append(candidates, env)
	}
	if path, err := exec.LookPath("go-librespot"); err == nil {
		candidates = append(candidates, path)
	}
	if executable, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(executable), "go-librespot"))
	}
	candidates = append(candidates, filepath.Join("bin", "go-librespot"))
	for _, candidate := range candidates {
		absolute, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		info, err := os.Stat(absolute)
		if err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
			return absolute
		}
	}
	return ""
}

func (r *Runtime) Available() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.available
}

func (r *Runtime) SetEmitter(emit func(AudioState)) {
	r.mu.Lock()
	r.emit = emit
	r.mu.Unlock()
}

func (r *Runtime) Client() *Client {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.client
}

func (r *Runtime) BinaryPath() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.binaryPath
}

func (r *Runtime) ConfigDir() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.configDir
}

// Start launches the engine. It is safe to call again after a failed start.
func (r *Runtime) Start(ctx context.Context) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return nil
	}
	binary := r.binaryPath
	configDir := r.configDir
	if binary == "" {
		r.lastError = "go-librespot is not installed. Build it with task audio:build or install a compatible binary."
		r.mu.Unlock()
		r.notify()
		return errors.New(r.lastError)
	}
	if configDir == "" {
		r.lastError = "the go-librespot config directory is not configured"
		r.mu.Unlock()
		r.notify()
		return errors.New(r.lastError)
	}
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		r.lastError = "cannot create the go-librespot config directory: " + err.Error()
		r.mu.Unlock()
		r.notify()
		return err
	}
	_ = os.Chmod(configDir, 0o700)
	port, err := freePort()
	if err != nil {
		r.lastError = "cannot allocate a local port for go-librespot: " + err.Error()
		r.mu.Unlock()
		r.notify()
		return err
	}
	if err := r.writeConfig(port, configDir); err != nil {
		r.lastError = "cannot write the go-librespot configuration: " + err.Error()
		r.mu.Unlock()
		r.notify()
		return err
	}
	_ = os.Remove(filepath.Join(configDir, "lockfile"))
	var logFile *os.File
	if r.logPath != "" {
		_ = os.MkdirAll(filepath.Dir(r.logPath), 0o700)
		logFile, _ = os.OpenFile(r.logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	}
	cmd := exec.Command(binary, "--config_dir", configDir)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGTERM}
	if logFile != nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}
	if err := cmd.Start(); err != nil {
		if logFile != nil {
			_ = logFile.Close()
		}
		r.lastError = "cannot start go-librespot: " + err.Error()
		r.mu.Unlock()
		r.notify()
		return err
	}
	if logFile != nil {
		_ = logFile.Close()
	}
	r.cmd = cmd
	r.client = NewClient(fmt.Sprintf("http://127.0.0.1:%d", port), r.httpClient)
	r.running = true
	r.stopping = false
	r.lastError = ""
	done := make(chan struct{})
	r.done = done
	r.mu.Unlock()

	go r.waitForExit(cmd, done)

	// Wait for the local API to come up.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-done:
			r.mu.Lock()
			lastError := r.lastError
			r.mu.Unlock()
			if lastError == "" {
				lastError = "go-librespot exited before the local API became ready"
			}
			r.notify()
			return errors.New(lastError)
		default:
		}
		if ctx.Err() != nil {
			_ = r.Stop(context.Background())
			return ctx.Err()
		}
		client := r.Client()
		if client != nil {
			if _, err := client.Root(ctx); err == nil {
				r.notify()
				return nil
			}
		}
		select {
		case <-ctx.Done():
			_ = r.Stop(context.Background())
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	_ = r.Stop(context.Background())
	r.mu.Lock()
	r.lastError = "go-librespot did not become ready in time"
	r.mu.Unlock()
	r.notify()
	return errors.New(r.lastError)
}

func (r *Runtime) waitForExit(cmd *exec.Cmd, done chan struct{}) {
	err := cmd.Wait()
	r.mu.Lock()
	if r.cmd == cmd {
		r.running = false
		r.client = nil
		if !r.stopping {
			if err != nil {
				r.lastError = "go-librespot exited: " + err.Error()
			} else {
				r.lastError = "go-librespot stopped unexpectedly"
			}
		}
		close(done)
	}
	r.mu.Unlock()
	r.notify()
}

// Stop terminates the sidecar.
func (r *Runtime) Stop(ctx context.Context) error {
	r.mu.Lock()
	if !r.running || r.cmd == nil || r.cmd.Process == nil {
		r.running = false
		r.client = nil
		r.mu.Unlock()
		return nil
	}
	cmd := r.cmd
	done := r.done
	r.stopping = true
	r.running = false
	r.client = nil
	r.mu.Unlock()

	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	select {
	case <-done:
	case <-ctx.Done():
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	case <-time.After(3 * time.Second):
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	r.mu.Lock()
	r.cmd = nil
	r.stopping = false
	r.mu.Unlock()
	r.notify()
	return nil
}

// Restart stops and starts the engine without clearing its credentials.
func (r *Runtime) Restart(ctx context.Context) error {
	if err := r.Stop(ctx); err != nil {
		return err
	}
	return r.Start(ctx)
}

// ResetSession stops the engine, removes its credentials and starts pairing
// again.
func (r *Runtime) ResetSession(ctx context.Context) error {
	if err := r.Stop(ctx); err != nil {
		return err
	}
	r.mu.Lock()
	configDir := r.configDir
	r.mu.Unlock()
	if err := os.Remove(filepath.Join(configDir, "state.json")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot remove the go-librespot session: %w", err)
	}
	_ = os.Remove(filepath.Join(configDir, "lockfile"))
	return r.Start(ctx)
}

// State queries the engine and returns token-free pairing information.
func (r *Runtime) State(ctx context.Context) AudioState {
	r.mu.Lock()
	available := r.available
	running := r.running
	client := r.client
	lastError := r.lastError
	configDir := r.configDir
	r.mu.Unlock()

	state := AudioState{
		Available:       available,
		Running:         running,
		Engine:          engineName,
		DeviceName:      r.deviceName,
		AudioBackend:    r.audioBackend,
		Error:           lastError,
		BinaryPath:      r.binaryPath,
		ConfigDirectory: configDir,
	}
	if !running || client == nil {
		return state
	}
	status, err := client.Status(ctx)
	if err != nil {
		state.Error = err.Error()
		return state
	}
	if status != nil {
		state.Authenticated = true
		state.DeviceName = status.DeviceName
		state.DeviceType = status.DeviceType
		state.Error = ""
		return state
	}
	code, err := client.AuthCode(ctx)
	if err != nil {
		state.Error = err.Error()
		return state
	}
	if code != nil {
		state.Pairing = &Pairing{
			URL:       code.URL,
			Code:      code.Code,
			ExpiresAt: code.ExpiresAt.UTC().Format(time.RFC3339),
		}
	}
	return state
}

func (r *Runtime) writeConfig(port int, configDir string) error {
	config := fmt.Sprintf(`log_level: info
device_name: %q
device_type: computer
bitrate: %d
# Prefer 443/80 access points; 4070 is blocked on some corporate networks.
prefer_firewall_friendly_ports: true
audio_backend: %s
audio_device: %s
zeroconf_enabled: false
credentials:
  type: device_auth
server:
  enabled: true
  address: 127.0.0.1
  port: %d
metadata:
  enabled: true
  context_sweep: true
  max_tracks: 800
mpris_enabled: false
`, r.deviceName, r.bitrate, r.audioBackend, r.audioDevice, port)
	return os.WriteFile(filepath.Join(configDir, "config.yml"), []byte(config), 0o600)
}

func freePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	return port, nil
}

func (r *Runtime) notify() {
	r.mu.Lock()
	emit := r.emit
	available := r.available
	running := r.running
	client := r.client
	lastError := r.lastError
	configDir := r.configDir
	r.mu.Unlock()
	if emit == nil {
		return
	}
	state := AudioState{
		Available: available, Running: running, Engine: engineName, DeviceName: r.deviceName,
		AudioBackend: r.audioBackend, Error: lastError, BinaryPath: r.binaryPath, ConfigDirectory: configDir,
	}
	if running && client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		if status, err := client.Status(ctx); err == nil && status != nil {
			state.Authenticated = true
			state.DeviceName = status.DeviceName
			state.DeviceType = status.DeviceType
			state.Error = ""
		}
		cancel()
	}
	emit(state)
}
