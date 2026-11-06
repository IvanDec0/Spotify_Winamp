package soloist

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// updateAge is how old an installed build may get before maintenance
	// replaces it. It runs before the 90-day build lifetime, so a successful
	// update is never racing the expiry.
	updateAge = 89 * 24 * time.Hour
	// staleTempAge bounds download and staging directories left behind by an
	// interrupted install. Fresh ones may belong to a running install.
	staleTempAge = 24 * time.Hour
)

// Options configures the Soloist service. All paths are injected; this
// package does not choose platform directories.
type Options struct {
	ProgramsDir   string
	DataDir       string
	CacheDir      string
	LogPath       string
	APIKeyPath    string
	DeviceName    string
	CacheSizeMB   int
	InitialVolume *int
	// EngineCredentials returns the library engine's paired account and its
	// reusable Spotify credentials for automatic Soloist pairing. Optional.
	EngineCredentials func() (username string, data []byte, err error)
	// Installer overrides the production installer. Nil builds an installer
	// for ProgramsDir with the production Spotify-only HTTP client. Tests use
	// this to inject an archive server without exposing a bypass on the
	// service surface.
	Installer *Installer
	Emit      func(State)
	Now       func() time.Time
}

// Service is the facade used by the application. It merges install, binary,
// credential and supervisor state without exposing the API key.
type Service struct {
	mu sync.Mutex
	// refreshMu serializes binary inspections without holding mu, so State
	// can never block Start, Stop or Install for the 15-second version probe.
	refreshMu sync.Mutex

	opts       Options
	now        func() time.Time
	installer  *Installer
	inspect    func(context.Context, string) (*Binary, error)
	binary     *Binary
	binarySig  binarySignature
	discovered bool
	accounts   []string
	supervisor *Supervisor
	emit       func(State)
	errorText  string
	errorCode  string
}

type binarySignature struct {
	path    string
	modTime time.Time
	size    int64
}

// NewService prepares the facade. Binary discovery is lazy: app construction
// must never run `soloist --version`, because a slow or hung installed binary
// would otherwise block startup before the window appears.
func NewService(opts Options) *Service {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	service := &Service{
		opts:     opts,
		now:      now,
		accounts: Accounts(opts.DataDir),
		inspect:  InspectBinary,
		emit:     opts.Emit,
	}
	if opts.Installer != nil {
		service.installer = opts.Installer
	} else {
		service.installer = NewInstaller(InstallerOptions{
			ProgramsDir: opts.ProgramsDir,
			Now:         now,
		})
	}
	return service
}

// discover locates the installed binary once. It takes refreshMu with TryLock
// so a concurrent version probe is never waited on, and caches "not found" so
// a missing install does not re-probe on every state read.
func (s *Service) discover() {
	if !s.refreshMu.TryLock() {
		return
	}
	defer s.refreshMu.Unlock()

	s.mu.Lock()
	if s.discovered {
		s.mu.Unlock()
		return
	}
	opts := s.opts
	s.mu.Unlock()

	binary, err := FindInstalled(opts.ProgramsDir, os.Getenv("CADENCE_SOLOIST"))
	s.mu.Lock()
	defer s.mu.Unlock()
	if err == nil {
		s.cacheBinaryLocked(binary)
	}
	s.discovered = true
}

// State merges binary, account, key and supervisor state. It never includes
// the API key.
func (s *Service) State() State {
	s.discover()
	s.refreshBinary()
	s.mu.Lock()
	binary := s.binary
	accounts := append([]string(nil), s.accounts...)
	supervisor := s.supervisor
	errorText := s.errorText
	errorCode := s.errorCode
	now := s.now()
	s.mu.Unlock()

	var state State
	state.Accounts = accounts
	state.Error = errorText
	state.ErrorCode = errorCode
	// The service-level binary wins over the supervisor's copy: after an
	// install the supervisor may still hold the build it was started with,
	// while Settings must show the newest one.
	if binary != nil {
		state.Installed = true
		state.BinaryPath = binary.Path
		state.Version = binary.Version
		state.BuiltAt = binary.BuiltAt
		state.ExpiresAt = binary.ExpiresAt()
		state.Expired = binary.Expired(now)
		state.DaysLeft = binary.DaysLeft(now)
	}

	if supervisor != nil {
		supervisorState := supervisor.State()
		state.Running = supervisorState.Running
		state.LoggedIn = supervisorState.LoggedIn
		state.NeedsPairing = supervisorState.NeedsPairing
		state.Active = supervisorState.Active
		state.DeviceName = supervisorState.DeviceName
		if len(supervisorState.Accounts) > 0 {
			state.Accounts = supervisorState.Accounts
		}
		if binary == nil && supervisorState.Installed {
			state.Installed = true
			state.BinaryPath = supervisorState.BinaryPath
			state.Version = supervisorState.Version
			state.BuiltAt = supervisorState.BuiltAt
			state.ExpiresAt = supervisorState.BuiltAt.Add(buildLifetime)
			// DaysLeft must follow the service clock, not the wall clock,
			// so tests and callers with an injected Now see one truth.
			merged := &Binary{Path: state.BinaryPath, Version: state.Version, BuiltAt: state.BuiltAt}
			state.Expired = merged.Expired(now)
			state.DaysLeft = merged.DaysLeft(now)
		}
		if supervisorState.Error != "" {
			state.Error = supervisorState.Error
		}
		if supervisorState.ErrorCode != "" {
			state.ErrorCode = supervisorState.ErrorCode
		}
	}
	if state.DeviceName == "" {
		state.DeviceName = s.opts.DeviceName
	}
	if state.Accounts == nil {
		state.Accounts = []string{}
	}
	return state
}

// Start refreshes the install, checks the key and starts Soloist. A pairing
// error is returned while the process keeps running.
func (s *Service) Start(ctx context.Context) error {
	s.discover()
	s.refreshBinary()
	s.mu.Lock()
	binary := s.binary
	opts := s.opts
	now := s.now()
	s.mu.Unlock()

	if binary == nil {
		return s.fail(ErrNotInstalled)
	}
	if binary.Expired(now) {
		return s.fail(ErrExpired)
	}
	key, err := ReadAPIKey(opts.APIKeyPath)
	if err != nil {
		return s.fail(err)
	}
	if err := ValidateAPIKey(key); err != nil {
		return s.fail(err)
	}

	supervisor := s.ensureSupervisor(binary, key)
	err = supervisor.Start(ctx)
	if err == nil {
		state := supervisor.State()
		if state.Running && state.NeedsPairing {
			err = ErrPairingRequired
		}
	}
	if err != nil {
		if CodeOf(err) == CodePairingRequired {
			s.refreshAccounts()
			s.emitMerged()
			return err
		}
		return s.fail(err)
	}
	s.clearError()
	s.refreshAccounts()
	s.emitMerged()
	return nil
}

func (s *Service) ensureSupervisor(binary *Binary, key string) *Supervisor {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.supervisor != nil {
		state := s.supervisor.State()
		if state.Running || (binary != nil && state.BinaryPath == binary.Path) {
			return s.supervisor
		}
	}
	s.supervisor = NewSupervisor(SupervisorOptions{
		Binary:            binary,
		APIKey:            key,
		DeviceName:        s.opts.DeviceName,
		DataDir:           s.opts.DataDir,
		CacheDir:          s.opts.CacheDir,
		LogPath:           s.opts.LogPath,
		CacheSizeMB:       s.opts.CacheSizeMB,
		InitialVolume:     s.opts.InitialVolume,
		EngineCredentials: s.opts.EngineCredentials,
		Emit: func(State) {
			s.emitMerged()
		},
	})
	return s.supervisor
}

// RetryPairing attempts the automatic library-account handoff on the
// running Soloist instance.
func (s *Service) RetryPairing(ctx context.Context) error {
	s.mu.Lock()
	supervisor := s.supervisor
	s.mu.Unlock()
	if supervisor == nil {
		return nil
	}
	return supervisor.RetryAutoPair(ctx)
}

// Stop stops Soloist.
func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	supervisor := s.supervisor
	s.mu.Unlock()
	if supervisor != nil {
		if err := supervisor.Stop(ctx); err != nil {
			return s.fail(err)
		}
	}
	s.clearError()
	s.refreshAccounts()
	s.emitMerged()
	return nil
}

// Restart stops and starts Soloist. When a new build was installed while the
// old supervisor was stopped, the old supervisor is discarded so the restart
// uses the new binary.
func (s *Service) Restart(ctx context.Context) error {
	s.mu.Lock()
	supervisor := s.supervisor
	binary := s.binary
	s.mu.Unlock()
	if supervisor == nil {
		return s.Start(ctx)
	}
	if binary != nil && supervisor.State().BinaryPath != binary.Path {
		if err := supervisor.Stop(ctx); err != nil {
			return s.fail(err)
		}
		s.mu.Lock()
		if s.supervisor == supervisor {
			s.supervisor = nil
		}
		s.mu.Unlock()
		return s.Start(ctx)
	}
	err := supervisor.Restart(ctx)
	if err != nil {
		if CodeOf(err) == CodePairingRequired {
			s.refreshAccounts()
			s.emitMerged()
			return err
		}
		return s.fail(err)
	}
	s.clearError()
	s.refreshAccounts()
	s.emitMerged()
	return nil
}

// Install downloads and installs the newest Soloist build.
func (s *Service) Install(ctx context.Context, progress func(Progress)) (*Binary, error) {
	s.mu.Lock()
	installer := s.installer
	s.mu.Unlock()
	binary, err := installer.Install(ctx, progress)
	if err != nil {
		return nil, s.fail(err)
	}
	s.mu.Lock()
	s.cacheBinaryLocked(binary)
	s.discovered = true
	s.accounts = Accounts(s.opts.DataDir)
	s.errorText = ""
	s.errorCode = ""
	supervisor := s.supervisor
	s.mu.Unlock()
	// A stopped supervisor still points at the previous build. Drop it so the
	// next start builds a supervisor for the freshly installed binary.
	if supervisor != nil && !supervisor.State().Running {
		s.mu.Lock()
		if s.supervisor == supervisor {
			s.supervisor = nil
		}
		s.mu.Unlock()
	}
	s.emitMerged()
	return binary, nil
}

// UpdateDue reports whether the installed build is missing, at least 89 days
// old, or within a day of expiry. It never downloads anything.
func (s *Service) UpdateDue(now time.Time) bool {
	s.discover()
	s.mu.Lock()
	binary := s.binary
	s.mu.Unlock()
	return updateDue(binary, now)
}

func updateDue(binary *Binary, now time.Time) bool {
	if binary == nil {
		return true
	}
	if now.Sub(binary.BuiltAt) >= updateAge {
		return true
	}
	// "Within a day" is the real remaining lifetime, not the floored
	// DaysLeft: 25 hours left is still more than a day.
	return binary.ExpiresAt().Sub(now) <= 24*time.Hour
}

// AutoUpdate installs Soloist when missing and updates it when due. It
// returns the current/new binary and whether a different build was installed.
// A fresh build makes no network request.
func (s *Service) AutoUpdate(ctx context.Context, progress func(Progress)) (*Binary, bool, error) {
	s.discover()
	s.mu.Lock()
	current := s.binary
	s.mu.Unlock()
	if !updateDue(current, s.now()) {
		return current, false, nil
	}

	next, err := s.Install(ctx, progress)
	if err != nil {
		return nil, false, err
	}
	updated := current == nil || !next.BuiltAt.Equal(current.BuiltAt) || next.Path != current.Path
	// Pruning is best effort: the install result is what the caller acts on.
	_ = s.Prune()
	return next, updated, nil
}

// Prune keeps the newest installed build and the build a supervisor was
// started with, removing older versioned directories and temp directories left
// behind by interrupted installs. Removal failures are ignored; only an
// unreadable programs directory is returned.
func (s *Service) Prune() error {
	s.mu.Lock()
	programsDir := s.opts.ProgramsDir
	supervisor := s.supervisor
	s.mu.Unlock()
	if programsDir == "" {
		return nil
	}

	entries, err := os.ReadDir(programsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	keep := make(map[string]bool)
	if path := newestInstalledPath(programsDir); path != "" {
		keep[filepath.Clean(filepath.Dir(path))] = true
	}
	if supervisor != nil {
		if path := supervisor.State().BinaryPath; path != "" {
			keep[filepath.Clean(filepath.Dir(path))] = true
		}
	}

	now := s.now()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		full := filepath.Join(programsDir, entry.Name())
		if keep[filepath.Clean(full)] {
			continue
		}
		if _, err := strconv.ParseInt(entry.Name(), 10, 64); err == nil {
			_ = os.RemoveAll(full)
			continue
		}
		if !strings.HasPrefix(entry.Name(), ".soloist-download-") &&
			!strings.Contains(entry.Name(), ".tmp-") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if now.Sub(info.ModTime()) >= staleTempAge {
			_ = os.RemoveAll(full)
		}
	}
	return nil
}

// SaveAPIKey stores the user's Soloist API key with owner-only permissions.
func (s *Service) SaveAPIKey(key string) error {
	key = strings.TrimSpace(key)
	s.mu.Lock()
	path := s.opts.APIKeyPath
	s.mu.Unlock()
	if err := SaveAPIKey(path, key); err != nil {
		return s.fail(err)
	}
	s.updateSupervisorKey(key)
	s.clearError()
	s.emitMerged()
	return nil
}

// ClearAPIKey removes the stored Soloist API key.
func (s *Service) ClearAPIKey() error {
	s.mu.Lock()
	path := s.opts.APIKeyPath
	s.mu.Unlock()
	if err := ClearAPIKey(path); err != nil {
		return s.fail(err)
	}
	s.updateSupervisorKey("")
	s.emitMerged()
	return nil
}

// updateSupervisorKey keeps the next start consistent with the key file
// without touching the running process.
func (s *Service) updateSupervisorKey(key string) {
	s.mu.Lock()
	supervisor := s.supervisor
	s.mu.Unlock()
	if supervisor != nil {
		supervisor.setAPIKey(key)
	}
}

// HasAPIKey reports whether a non-empty key file is present. It never returns
// the key.
func (s *Service) HasAPIKey() bool {
	s.mu.Lock()
	path := s.opts.APIKeyPath
	s.mu.Unlock()
	_, err := ReadAPIKey(path)
	return err == nil
}

// Client returns the running Soloist client, or nil.
func (s *Service) Client() *Client {
	s.mu.Lock()
	supervisor := s.supervisor
	s.mu.Unlock()
	if supervisor == nil || !supervisor.State().Running {
		return nil
	}
	return supervisor.Client()
}

// Binary returns a copy of the cached installed binary, discovering or
// refreshing it on first use.
func (s *Service) Binary() *Binary {
	s.discover()
	s.refreshBinary()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.binary == nil {
		return nil
	}
	copied := *s.binary
	return &copied
}

// SetEmitter replaces the state emitter.
func (s *Service) SetEmitter(emit func(State)) {
	s.mu.Lock()
	s.emit = emit
	supervisor := s.supervisor
	s.mu.Unlock()
	if supervisor != nil {
		supervisor.SetEmitter(func(State) {
			s.emitMerged()
		})
	}
}

// refreshBinary re-inspects the cached binary when its path, modification
// time or size changed. The version probe runs without s.mu, so State can
// never block a concurrent Start, Stop or Install.
func (s *Service) refreshBinary() {
	// If another goroutine is already probing --version, use the cached
	// binary instead of waiting: callers of State, Start, Stop and Install
	// must never be delayed by the version probe.
	if !s.refreshMu.TryLock() {
		return
	}
	defer s.refreshMu.Unlock()

	s.mu.Lock()
	binary := s.binary
	sig := s.binarySig
	inspect := s.inspect
	s.mu.Unlock()
	if binary == nil {
		return
	}
	info, err := os.Stat(binary.Path)
	if err != nil {
		s.mu.Lock()
		if s.binary == binary {
			s.binary = nil
			s.binarySig = binarySignature{}
		}
		s.mu.Unlock()
		return
	}
	current := binarySignature{path: binary.Path, modTime: info.ModTime(), size: info.Size()}
	if current == sig {
		return
	}
	if inspect == nil {
		inspect = InspectBinary
	}
	refreshed, err := inspect(context.Background(), binary.Path)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.binary != binary {
		// A concurrent Install replaced the cached build.
		return
	}
	if err != nil {
		s.binary = nil
		s.binarySig = binarySignature{}
		return
	}
	s.binary = refreshed
	s.binarySig = current
}

func (s *Service) cacheBinary(binary *Binary) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cacheBinaryLocked(binary)
}

func (s *Service) cacheBinaryLocked(binary *Binary) {
	s.binary = binary
	s.binarySig = binarySignature{}
	if binary == nil {
		return
	}
	if info, err := os.Stat(binary.Path); err == nil {
		s.binarySig = binarySignature{path: binary.Path, modTime: info.ModTime(), size: info.Size()}
	}
}

func (s *Service) refreshAccounts() {
	s.mu.Lock()
	s.accounts = Accounts(s.opts.DataDir)
	s.mu.Unlock()
}

func (s *Service) clearError() {
	s.mu.Lock()
	s.errorText = ""
	s.errorCode = ""
	s.mu.Unlock()
}

func (s *Service) fail(err error) error {
	if err == nil {
		return nil
	}
	code := CodeOf(err)
	if code == "" {
		code = CodeStartFailed
	}
	s.mu.Lock()
	s.errorCode = code
	s.errorText = err.Error()
	s.mu.Unlock()
	s.emitMerged()
	return err
}

func (s *Service) emitMerged() {
	s.mu.Lock()
	emit := s.emit
	s.mu.Unlock()
	if emit == nil {
		return
	}
	emit(s.State())
}
