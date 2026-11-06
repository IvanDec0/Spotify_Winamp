package webplayer

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"cadence/internal/artwork"
	"cadence/internal/webplayer/helper"

	"github.com/adrg/xdg"
)

const (
	// DefaultListenAddr is the fixed loopback port registered as the Spotify
	// redirect URI. Spotify rejects dynamic ports for practical dashboard
	// entries, so this port is part of the contract.
	DefaultListenAddr  = "127.0.0.1:8888"
	DefaultRedirectURI = "http://127.0.0.1:8888/callback"
	DefaultDeviceName  = "Cadence Web"

	defaultServiceHTTPTimeout = 30 * time.Second
)

// readyTimeout bounds how long EnsureStarted waits for the helper's ready
// event.
var readyTimeout = 2 * time.Minute

// ServiceOptions configures the web player service. Zero values select the
// production defaults; NewService performs no network access, starts no
// process and binds no socket.
type ServiceOptions struct {
	ClientID    string
	RedirectURI string
	ListenAddr  string
	Root        string
	ConfigDir   string
	DeviceName  string
	HTTPClient  *http.Client
	OpenURL     func(string) error
	Art         *artwork.Cache
	Now         func() time.Time
}

// Service owns the web player runtime, OAuth session and provider.
type Service struct {
	clientID    string
	redirectURI string
	listenAddr  string
	root        string
	configDir   string
	deviceName  string

	store    *TokenStore
	oauth    *OAuth
	api      *WebAPI
	runtime  *Runtime
	provider *Provider

	mu        sync.Mutex
	startMu   sync.Mutex
	serverMu  sync.Mutex
	server    *Server
	process   *Process
	control   *Control
	deviceID  string
	lastError string
}

// Status is the web player status shown by the settings UI. It never starts
// anything.
type Status struct {
	LoggedIn           bool
	Running            bool
	RuntimeInstalled   bool
	RuntimeVersion     string
	DeviceID           string
	Account            string
	LastError          string
	ClientID           string
	ConfiguredClientID string
}

var _ ProviderBackend = (*Service)(nil)

// NewService validates the options and builds the OAuth, Web API, runtime and
// provider objects. It does not bind the HTTP server, download the runtime or
// start the helper.
func NewService(opts ServiceOptions) (*Service, error) {
	if strings.TrimSpace(opts.RedirectURI) == "" {
		opts.RedirectURI = DefaultRedirectURI
	}
	if strings.TrimSpace(opts.ListenAddr) == "" {
		opts.ListenAddr = DefaultListenAddr
	}
	if strings.TrimSpace(opts.Root) == "" {
		opts.Root = defaultRoot()
	}
	if strings.TrimSpace(opts.ConfigDir) == "" {
		opts.ConfigDir = defaultConfigDir()
	}
	// An explicit Client ID wins (environment override and flags). Otherwise
	// the app configured by Settings is used, then the built-in public app.
	if strings.TrimSpace(opts.ClientID) == "" {
		opts.ClientID = LoadClientID(opts.ConfigDir)
	}
	if strings.TrimSpace(opts.ClientID) == "" {
		opts.ClientID = DefaultClientID
	}
	if strings.TrimSpace(opts.DeviceName) == "" {
		opts.DeviceName = DefaultDeviceName
	} else {
		opts.DeviceName = strings.TrimSpace(opts.DeviceName)
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: defaultServiceHTTPTimeout}
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}

	listenPort, err := validateLoopbackListenAddr(opts.ListenAddr)
	if err != nil {
		return nil, err
	}
	if err := validateRedirectURI(opts.RedirectURI, listenPort); err != nil {
		return nil, err
	}

	store := NewTokenStore(filepath.Join(opts.ConfigDir, "tokens.json"))
	oauth, err := NewOAuth(OAuthOptions{
		ClientID:    opts.ClientID,
		RedirectURI: opts.RedirectURI,
		Scopes:      DefaultScopes,
		Store:       store,
		HTTPClient:  client,
		OpenURL:     opts.OpenURL,
		Now:         opts.Now,
	})
	if err != nil {
		return nil, err
	}
	service := &Service{
		clientID:    opts.ClientID,
		redirectURI: opts.RedirectURI,
		listenAddr:  opts.ListenAddr,
		root:        opts.Root,
		configDir:   opts.ConfigDir,
		deviceName:  opts.DeviceName,
		store:       store,
		oauth:       oauth,
		api:         NewWebAPI(WebAPIOptions{Tokens: oauth, HTTPClient: client, Now: opts.Now}),
		runtime:     NewRuntime(RuntimeOptions{Root: opts.Root, HTTPClient: opts.HTTPClient, Now: opts.Now}),
	}
	service.provider = NewProvider(service, opts.Art)
	return service, nil
}

// Provider returns the playback provider backed by this service.
func (s *Service) Provider() *Provider {
	return s.provider
}

// ClientID returns the Spotify Client ID the service was built with. The
// helper and OAuth flow keep using it until the service is recreated.
func (s *Service) ClientID() string {
	return s.clientID
}

// ConfiguredClientID returns the Client ID stored by Settings, or "" when no
// valid file exists. It reads the file on every call so a newly configured app
// is visible without restarting the service.
func (s *Service) ConfiguredClientID() string {
	return LoadClientID(s.configDir)
}

// API returns the Spotify Web API client.
func (s *Service) API() *WebAPI {
	return s.api
}

// DeviceID returns the ephemeral SDK device id reported by the helper.
func (s *Service) DeviceID() string {
	s.mu.Lock()
	deviceID := s.deviceID
	process := s.process
	s.mu.Unlock()
	if process != nil && process.Exited() {
		return ""
	}
	return deviceID
}

// DeviceName returns the Spotify Connect device name for the helper.
func (s *Service) DeviceName() string {
	return s.deviceName
}

// LoggedIn reports whether a refreshable Spotify session is stored.
func (s *Service) LoggedIn() bool {
	return s.oauth.LoggedIn()
}

// Running reports whether the helper process is alive.
func (s *Service) Running() bool {
	s.mu.Lock()
	process := s.process
	s.mu.Unlock()
	return process != nil && !process.Exited()
}

// RuntimeInstalled reports whether a verified runtime is installed, without
// downloading or starting anything.
func (s *Service) RuntimeInstalled() bool {
	_, ok := s.runtime.Installed()
	return ok
}

// InstallRuntime downloads, verifies and installs the runtime and materializes
// the embedded helper. It starts no process and binds no socket.
func (s *Service) InstallRuntime(ctx context.Context) (Install, error) {
	install, err := s.runtime.Ensure(ctx)
	if err != nil {
		s.setLastError(err)
	} else {
		s.clearLastError()
	}
	return install, err
}

// EnsureStarted starts the helper when needed and waits for its ready event.
// It is lazy: it is only called by Play, PlayContext and Transfer(empty).
func (s *Service) EnsureStarted(ctx context.Context) error {
	if err := s.ensureStarted(ctx); err != nil {
		s.setLastError(err)
		return err
	}
	s.clearLastError()
	return nil
}

func (s *Service) ensureStarted(ctx context.Context) error {
	s.startMu.Lock()
	defer s.startMu.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	process := s.process
	s.mu.Unlock()
	if process != nil && !process.Exited() {
		return nil
	}
	if !s.oauth.LoggedIn() {
		return ErrLoginRequired
	}
	// Installing the runtime is an explicit action (InstallRuntime). Starting
	// the helper must never trigger a multi-minute download behind a 15 s
	// catalog action.
	if !s.RuntimeInstalled() {
		return ErrRuntimeNotInstalled
	}
	if err := s.ensureServer(); err != nil {
		return err
	}
	s.mu.Lock()
	server := s.server
	s.mu.Unlock()
	if server == nil {
		return codedError(CodeServerFailed, "the Spotify login server is not running", nil)
	}
	install, err := s.runtime.Ensure(ctx)
	if err != nil {
		return err
	}
	control, err := NewControl()
	if err != nil {
		return codedError(CodeRuntimeFailed, "cannot start the web player control socket", err)
	}
	helperProcess, err := s.runtime.Start(ctx, install, StartOptions{
		ControlURL: control.URL(),
		PlayerURL:  server.PlayerURL(),
	})
	if err != nil {
		_ = control.Close()
		return err
	}
	deviceID, err := waitForReady(ctx, helperProcess, control)
	if err != nil {
		logProcessTail(helperProcess)
		_ = control.Close()
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = helperProcess.Stop(stopCtx)
		cancel()
		return err
	}

	s.mu.Lock()
	s.process = helperProcess
	s.control = control
	s.deviceID = deviceID
	s.mu.Unlock()
	go s.watchProcess(helperProcess, control)
	return nil
}

// Login starts the Spotify OAuth flow through the system browser. It never
// starts the helper.
func (s *Service) Login(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.ensureServer(); err != nil {
		s.setLastError(err)
		return err
	}
	if err := s.oauth.Login(ctx); err != nil {
		s.setLastError(err)
		return err
	}
	s.clearLastError()
	// The profile label is a convenience: a failed fetch must not invalidate a
	// session that was just authorized.
	s.storeAccount(ctx)
	return nil
}

// storeAccount reads the Spotify profile and persists a display label. It is
// best effort and never exposes token values.
func (s *Service) storeAccount(ctx context.Context) {
	profileCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	profile, err := s.api.Me(profileCtx)
	if err != nil || profile == nil {
		return
	}
	account := strings.TrimSpace(profile.DisplayName)
	if account == "" {
		account = strings.TrimSpace(profile.Email)
	}
	if account == "" {
		account = strings.TrimSpace(profile.ID)
	}
	if account == "" {
		return
	}
	tokens, err := s.store.Load()
	if err != nil {
		return
	}
	tokens.Account = account
	_ = s.store.Save(tokens)
}

// Logout stops the helper, closes the local server and removes the stored
// session.
func (s *Service) Logout() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stopErr := s.Stop(ctx)
	logoutErr := s.oauth.Logout()
	s.clearLastError()
	return errors.Join(stopErr, logoutErr)
}

// Stop shuts the helper down cleanly, closes the control socket and the local
// HTTP server and clears the ephemeral device id. It is idempotent.
func (s *Service) Stop(ctx context.Context) error {
	s.startMu.Lock()
	defer s.startMu.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	process := s.process
	control := s.control
	server := s.server
	s.process = nil
	s.control = nil
	s.server = nil
	s.deviceID = ""
	s.mu.Unlock()

	var errs []error
	if control != nil {
		sendCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		if err := control.Send(sendCtx, Message{Type: "shutdown"}); err != nil && !errors.Is(err, ErrNotConnected) {
			errs = append(errs, err)
		}
		cancel()
	}
	if process != nil {
		waitCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		select {
		case <-process.Done():
		case <-waitCtx.Done():
		}
		cancel()
		errs = append(errs, process.Stop(ctx))
	}
	if control != nil {
		errs = append(errs, control.Close())
	}
	if server != nil {
		errs = append(errs, server.Close())
	}
	return errors.Join(errs...)
}

// Status returns the current status without starting or downloading anything.
func (s *Service) Status() Status {
	var status Status
	// One load feeds both fields; a corrupt or missing store means signed out.
	// A session from another Client ID is invalid until the user reconnects.
	if tokens, err := s.store.Load(); err == nil {
		status.LoggedIn = tokens.RefreshToken != "" && tokens.ClientID == s.clientID
		if status.LoggedIn {
			status.Account = tokens.Account
		}
	}
	if install, ok := s.runtime.Installed(); ok {
		status.RuntimeInstalled = true
		status.RuntimeVersion = install.Version
	}
	status.ClientID = s.clientID
	status.ConfiguredClientID = s.ConfiguredClientID()
	s.mu.Lock()
	process := s.process
	deviceID := s.deviceID
	lastError := s.lastError
	s.mu.Unlock()
	if process != nil && !process.Exited() {
		status.Running = true
		status.DeviceID = deviceID
	}
	status.LastError = lastError
	return status
}

func (s *Service) setLastError(err error) {
	s.mu.Lock()
	s.lastError = CodeOf(err)
	s.mu.Unlock()
}

func (s *Service) clearLastError() {
	s.mu.Lock()
	s.lastError = ""
	s.mu.Unlock()
}

func (s *Service) ensureServer() error {
	s.serverMu.Lock()
	defer s.serverMu.Unlock()
	s.mu.Lock()
	existing := s.server
	s.mu.Unlock()
	if existing != nil {
		return nil
	}
	server, err := NewServer(ServerOptions{
		Addr:     s.listenAddr,
		Page:     helper.Page(),
		Script:   helper.Script(),
		Tokens:   s.oauth,
		Callback: s.oauth,
	})
	if err != nil {
		return codedError(
			CodeServerFailed,
			fmt.Sprintf("cannot start the Spotify login server on %s; the fixed Spotify login port may be in use", s.listenAddr),
			err,
		)
	}
	s.mu.Lock()
	s.server = server
	s.mu.Unlock()
	return nil
}

// watchProcess clears the ephemeral device id when the helper exits and keeps
// the redacted tail for diagnostics.
func (s *Service) watchProcess(process *Process, control *Control) {
	<-process.Done()
	s.mu.Lock()
	if s.process == process {
		s.process = nil
		s.control = nil
		s.deviceID = ""
	}
	s.mu.Unlock()
	_ = control.Close()
	logProcessTail(process)
}

func waitForReady(ctx context.Context, process *Process, control *Control) (string, error) {
	timer := time.NewTimer(readyTimeout)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timer.C:
			return "", codedError(CodeRuntimeFailed, "the web player helper did not become ready in time", nil)
		case <-process.Done():
			return "", codedError(CodeRuntimeFailed, "the web player helper exited before it was ready", nil)
		case message, ok := <-control.Events():
			if !ok {
				return "", codedError(CodeRuntimeFailed, "the web player control socket closed", nil)
			}
			switch message.Type {
			case "ready":
				if message.DeviceID == "" {
					continue
				}
				return message.DeviceID, nil
			case "error":
				return "", readyError(message)
			}
		}
	}
}

func readyError(message Message) error {
	switch message.Code {
	case "widevine":
		return codedError(CodeWidevine, "Widevine is not available in the web player runtime", nil)
	case "authentication_error":
		return ErrLoginRequired
	default:
		return codedError(CodeRuntimeFailed, "the web player helper reported an error", nil)
	}
}

func logProcessTail(process *Process) {
	if process == nil {
		return
	}
	for _, line := range process.Tail() {
		log.Printf("webplayer helper: %s", line)
	}
}

func validateLoopbackListenAddr(raw string) (int, error) {
	host, port, err := net.SplitHostPort(strings.TrimSpace(raw))
	if err != nil {
		return 0, codedError(CodeInvalidArgument, "the web player listen address is not a host:port pair", err)
	}
	if host != "127.0.0.1" {
		return 0, codedError(CodeInvalidArgument, "the web player listen address must be 127.0.0.1", nil)
	}
	value, err := strconv.Atoi(port)
	if err != nil || value <= 0 || value > 65535 {
		return 0, codedError(CodeInvalidArgument, "the web player listen address must have a valid port", nil)
	}
	return value, nil
}

func validateRedirectURI(raw string, listenPort int) error {
	target, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return codedError(CodeInvalidArgument, "the Spotify redirect URI is not a valid URL", err)
	}
	if !strings.EqualFold(target.Scheme, "http") {
		return codedError(CodeInvalidArgument, "the Spotify redirect URI must use http", nil)
	}
	if target.Hostname() != "127.0.0.1" {
		return codedError(CodeInvalidArgument, "the Spotify redirect URI must use 127.0.0.1", nil)
	}
	if target.User != nil {
		return codedError(CodeInvalidArgument, "the Spotify redirect URI must not embed credentials", nil)
	}
	if target.Path != "/callback" {
		return codedError(CodeInvalidArgument, "the Spotify redirect URI must end in /callback", nil)
	}
	if target.RawQuery != "" || target.Fragment != "" {
		return codedError(CodeInvalidArgument, "the Spotify redirect URI must not carry a query or fragment", nil)
	}
	port, err := strconv.Atoi(target.Port())
	if err != nil || port != listenPort {
		return codedError(CodeInvalidArgument, "the Spotify redirect URI port must match the listen address", nil)
	}
	return nil
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
	return filepath.Join(base, "cadence", "webplayer")
}
