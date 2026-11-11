package developer

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"cadence/internal/developer/helper"
	"cadence/internal/webplayer"

	"github.com/adrg/xdg"
)

const (
	// actionTimeout bounds one dashboard action. The helper has its own
	// ten-minute backstop; this timeout also covers runtime preparation.
	actionTimeout = 10 * time.Minute

	clientIDFileMode = 0o600
	directoryMode    = 0o700
	configDirMode    = 0o700
)

var developerClientIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// Options configures the developer automation service. Zero values select the
// production XDG paths. NewService starts no process and performs no network
// access.
type Options struct {
	// Runtime is the Chromium runtime used by the web player. When nil, a
	// runtime with the standard web player cache root is created.
	Runtime *webplayer.Runtime
	// Root is the cache root for the developer helper app.
	Root string
	// ConfigDir is Cadence's config root, not the web player subdirectory.
	ConfigDir string
	// ProfileDir is the persistent dashboard browser profile.
	ProfileDir string
	// LogPath is the redacted helper log.
	LogPath string
	Now     func() time.Time
}

// Service automates the Spotify Developer dashboard. Actions are serialized:
// one hidden helper runs at a time and is stopped before the action returns.
type Service struct {
	runtime    *webplayer.Runtime
	root       string
	configDir  string
	profileDir string
	logPath    string
	now        func() time.Time

	actionMu sync.Mutex

	mu      sync.Mutex
	process *webplayer.Process
	control *webplayer.Control
}

// NewService validates the options and prepares the persistent browser
// profile. It never launches the helper.
func NewService(opts Options) (*Service, error) {
	if strings.TrimSpace(opts.Root) == "" {
		opts.Root = defaultRoot()
	}
	if strings.TrimSpace(opts.ConfigDir) == "" {
		opts.ConfigDir = defaultConfigDir()
	}
	if strings.TrimSpace(opts.ProfileDir) == "" {
		opts.ProfileDir = filepath.Join(opts.ConfigDir, "developer", "profile")
	}
	if strings.TrimSpace(opts.LogPath) == "" {
		opts.LogPath = defaultLogPath()
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Runtime == nil {
		opts.Runtime = webplayer.NewRuntime(webplayer.RuntimeOptions{})
	}

	if err := os.MkdirAll(opts.ProfileDir, directoryMode); err != nil {
		return nil, fmt.Errorf("create the Spotify developer profile directory: %w", err)
	}
	if err := os.Chmod(opts.ProfileDir, directoryMode); err != nil {
		return nil, fmt.Errorf("secure the Spotify developer profile directory: %w", err)
	}

	return &Service{
		runtime:    opts.Runtime,
		root:       opts.Root,
		configDir:  opts.ConfigDir,
		profileDir: opts.ProfileDir,
		logPath:    opts.LogPath,
		now:        opts.Now,
	}, nil
}

// EnsureWebApp returns the Cadence Client ID, creating the Spotify Developer
// app when it does not exist, and stores it for the next web player start.
func (s *Service) EnsureWebApp(ctx context.Context) (string, error) {
	message, err := s.runHelper(ctx, "web-app", false)
	if err != nil {
		return "", err
	}
	clientID, err := parseWebApp(message)
	if err != nil {
		return "", err
	}
	if err := s.writeClientID(clientID); err != nil {
		return "", err
	}
	return clientID, nil
}

// EnsureSoloistKey returns a Soloist API key, creating or rotating it as
// requested. The caller owns the key; this service never stores or logs it.
func (s *Service) EnsureSoloistKey(ctx context.Context, rotate bool) (string, error) {
	message, err := s.runHelper(ctx, "soloist-key", rotate)
	if err != nil {
		return "", err
	}
	return parseSoloistKey(message)
}

// OpenOAuthURL drives the persistent dashboard profile to a Spotify
// authorization URL. It returns once the helper is running; the helper quits
// after Spotify redirects to the loopback callback or the login times out.
// The URL query carries only public PKCE material and is never logged.
func (s *Service) OpenOAuthURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if err := validateOAuthURL(raw); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
	defer cancel()

	s.actionMu.Lock()
	defer s.actionMu.Unlock()

	// Only one browser may use the persistent profile. Replacing a helper from
	// a previous attempt also avoids the single-instance lock rejecting this
	// launch.
	_ = s.Stop(context.Background())

	if err := helper.Materialize(filepath.Join(s.root, "helper")); err != nil {
		return codedError(CodeFailed, "cannot prepare the Spotify developer helper", err)
	}
	install, err := s.runtime.Ensure(ctx)
	if err != nil {
		return codedError(CodeFailed, "cannot prepare the Spotify developer runtime", err)
	}

	// OAuth mode never uses the control socket. The helper skips connecting to
	// it, so the listener is closed immediately after the process starts.
	control, err := webplayer.NewControl()
	if err != nil {
		return codedError(CodeFailed, "cannot start the Spotify developer control socket", err)
	}
	process, err := s.runtime.Start(ctx, install, webplayer.StartOptions{
		ControlURL: control.URL(),
		AppDir:     filepath.Join(s.root, "helper"),
		ProfileDir: s.profileDir,
		LogPath:    s.logPath,
		ExtraArgs: []string{
			"--cadence-action=oauth",
			"--cadence-oauth-url=" + raw,
		},
	})
	_ = control.Close()
	if err != nil {
		return codedError(CodeFailed, "cannot start the Spotify developer helper", err)
	}

	s.mu.Lock()
	s.process = process
	s.control = nil
	s.mu.Unlock()
	go s.watchOAuthProcess(process)
	return nil
}

func validateOAuthURL(raw string) error {
	target, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return codedError(CodeFailed, "the Spotify authorize URL is not valid", err)
	}
	if !strings.EqualFold(target.Scheme, "https") {
		return codedError(CodeFailed, "the Spotify authorize URL must use https", nil)
	}
	if target.User != nil {
		return codedError(CodeFailed, "the Spotify authorize URL must not embed credentials", nil)
	}
	if !strings.EqualFold(target.Hostname(), "accounts.spotify.com") {
		return codedError(CodeFailed, "the Spotify authorize URL must use accounts.spotify.com", nil)
	}
	if target.Path != "/authorize" {
		return codedError(CodeFailed, "the Spotify authorize URL must use the /authorize path", nil)
	}
	return nil
}

// watchOAuthProcess clears the tracked OAuth helper after it quits. It never
// logs the process tail: an OAuth failure may carry a callback URL query.
func (s *Service) watchOAuthProcess(process *webplayer.Process) {
	<-process.Done()
	s.mu.Lock()
	if s.process == process {
		s.process = nil
		s.control = nil
	}
	s.mu.Unlock()
}

// parseWebApp validates a webApp control message and returns its Client ID.
func parseWebApp(message webplayer.Message) (string, error) {
	if message.Type != "webApp" {
		return "", codedError(CodeFailed, "the Spotify developer helper returned an unexpected result", nil)
	}
	clientID := strings.TrimSpace(message.ClientID)
	if !developerClientIDPattern.MatchString(clientID) {
		return "", codedError(CodeFailed, "the Spotify Developer dashboard returned an invalid Client ID", nil)
	}
	return clientID, nil
}

// parseSoloistKey validates a soloistKey control message and returns its key.
func parseSoloistKey(message webplayer.Message) (string, error) {
	if message.Type != "soloistKey" {
		return "", codedError(CodeFailed, "the Spotify developer helper returned an unexpected result", nil)
	}
	key := strings.TrimSpace(message.Key)
	if !validSoloistKey(key) {
		return "", codedError(CodeFailed, "the Spotify Developer dashboard returned an invalid Soloist API key", nil)
	}
	return key, nil
}

// Stop stops an in-flight helper, if any. Actions stop their helper before
// returning, so this is only a shutdown backstop.
func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	process := s.process
	control := s.control
	s.process = nil
	s.control = nil
	s.mu.Unlock()
	return stopHelper(ctx, process, control)
}

func (s *Service) runHelper(ctx context.Context, action string, rotate bool) (webplayer.Message, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, actionTimeout)
	defer cancel()

	s.actionMu.Lock()
	defer s.actionMu.Unlock()

	if err := helper.Materialize(filepath.Join(s.root, "helper")); err != nil {
		return webplayer.Message{}, codedError(CodeFailed, "cannot prepare the Spotify developer helper", err)
	}
	install, err := s.runtime.Ensure(ctx)
	if err != nil {
		return webplayer.Message{}, codedError(CodeFailed, "cannot prepare the Spotify developer runtime", err)
	}

	control, err := webplayer.NewControl()
	if err != nil {
		return webplayer.Message{}, codedError(CodeFailed, "cannot start the Spotify developer control socket", err)
	}
	defer func() { _ = control.Close() }()

	extraArgs := []string{"--cadence-action=" + action}
	if action == "soloist-key" {
		extraArgs = append(extraArgs, fmt.Sprintf("--cadence-rotate=%t", rotate))
	}
	process, err := s.runtime.Start(ctx, install, webplayer.StartOptions{
		ControlURL: control.URL(),
		AppDir:     filepath.Join(s.root, "helper"),
		ProfileDir: s.profileDir,
		LogPath:    s.logPath,
		ExtraArgs:  extraArgs,
	})
	if err != nil {
		return webplayer.Message{}, codedError(CodeFailed, "cannot start the Spotify developer helper", err)
	}
	s.mu.Lock()
	s.process = process
	s.control = control
	s.mu.Unlock()
	defer func() {
		_ = s.Stop(context.Background())
	}()

	message, err := waitForResult(ctx, action, process, control)
	if err != nil {
		return webplayer.Message{}, err
	}
	return message, nil
}

func waitForResult(
	ctx context.Context,
	action string,
	process *webplayer.Process,
	control *webplayer.Control,
) (webplayer.Message, error) {
	for {
		select {
		case <-ctx.Done():
			return webplayer.Message{}, ctx.Err()
		case <-process.Done():
			return webplayer.Message{}, codedError(
				CodeFailed,
				"the Spotify developer helper exited before reporting a result",
				nil,
			)
		case message, ok := <-control.Events():
			if !ok {
				return webplayer.Message{}, codedError(
					CodeFailed,
					"the Spotify developer control socket closed",
					nil,
				)
			}
			if message.Type == "error" {
				return webplayer.Message{}, helperError(message)
			}
			if action == "web-app" && message.Type == "webApp" {
				return message, nil
			}
			if action == "soloist-key" && message.Type == "soloistKey" {
				return message, nil
			}
		}
	}
}

func helperError(message webplayer.Message) error {
	switch message.Code {
	case "login_required":
		return ErrLoginRequired
	case "premium_required":
		return ErrPremiumRequired
	case "app_limit":
		return ErrAppLimit
	default:
		if strings.TrimSpace(message.Message) != "" {
			return codedError(CodeFailed, message.Message, nil)
		}
		return codedError(CodeFailed, "the Spotify Developer dashboard automation failed", nil)
	}
}

func stopHelper(ctx context.Context, process *webplayer.Process, control *webplayer.Control) error {
	if process == nil && control == nil {
		return nil
	}
	base := context.Background()
	if ctx != nil {
		base = context.WithoutCancel(ctx)
	}
	var errs []error
	if control != nil {
		sendCtx, cancel := context.WithTimeout(base, 2*time.Second)
		if err := control.Send(sendCtx, webplayer.Message{Type: "shutdown"}); err != nil && !errors.Is(err, webplayer.ErrNotConnected) {
			errs = append(errs, err)
		}
		cancel()
	}
	if process != nil {
		stopCtx, cancel := context.WithTimeout(base, 5*time.Second)
		errs = append(errs, process.Stop(stopCtx))
		cancel()
	}
	if control != nil {
		errs = append(errs, control.Close())
	}
	return errors.Join(errs...)
}

// writeClientID stores the public Client ID atomically with owner-only
// permissions in the web player config directory.
func (s *Service) writeClientID(clientID string) error {
	if !developerClientIDPattern.MatchString(clientID) {
		return codedError(CodeFailed, "the Spotify Client ID is not valid", nil)
	}
	dir := filepath.Join(s.configDir, "webplayer")
	if err := os.MkdirAll(dir, configDirMode); err != nil {
		return codedError(CodeFailed, "cannot create the web player config directory", err)
	}
	_ = os.Chmod(dir, configDirMode)

	temp, err := os.CreateTemp(dir, ".client-id-*.tmp")
	if err != nil {
		return codedError(CodeFailed, "cannot create the Client ID file", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)

	if err := temp.Chmod(clientIDFileMode); err != nil {
		_ = temp.Close()
		return codedError(CodeFailed, "cannot secure the Client ID file", err)
	}
	if _, err := temp.WriteString(clientID + "\n"); err != nil {
		_ = temp.Close()
		return codedError(CodeFailed, "cannot write the Client ID file", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return codedError(CodeFailed, "cannot flush the Client ID file", err)
	}
	if err := temp.Close(); err != nil {
		return codedError(CodeFailed, "cannot close the Client ID file", err)
	}
	if err := os.Rename(tempName, webplayer.ClientIDPath(dir)); err != nil {
		return codedError(CodeFailed, "cannot replace the Client ID file", err)
	}
	return nil
}

func validSoloistKey(key string) bool {
	if !strings.HasPrefix(key, "spak_") || len(key) <= len("spak_") || len(key) > 512 {
		return false
	}
	for _, character := range key {
		if character <= ' ' || character == 0x7f {
			return false
		}
	}
	return true
}

func defaultRoot() string {
	base := xdg.CacheHome
	if base == "" {
		if dir, err := os.UserCacheDir(); err == nil {
			base = dir
		}
	}
	if base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "cadence", "developer")
}

func defaultConfigDir() string {
	base := xdg.ConfigHome
	if base == "" {
		if dir, err := os.UserConfigDir(); err == nil {
			base = dir
		}
	}
	if base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "cadence")
}

func defaultLogPath() string {
	base := xdg.CacheHome
	if base == "" {
		if dir, err := os.UserCacheDir(); err == nil {
			base = dir
		}
	}
	if base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "cadence", "logs", "developer.log")
}
