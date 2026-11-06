package audioapi

import (
	"context"
	"errors"
	"sync"
	"testing"

	"cadence/internal/webplayer"
)

// fakeWebBackend is a mutable, call-recording WebBackend. It never touches the
// network, the filesystem or a real runtime process.
type fakeWebBackend struct {
	mu sync.Mutex

	status     webplayer.Status
	clientID   string
	loginErr   error
	logoutErr  error
	installErr error
	startErr   error
	stopErr    error

	loginCalls      int
	logoutCalls     int
	installCalls    int
	startCalls      int
	stopCalls       int
	statusCalls     int
	invalidContexts int
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
	return webplayer.Install{Version: "1.2.3"}, f.installErr
}

func (f *fakeWebBackend) EnsureStarted(ctx context.Context) error {
	f.record(ctx)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.startCalls++
	return f.startErr
}

func (f *fakeWebBackend) Stop(ctx context.Context) error {
	f.record(ctx)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopCalls++
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

func (f *fakeWebBackend) calls() (login, logout, install, start, stop int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.loginCalls, f.logoutCalls, f.installCalls, f.startCalls, f.stopCalls
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
