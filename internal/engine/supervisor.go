package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	Name          = "go-librespot"
	defaultDevice = "Cadence"
)

var engineVersionPattern = regexp.MustCompile(`running go-librespot ([^\s"]+)"`)

const engineVersionRetryInterval = 2 * time.Second

// Options configures the managed go-librespot process.
type Options struct {
	BinaryPath   string
	ConfigDir    string
	LogPath      string
	DeviceName   string
	AudioBackend string
	AudioDevice  string
	Bitrate      int
	// CrossfadeDurationMS is the crossfade between tracks; 0 disables it.
	CrossfadeDurationMS int
	// NormalisationDisabled disables track/album normalisation.
	NormalisationDisabled     bool
	NormalisationUseAlbumGain bool
	NormalisationPregain      float32
	CacheEnabled              bool
	CacheDir                  string
	CacheSizeLimit            string
	FlacEnabled               bool
	MPRISEnabled              bool
	// DeviceIsGroup advertises a speaker-group icon; it does not enable Jam.
	DeviceIsGroup bool
	ImageSize     string
	HTTPClient    *http.Client
	Emit          func(AudioState)
	// Client overrides the loopback API client. It is used by tests that do
	// not start the sidecar; when nil, Start creates one.
	Client *Client
}

// Supervisor owns the go-librespot sidecar and its loopback API client.
type Supervisor struct {
	mu sync.Mutex

	binaryPath                string
	configDir                 string
	logPath                   string
	deviceName                string
	audioBackend              string
	audioDevice               string
	bitrate                   int
	crossfadeDurationMS       int
	normalisationDisabled     bool
	normalisationUseAlbumGain bool
	normalisationPregain      float32
	cacheEnabled              bool
	cacheDir                  string
	cacheSizeLimit            string
	flacEnabled               bool
	mprisEnabled              bool
	deviceIsGroup             bool
	imageSize                 string
	engineVersion             string
	versionAttempt            time.Time
	httpClient                *http.Client
	emit                      func(AudioState)

	client    *Client
	cmd       *exec.Cmd
	running   bool
	stopping  bool
	available bool
	lastError string
	done      chan struct{}
}

func NewSupervisor(options Options) *Supervisor {
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
	playback := NormalizePlaybackConfig(PlaybackConfig{
		Bitrate:                   options.Bitrate,
		CrossfadeDurationMS:       options.CrossfadeDurationMS,
		NormalisationDisabled:     options.NormalisationDisabled,
		NormalisationUseAlbumGain: options.NormalisationUseAlbumGain,
		NormalisationPregain:      options.NormalisationPregain,
		CacheEnabled:              options.CacheEnabled,
		CacheDir:                  options.CacheDir,
		CacheSizeLimit:            options.CacheSizeLimit,
		FlacEnabled:               options.FlacEnabled,
		MPRISEnabled:              options.MPRISEnabled,
		DeviceIsGroup:             options.DeviceIsGroup,
		ImageSize:                 options.ImageSize,
	})
	return &Supervisor{
		binaryPath:                binary,
		configDir:                 options.ConfigDir,
		logPath:                   options.LogPath,
		deviceName:                deviceName,
		audioBackend:              backend,
		audioDevice:               device,
		bitrate:                   playback.Bitrate,
		crossfadeDurationMS:       playback.CrossfadeDurationMS,
		normalisationDisabled:     playback.NormalisationDisabled,
		normalisationUseAlbumGain: playback.NormalisationUseAlbumGain,
		normalisationPregain:      playback.NormalisationPregain,
		cacheEnabled:              playback.CacheEnabled,
		cacheDir:                  playback.CacheDir,
		cacheSizeLimit:            playback.CacheSizeLimit,
		flacEnabled:               playback.FlacEnabled,
		mprisEnabled:              playback.MPRISEnabled,
		deviceIsGroup:             playback.DeviceIsGroup,
		imageSize:                 playback.ImageSize,
		httpClient:                options.HTTPClient,
		emit:                      options.Emit,
		client:                    options.Client,
		available:                 binary != "",
	}
}

func normalizeImageSize(value string) string {
	switch value {
	case "default", "small", "large", "xlarge":
		return value
	default:
		return "default"
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

func (r *Supervisor) Available() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.available
}

func (r *Supervisor) SetEmitter(emit func(AudioState)) {
	r.mu.Lock()
	r.emit = emit
	r.mu.Unlock()
}

func (r *Supervisor) Client() *Client {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.client
}

func (r *Supervisor) BinaryPath() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.binaryPath
}

func (r *Supervisor) ConfigDir() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.configDir
}

// Version returns the running engine version parsed from its log. It returns
// an empty string when the engine is not running, and retries while the cached
// value is empty. It never returns an error.
func (r *Supervisor) Version() string {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return ""
	}
	version := r.engineVersion
	r.mu.Unlock()
	if version != "" {
		return version
	}
	return r.loadEngineVersion()
}

func (r *Supervisor) loadEngineVersion() string {
	r.mu.Lock()
	logPath := r.logPath
	running := r.running
	r.mu.Unlock()
	if !running {
		return ""
	}
	version := readEngineVersion(logPath)
	if version == "" {
		return ""
	}
	r.mu.Lock()
	if r.engineVersion == "" {
		r.engineVersion = version
	}
	cached := r.engineVersion
	r.mu.Unlock()
	return cached
}

func readEngineVersion(logPath string) string {
	if logPath == "" {
		return ""
	}
	file, err := os.Open(logPath)
	if err != nil {
		return ""
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 8<<10))
	if err != nil {
		return ""
	}
	match := engineVersionPattern.FindSubmatch(data)
	if len(match) != 2 {
		return ""
	}
	return string(match[1])
}

// Start launches the engine. It is safe to call again after a failed start.
func (r *Supervisor) Start(ctx context.Context) error {
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
	r.engineVersion = ""
	r.versionAttempt = time.Time{}
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
				r.loadEngineVersion()
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

func (r *Supervisor) waitForExit(cmd *exec.Cmd, done chan struct{}) {
	err := cmd.Wait()
	r.mu.Lock()
	if r.cmd == cmd {
		r.running = false
		r.client = nil
		r.engineVersion = ""
		r.versionAttempt = time.Time{}
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
func (r *Supervisor) Stop(ctx context.Context) error {
	r.mu.Lock()
	if !r.running || r.cmd == nil || r.cmd.Process == nil {
		r.running = false
		r.client = nil
		r.engineVersion = ""
		r.versionAttempt = time.Time{}
		r.mu.Unlock()
		return nil
	}
	cmd := r.cmd
	done := r.done
	r.stopping = true
	r.running = false
	r.client = nil
	r.engineVersion = ""
	r.versionAttempt = time.Time{}
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
func (r *Supervisor) Restart(ctx context.Context) error {
	if err := r.Stop(ctx); err != nil {
		return err
	}
	return r.Start(ctx)
}

// ResetSession stops the engine, removes its credentials and starts pairing
// again.
func (r *Supervisor) ResetSession(ctx context.Context) error {
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
func (r *Supervisor) State(ctx context.Context) AudioState {
	r.mu.Lock()
	available := r.available
	running := r.running
	client := r.client
	lastError := r.lastError
	configDir := r.configDir
	engineVersion := r.engineVersion
	attemptVersion := running && engineVersion == "" &&
		(r.versionAttempt.IsZero() || time.Since(r.versionAttempt) >= engineVersionRetryInterval)
	if attemptVersion {
		r.versionAttempt = time.Now()
	}
	r.mu.Unlock()
	if attemptVersion {
		engineVersion = r.loadEngineVersion()
	}

	state := AudioState{
		Available:       available,
		Running:         running,
		Engine:          Name,
		EngineVersion:   engineVersion,
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

func (r *Supervisor) writeConfig(port int, configDir string) error {
	config := fmt.Sprintf(`# This file is generated by Cadence on every start; manual changes are overwritten.
log_level: info
device_name: %q
device_type: computer
device_is_group: %t
bitrate: %d
crossfade_duration: %d
normalisation_disabled: %t
normalisation_use_album_gain: %t
normalisation_pregain: %g
flac_enabled: %t
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
  image_size: %s
metadata:
  enabled: true
  context_sweep: true
  max_tracks: 800
mpris_enabled: %t
cache:
  enabled: %t
  dir: %q
  size_limit: %q
`,
		r.deviceName,
		r.deviceIsGroup,
		r.bitrate,
		r.crossfadeDurationMS,
		r.normalisationDisabled,
		r.normalisationUseAlbumGain,
		r.normalisationPregain,
		r.flacEnabled,
		r.audioBackend,
		r.audioDevice,
		port,
		r.imageSize,
		r.mprisEnabled,
		r.cacheEnabled,
		r.cacheDir,
		r.cacheSizeLimit,
	)
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

func (r *Supervisor) notify() {
	r.mu.Lock()
	emit := r.emit
	available := r.available
	running := r.running
	client := r.client
	lastError := r.lastError
	configDir := r.configDir
	engineVersion := r.engineVersion
	r.mu.Unlock()
	if emit == nil {
		return
	}
	state := AudioState{
		Available: available, Running: running, Engine: Name, EngineVersion: engineVersion,
		DeviceName: r.deviceName, AudioBackend: r.audioBackend, Error: lastError,
		BinaryPath: r.binaryPath, ConfigDirectory: configDir,
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
