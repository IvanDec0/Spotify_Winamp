package audioapi

import (
	"context"
	"errors"
	"sync"
	"testing"

	"cadence/internal/settings"
	"cadence/internal/webplayer"
)

type fakeDeveloperBackend struct {
	mu sync.Mutex

	webAppClientID string
	webAppErr      error
	soloistKey     string
	soloistErr     error

	webAppCalls  int
	soloistCalls int
	rotateArgs   []bool
	invalidCtx   int
}

func (f *fakeDeveloperBackend) EnsureWebApp(ctx context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.webAppCalls++
	if ctx == nil || ctx.Err() != nil {
		f.invalidCtx++
	}
	return f.webAppClientID, f.webAppErr
}

func (f *fakeDeveloperBackend) EnsureSoloistKey(ctx context.Context, rotate bool) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.soloistCalls++
	f.rotateArgs = append(f.rotateArgs, rotate)
	if ctx == nil || ctx.Err() != nil {
		f.invalidCtx++
	}
	return f.soloistKey, f.soloistErr
}

func (f *fakeDeveloperBackend) calls() (webApp, soloist int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.webAppCalls, f.soloistCalls
}

func TestConfigureWebPlayerAppChangesClientID(t *testing.T) {
	oldID := "1c6adf01cc01423d99ff7e87682c057a"
	newID := "7fe450922d8b4d31ac8bda4a1767d903"
	web := &fakeWebBackend{
		clientID: oldID,
		status:   webplayerStatus(oldID, newID),
	}
	developer := &fakeDeveloperBackend{webAppClientID: newID}
	service := NewService(Options{Web: web, Developer: developer})

	status, err := service.ConfigureWebPlayerApp()
	if err != nil {
		t.Fatalf("ConfigureWebPlayerApp() error = %v", err)
	}
	if status.ClientID != newID {
		t.Fatalf("ClientID = %q, want the new app", status.ClientID)
	}
	if !status.RestartRequired {
		t.Fatal("RestartRequired = false after a Client ID change")
	}
	if web.logoutCalls != 1 {
		t.Fatalf("logout calls = %d, want 1", web.logoutCalls)
	}
	webApp, _ := developer.calls()
	if webApp != 1 {
		t.Fatalf("EnsureWebApp calls = %d", webApp)
	}
}

func TestConfigureWebPlayerAppKeepsSessionForSameClientID(t *testing.T) {
	clientID := "7fe450922d8b4d31ac8bda4a1767d903"
	web := &fakeWebBackend{
		clientID: clientID,
		status:   webplayerStatus(clientID, clientID),
	}
	developer := &fakeDeveloperBackend{webAppClientID: clientID}
	service := NewService(Options{Web: web, Developer: developer})

	status, err := service.ConfigureWebPlayerApp()
	if err != nil {
		t.Fatalf("ConfigureWebPlayerApp() error = %v", err)
	}
	if status.ClientID != clientID {
		t.Fatalf("ClientID = %q", status.ClientID)
	}
	if status.RestartRequired {
		t.Fatal("RestartRequired = true when the app did not change")
	}
	if web.logoutCalls != 0 {
		t.Fatalf("logout calls = %d, want 0", web.logoutCalls)
	}
}

func TestConfigureWebPlayerAppPropagatesDeveloperErrors(t *testing.T) {
	actionErr := errors.New("dashboard login required")
	web := &fakeWebBackend{clientID: "old", status: webplayerStatus("old", "old")}
	service := NewService(Options{
		Web:       web,
		Developer: &fakeDeveloperBackend{webAppErr: actionErr},
	})

	if _, err := service.ConfigureWebPlayerApp(); !errors.Is(err, actionErr) {
		t.Fatalf("ConfigureWebPlayerApp() error = %v", err)
	}
	if web.logoutCalls != 0 {
		t.Fatal("a failed automation logged the player out")
	}
}

func TestConfigureWebPlayerAppRequiresBackends(t *testing.T) {
	service := NewService(Options{})
	if _, err := service.ConfigureWebPlayerApp(); err == nil {
		t.Fatal("ConfigureWebPlayerApp() error = nil without a developer backend")
	}
	service = NewService(Options{Developer: &fakeDeveloperBackend{}})
	if _, err := service.ConfigureWebPlayerApp(); err == nil {
		t.Fatal("ConfigureWebPlayerApp() error = nil without a web backend")
	}
}

func TestGenerateSoloistAPIKey(t *testing.T) {
	fake := &fakeSoloistBackend{}
	service, _ := newTestService(t, settings.BackendSoloist, fake)
	developer := &fakeDeveloperBackend{soloistKey: "spak_abcdef123456"}
	service.developer = developer

	state, err := service.GenerateSoloistAPIKey(false)
	if err != nil {
		t.Fatalf("GenerateSoloistAPIKey() error = %v", err)
	}
	fake.mu.Lock()
	saved := fake.savedKey
	hasKey := fake.hasKey
	fake.mu.Unlock()
	if saved != "spak_abcdef123456" || !hasKey {
		t.Fatalf("saved key = %q, hasKey = %v", saved, hasKey)
	}
	if state.Soloist == nil || !state.Soloist.APIKeySet {
		t.Fatalf("state.Soloist = %#v", state.Soloist)
	}

	if _, err := service.GenerateSoloistAPIKey(true); err != nil {
		t.Fatalf("GenerateSoloistAPIKey(true) error = %v", err)
	}
	developer.mu.Lock()
	rotateArgs := append([]bool(nil), developer.rotateArgs...)
	developer.mu.Unlock()
	if len(rotateArgs) != 2 || rotateArgs[0] || !rotateArgs[1] {
		t.Fatalf("rotate args = %v, want [false true]", rotateArgs)
	}
}

func TestGenerateSoloistAPIKeyErrors(t *testing.T) {
	actionErr := errors.New("premium required")
	fake := &fakeSoloistBackend{}
	service, _ := newTestService(t, settings.BackendSoloist, fake)
	service.developer = &fakeDeveloperBackend{soloistErr: actionErr}

	if _, err := service.GenerateSoloistAPIKey(false); !errors.Is(err, actionErr) {
		t.Fatalf("GenerateSoloistAPIKey() error = %v", err)
	}
	fake.mu.Lock()
	saved := fake.savedKey
	fake.mu.Unlock()
	if saved != "" {
		t.Fatal("a failed automation stored a key")
	}

	service.developer = nil
	if _, err := service.GenerateSoloistAPIKey(false); err == nil {
		t.Fatal("GenerateSoloistAPIKey() error = nil without a developer backend")
	}
}

func TestGenerateSoloistAPIKeyRequiresSoloist(t *testing.T) {
	service, _ := newTestService(t, settings.BackendSoloist, nil)
	service.developer = &fakeDeveloperBackend{soloistKey: "spak_key"}
	if _, err := service.GenerateSoloistAPIKey(false); err == nil {
		t.Fatal("GenerateSoloistAPIKey() error = nil without a Soloist backend")
	}
}

func webplayerStatus(clientID, configured string) webplayer.Status {
	return webplayer.Status{
		LoggedIn:           true,
		ClientID:           clientID,
		ConfiguredClientID: configured,
	}
}
