package audioapi

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"cadence/internal/webplayer"
)

// fakeWebBackend is a mutable, call-recording WebBackend. It never touches the
// network, the filesystem or a real runtime process.
type fakeWebBackend struct {
	mu sync.Mutex

	status             webplayer.Status
	clientID           string
	configuredClientID string
	keepAvailable      bool
	loginErr           error
	logoutErr          error
	installErr         error
	startErr           error
	stopErr            error

	loginCalls      int
	logoutCalls     int
	installCalls    int
	startCalls      int
	stopCalls       int
	statusCalls     int
	invalidContexts int
	order           []string
}

func (f *fakeWebBackend) recordCall(name string) {
	f.order = append(f.order, name)
}

func (f *fakeWebBackend) record(ctx context.Context) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if ctx == nil || ctx.Err() != nil {
		f.invalidContexts++
	}
}

func (f *fakeWebBackend) Login(ctx context.Context) error {
	f.record(ctx)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.loginCalls++
	f.recordCall("login")
	if f.loginErr == nil {
		f.status.LoggedIn = true
	}
	return f.loginErr
}

func (f *fakeWebBackend) Logout() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logoutCalls++
	return f.logoutErr
}

func (f *fakeWebBackend) InstallRuntime(ctx context.Context) (webplayer.Install, error) {
	f.record(ctx)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.installCalls++
	f.recordCall("install")
	if f.installErr == nil {
		f.status.RuntimeInstalled = true
		f.status.RuntimeVersion = "1.2.3"
	}
	return webplayer.Install{Version: "1.2.3"}, f.installErr
}

func (f *fakeWebBackend) EnsureStarted(ctx context.Context) error {
	f.record(ctx)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.startCalls++
	f.recordCall("start")
	if f.startErr == nil {
		f.status.Running = true
	}
	return f.startErr
}

func (f *fakeWebBackend) Stop(ctx context.Context) error {
	f.record(ctx)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopCalls++
	f.recordCall("stop")
	if f.stopErr == nil {
		f.status.Running = false
	}
	return f.stopErr
}

func (f *fakeWebBackend) Status() webplayer.Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statusCalls++
	return f.status
}

func (f *fakeWebBackend) ClientID() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.clientID
}

func (f *fakeWebBackend) ConfiguredClientID() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.configuredClientID
}

func (f *fakeWebBackend) SetClientID(value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.clientID = value
	f.configuredClientID = value
	f.recordCall("set-client")
	return nil
}

func (f *fakeWebBackend) SetKeepAvailable(value bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.keepAvailable = value
}

func (f *fakeWebBackend) KeepAvailable() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.keepAvailable
}

func (f *fakeWebBackend) calls() (login, logout, install, start, stop int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.loginCalls, f.logoutCalls, f.installCalls, f.startCalls, f.stopCalls
}

func (f *fakeWebBackend) callOrder() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.order...)
}

func newWebServiceForTest(fake WebBackend) *Service {
	return NewService(Options{Web: fake})
}

func TestGetWebPlayerStatusMapsFields(t *testing.T) {
	fake := &fakeWebBackend{status: webplayer.Status{
		LoggedIn:         true,
		Running:          true,
		RuntimeInstalled: true,
		RuntimeVersion:   "44.9.3",
		DeviceID:         "device-123",
		Account:          "Listener",
		LastError:        webplayer.CodeRateLimited,
		ClientID:         "7fe450922d8b4d31ac8bda4a1767d903",
	}}
	service := newWebServiceForTest(fake)

	status := service.GetWebPlayerStatus()
	if !status.Available {
		t.Fatal("Available = false with a configured backend")
	}
	if !status.LoggedIn || !status.Running || !status.RuntimeInstalled {
		t.Fatalf("mapped status = %#v", status)
	}
	if status.RuntimeVersion != "44.9.3" || status.DeviceID != "device-123" {
		t.Fatalf("mapped status = %#v", status)
	}
	if status.Account != "Listener" || status.LastError != webplayer.CodeRateLimited {
		t.Fatalf("mapped account/error = %#v", status)
	}
	if status.ClientID != "7fe450922d8b4d31ac8bda4a1767d903" {
		t.Fatalf("mapped ClientID = %q", status.ClientID)
	}
	if status.RestartRequired {
		t.Fatal("RestartRequired = true when the configured file matches the running app")
	}
}

func TestGetWebPlayerStatusFlagsConfiguredClientIDChange(t *testing.T) {
	fake := &fakeWebBackend{status: webplayer.Status{
		ClientID:           "1c6adf01cc01423d99ff7e87682c057a",
		ConfiguredClientID: "7fe450922d8b4d31ac8bda4a1767d903",
	}}
	service := newWebServiceForTest(fake)
	status := service.GetWebPlayerStatus()
	if !status.RestartRequired {
		t.Fatal("RestartRequired = false when the configured app differs from the running app")
	}
	if status.ClientID != "1c6adf01cc01423d99ff7e87682c057a" {
		t.Fatalf("ClientID = %q", status.ClientID)
	}
}

func TestWebPlayerActionsCallTheBackend(t *testing.T) {
	fake := &fakeWebBackend{status: webplayer.Status{LoggedIn: true, RuntimeInstalled: true}}
	service := newWebServiceForTest(fake)

	if status, err := service.ConnectWebPlayer(); err != nil || !status.Available {
		t.Fatalf("ConnectWebPlayer() = %#v, %v", status, err)
	}
	if status, err := service.DisconnectWebPlayer(); err != nil || !status.Available {
		t.Fatalf("DisconnectWebPlayer() = %#v, %v", status, err)
	}
	if status, err := service.InstallWebPlayerRuntime(); err != nil || !status.Available {
		t.Fatalf("InstallWebPlayerRuntime() = %#v, %v", status, err)
	}
	if status, err := service.StartWebPlayer(); err != nil || !status.Available {
		t.Fatalf("StartWebPlayer() = %#v, %v", status, err)
	}
	if status, err := service.StopWebPlayer(); err != nil || !status.Available {
		t.Fatalf("StopWebPlayer() = %#v, %v", status, err)
	}

	login, logout, install, start, stop := fake.calls()
	if login != 1 || logout != 1 || install != 1 || start != 1 || stop != 1 {
		t.Fatalf("calls = login:%d logout:%d install:%d start:%d stop:%d", login, logout, install, start, stop)
	}
}

func TestWebPlayerActionsUseNonNilContexts(t *testing.T) {
	fake := &fakeWebBackend{}
	service := newWebServiceForTest(fake)

	for name, action := range map[string]func() (WebPlayerStatus, error){
		"connect": service.ConnectWebPlayer,
		"install": service.InstallWebPlayerRuntime,
		"start":   service.StartWebPlayer,
		"stop":    service.StopWebPlayer,
	} {
		if _, err := action(); err != nil {
			t.Fatalf("%s error = %v", name, err)
		}
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.invalidContexts != 0 {
		t.Fatalf("actions received %d nil or cancelled contexts", fake.invalidContexts)
	}
}

func TestWebPlayerActionsReturnStatusWithErrors(t *testing.T) {
	actionErr := errors.New("spotify is not connected")
	fake := &fakeWebBackend{
		status:     webplayer.Status{LastError: webplayer.CodeLoginRequired},
		loginErr:   actionErr,
		startErr:   actionErr,
		stopErr:    actionErr,
		installErr: actionErr,
	}
	service := newWebServiceForTest(fake)

	for name, action := range map[string]func() (WebPlayerStatus, error){
		"connect": service.ConnectWebPlayer,
		"install": service.InstallWebPlayerRuntime,
		"start":   service.StartWebPlayer,
		"stop":    service.StopWebPlayer,
	} {
		status, err := action()
		if !errors.Is(err, actionErr) {
			t.Fatalf("%s error = %v, want the backend error", name, err)
		}
		if !status.Available || status.LastError != webplayer.CodeLoginRequired {
			t.Fatalf("%s status = %#v, want the post-action status", name, status)
		}
	}

	fake.logoutErr = actionErr
	status, err := service.DisconnectWebPlayer()
	if !errors.Is(err, actionErr) {
		t.Fatalf("DisconnectWebPlayer() error = %v", err)
	}
	if !status.Available || status.LastError != webplayer.CodeLoginRequired {
		t.Fatalf("DisconnectWebPlayer() status = %#v", status)
	}
}

func TestWebPlayerWithoutBackendIsSafe(t *testing.T) {
	service := NewService(Options{})

	status := service.GetWebPlayerStatus()
	if status.Available {
		t.Fatal("Available = true without a backend")
	}
	if status.LoggedIn || status.Running || status.RuntimeInstalled {
		t.Fatalf("status = %#v", status)
	}

	for name, action := range map[string]func() (WebPlayerStatus, error){
		"connect":    service.ConnectWebPlayer,
		"disconnect": service.DisconnectWebPlayer,
		"install":    service.InstallWebPlayerRuntime,
		"start":      service.StartWebPlayer,
		"stop":       service.StopWebPlayer,
	} {
		status, err := action()
		if err == nil {
			t.Fatalf("%s error = nil without a backend", name)
		}
		if status.Available {
			t.Fatalf("%s Available = true without a backend", name)
		}
	}
}

func TestProvisionWebPlayerInstallsAndLogsInWithoutStarting(t *testing.T) {
	fake := &fakeWebBackend{}
	service := NewService(Options{Web: fake})

	if err := service.provisionWebPlayer(context.Background()); err != nil {
		t.Fatalf("provisionWebPlayer() error = %v", err)
	}
	want := []string{"install", "login"}
	if got := fake.callOrder(); !slices.Equal(got, want) {
		t.Fatalf("call order = %v, want %v", got, want)
	}
}

func TestProvisionWebPlayerReusesRuntimeAndSessionWithoutStarting(t *testing.T) {
	fake := &fakeWebBackend{status: webplayer.Status{
		RuntimeInstalled: true,
		LoggedIn:         true,
	}}
	service := NewService(Options{Web: fake})

	if err := service.provisionWebPlayer(context.Background()); err != nil {
		t.Fatalf("provisionWebPlayer() error = %v", err)
	}
	if got := fake.callOrder(); len(got) != 0 {
		t.Fatalf("call order = %v, want no calls", got)
	}
}

func TestProvisionWebPlayerSkipsEverythingWhenRunning(t *testing.T) {
	fake := &fakeWebBackend{status: webplayer.Status{
		RuntimeInstalled: true,
		LoggedIn:         true,
		Running:          true,
	}}
	service := NewService(Options{Web: fake})

	if err := service.provisionWebPlayer(context.Background()); err != nil {
		t.Fatalf("provisionWebPlayer() error = %v", err)
	}
	if got := fake.callOrder(); len(got) != 0 {
		t.Fatalf("call order = %v, want no calls", got)
	}
}

func TestProvisionWebPlayerStopsOnInstallFailure(t *testing.T) {
	fake := &fakeWebBackend{installErr: errors.New("download failed")}
	service := NewService(Options{Web: fake})

	if err := service.provisionWebPlayer(context.Background()); !errors.Is(err, fake.installErr) {
		t.Fatalf("provisionWebPlayer() error = %v, want the install error", err)
	}
	want := []string{"install"}
	if got := fake.callOrder(); !slices.Equal(got, want) {
		t.Fatalf("call order = %v, want %v", got, want)
	}
}

func TestProvisionWebPlayerStopsOnLoginFailure(t *testing.T) {
	fake := &fakeWebBackend{
		status:   webplayer.Status{RuntimeInstalled: true},
		loginErr: errors.New("login denied"),
	}
	service := NewService(Options{Web: fake})

	if err := service.provisionWebPlayer(context.Background()); !errors.Is(err, fake.loginErr) {
		t.Fatalf("provisionWebPlayer() error = %v, want the login error", err)
	}
	want := []string{"login"}
	if got := fake.callOrder(); !slices.Equal(got, want) {
		t.Fatalf("call order = %v, want %v", got, want)
	}
}

func TestProvisionWebPlayerLeavesAManualLoginAlone(t *testing.T) {
	fake := &fakeWebBackend{
		status:   webplayer.Status{RuntimeInstalled: true},
		loginErr: webplayer.ErrLoginInProgress,
	}
	service := NewService(Options{Web: fake})

	if err := service.provisionWebPlayer(context.Background()); err != nil {
		t.Fatalf("provisionWebPlayer() error = %v, want nil", err)
	}
	want := []string{"login"}
	if got := fake.callOrder(); !slices.Equal(got, want) {
		t.Fatalf("call order = %v, want %v", got, want)
	}
}

func TestProvisionWebPlayerStartsOnlyWhenKeepAvailable(t *testing.T) {
	fake := &fakeWebBackend{
		status:        webplayer.Status{RuntimeInstalled: true, LoggedIn: true},
		keepAvailable: true,
	}
	service := NewService(Options{Web: fake})

	if err := service.provisionWebPlayer(context.Background()); err != nil {
		t.Fatalf("provisionWebPlayer() error = %v", err)
	}
	want := []string{"start"}
	if got := fake.callOrder(); !slices.Equal(got, want) {
		t.Fatalf("call order = %v, want %v", got, want)
	}
}

func TestProvisionWebPlayerHonorsCancellation(t *testing.T) {
	fake := &fakeWebBackend{}
	service := NewService(Options{Web: fake})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := service.provisionWebPlayer(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("provisionWebPlayer() error = %v, want context.Canceled", err)
	}
	if got := fake.callOrder(); len(got) != 0 {
		t.Fatalf("call order = %v, want no calls", got)
	}
}

type fakeDeveloper struct {
	clientID string
	err      error
	calls    int
}

func (f *fakeDeveloper) EnsureWebApp(context.Context) (string, error) {
	f.calls++
	return f.clientID, f.err
}

func (f *fakeDeveloper) EnsureSoloistKey(context.Context, bool) (string, error) {
	return "", nil
}

func TestProvisionWebPlayerCreatesAnAppAutomatically(t *testing.T) {
	fake := &fakeWebBackend{}
	developer := &fakeDeveloper{clientID: "7fe450922d8b4d31ac8bda4a1767d903"}
	service := NewService(Options{Web: fake, Developer: developer})

	if err := service.provisionWebPlayer(context.Background()); err != nil {
		t.Fatalf("provisionWebPlayer() error = %v", err)
	}
	if developer.calls != 1 {
		t.Fatalf("EnsureWebApp calls = %d, want 1", developer.calls)
	}
	if fake.clientID != developer.clientID {
		t.Fatalf("web client ID = %q, want %q", fake.clientID, developer.clientID)
	}
	want := []string{"set-client", "install", "login"}
	if got := fake.callOrder(); !slices.Equal(got, want) {
		t.Fatalf("call order = %v, want %v", got, want)
	}
}

func TestProvisionWebPlayerKeepsAStoredApp(t *testing.T) {
	fake := &fakeWebBackend{configuredClientID: "7fe450922d8b4d31ac8bda4a1767d903"}
	developer := &fakeDeveloper{clientID: "7fe450922d8b4d31ac8bda4a1767d903"}
	service := NewService(Options{Web: fake, Developer: developer})

	if err := service.provisionWebPlayer(context.Background()); err != nil {
		t.Fatalf("provisionWebPlayer() error = %v", err)
	}
	if developer.calls != 0 {
		t.Fatalf("EnsureWebApp calls = %d, want 0", developer.calls)
	}
	want := []string{"install", "login"}
	if got := fake.callOrder(); !slices.Equal(got, want) {
		t.Fatalf("call order = %v, want %v", got, want)
	}
}

func TestProvisionWebPlayerContinuesWhenAppCreationFails(t *testing.T) {
	fake := &fakeWebBackend{}
	developer := &fakeDeveloper{err: errors.New("dashboard login required")}
	service := NewService(Options{Web: fake, Developer: developer})

	if err := service.provisionWebPlayer(context.Background()); err != nil {
		t.Fatalf("provisionWebPlayer() error = %v", err)
	}
	if developer.calls != 1 {
		t.Fatalf("EnsureWebApp calls = %d, want 1", developer.calls)
	}
	want := []string{"install", "login"}
	if got := fake.callOrder(); !slices.Equal(got, want) {
		t.Fatalf("call order = %v, want %v", got, want)
	}
}

func TestProvisionWebPlayerRespectsDisableAutoProvision(t *testing.T) {
	fake := &fakeWebBackend{}
	developer := &fakeDeveloper{clientID: "7fe450922d8b4d31ac8bda4a1767d903"}
	service := NewService(Options{
		Web:                  fake,
		Developer:            developer,
		DisableAutoProvision: true,
	})

	if err := service.provisionWebPlayer(context.Background()); err != nil {
		t.Fatalf("provisionWebPlayer() error = %v", err)
	}
	if developer.calls != 0 {
		t.Fatalf("EnsureWebApp calls = %d, want 0", developer.calls)
	}
	want := []string{"install", "login"}
	if got := fake.callOrder(); !slices.Equal(got, want) {
		t.Fatalf("call order = %v, want %v", got, want)
	}
}
