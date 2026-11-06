package soloist

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"
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

// NewService discovers the installed binary and stored accounts.
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
	binary, err := FindInstalled(opts.ProgramsDir, os.Getenv("CADENCE_SOLOIST"))
	if err == nil {
		service.cacheBinary(binary)
	}
	return service
}

// State merges binary, account, key and supervisor state. It never includes
// the API key.
func (s *Service) State() State {
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
		if supervisorState.Installed {
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
		Binary:        binary,
		APIKey:        key,
		DeviceName:    s.opts.DeviceName,
		DataDir:       s.opts.DataDir,
		CacheDir:      s.opts.CacheDir,
		LogPath:       s.opts.LogPath,
		CacheSizeMB:   s.opts.CacheSizeMB,
		InitialVolume: s.opts.InitialVolume,
		Emit: func(State) {
			s.emitMerged()
		},
	})
	return s.supervisor
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

// Restart stops and starts Soloist.
func (s *Service) Restart(ctx context.Context) error {
	s.mu.Lock()
	supervisor := s.supervisor
	s.mu.Unlock()
	if supervisor == nil {
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
	s.accounts = Accounts(s.opts.DataDir)
	s.errorText = ""
	s.errorCode = ""
	s.mu.Unlock()
	s.emitMerged()
	return binary, nil
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

// Binary returns a copy of the cached installed binary, refreshing it when
// the file changed.
func (s *Service) Binary() *Binary {
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
