package webplayer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newServiceForTest(t *testing.T, options ServiceOptions) *Service {
	t.Helper()
	if options.Root == "" {
		options.Root = filepath.Join(t.TempDir(), "root")
	}
	if options.ConfigDir == "" {
		options.ConfigDir = filepath.Join(t.TempDir(), "config")
	}
	service, err := NewService(options)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

func TestNewServiceValidatesRedirectURI(t *testing.T) {
	tests := []struct {
		name        string
		listenAddr  string
		redirectURI string
		wantErr     bool
	}{
		{
			name:        "valid",
			listenAddr:  "127.0.0.1:8888",
			redirectURI: "http://127.0.0.1:8888/callback",
		},
		{
			name:        "port mismatch",
			listenAddr:  "127.0.0.1:9000",
			redirectURI: "http://127.0.0.1:8888/callback",
			wantErr:     true,
		},
		{
			name:        "non loopback redirect",
			listenAddr:  "127.0.0.1:8888",
			redirectURI: "http://localhost:8888/callback",
			wantErr:     true,
		},
		{
			name:        "wrong path",
			listenAddr:  "127.0.0.1:8888",
			redirectURI: "http://127.0.0.1:8888/callback/extra",
			wantErr:     true,
		},
		{
			name:        "query",
			listenAddr:  "127.0.0.1:8888",
			redirectURI: "http://127.0.0.1:8888/callback?x=1",
			wantErr:     true,
		},
		{
			name:        "https",
			listenAddr:  "127.0.0.1:8888",
			redirectURI: "https://127.0.0.1:8888/callback",
			wantErr:     true,
		},
		{
			name:        "non loopback listen",
			listenAddr:  "0.0.0.0:8888",
			redirectURI: "http://127.0.0.1:8888/callback",
			wantErr:     true,
		},
		{
			name:        "missing redirect port",
			listenAddr:  "127.0.0.1:8888",
			redirectURI: "http://127.0.0.1/callback",
			wantErr:     true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, err := NewService(ServiceOptions{
				ListenAddr:  test.listenAddr,
				RedirectURI: test.redirectURI,
				Root:        filepath.Join(t.TempDir(), "root"),
				ConfigDir:   filepath.Join(t.TempDir(), "config"),
			})
			if test.wantErr {
				if err == nil {
					t.Fatalf("NewService() error = nil for %s", test.name)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewService() error = %v", err)
			}
			if service.Provider() == nil || service.API() == nil {
				t.Fatal("the service did not build its provider and API")
			}
			if service.clientID != DefaultClientID {
				t.Fatalf("client ID = %q, want %q", service.clientID, DefaultClientID)
			}
			if service.DeviceName() != DefaultDeviceName {
				t.Fatalf("device name = %q", service.DeviceName())
			}
			if service.store.Path() != filepath.Join(service.configDir, "tokens.json") {
				t.Fatalf("token path = %q", service.store.Path())
			}
		})
	}
}

func TestServiceStatusWithoutTokens(t *testing.T) {
	service := newServiceForTest(t, ServiceOptions{})
	status := service.Status()
	if status.LoggedIn {
		t.Fatal("Status().LoggedIn = true without a token file")
	}
	if status.Running {
		t.Fatal("Status().Running = true without a process")
	}
	if status.DeviceID != "" {
		t.Fatalf("Status().DeviceID = %q", status.DeviceID)
	}
	if status.RuntimeInstalled {
		t.Fatal("Status().RuntimeInstalled = true in an empty root")
	}
	if status.RuntimeVersion != "" {
		t.Fatalf("Status().RuntimeVersion = %q", status.RuntimeVersion)
	}
	if service.RuntimeInstalled() {
		t.Fatal("RuntimeInstalled() = true in an empty root")
	}
	if service.LoggedIn() {
		t.Fatal("LoggedIn() = true without a token file")
	}
	if service.Running() {
		t.Fatal("Running() = true without a process")
	}
}

func TestServiceStatusWithTokensAndLogout(t *testing.T) {
	configDir := filepath.Join(t.TempDir(), "config")
	store := NewTokenStore(filepath.Join(configDir, "tokens.json"))
	if err := store.Save(Tokens{
		AccessToken:  "access",
		RefreshToken: "refresh",
		Expiry:       time.Now().Add(time.Hour),
		ClientID:     DefaultClientID,
		Account:      "Listener",
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	service := newServiceForTest(t, ServiceOptions{ConfigDir: configDir})
	status := service.Status()
	if !status.LoggedIn {
		t.Fatal("Status().LoggedIn = false with a valid token file")
	}
	if status.Account != "Listener" {
		t.Fatalf("Status().Account = %q, want %q", status.Account, "Listener")
	}
	if !service.LoggedIn() {
		t.Fatal("LoggedIn() = false with a valid token file")
	}

	if err := service.Logout(); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if service.LoggedIn() {
		t.Fatal("LoggedIn() = true after Logout()")
	}
	if _, err := store.Load(); !errors.Is(err, ErrNoTokens) {
		t.Fatalf("the token file survived Logout(): %v", err)
	}
}

func TestServiceClientIDResolution(t *testing.T) {
	valid := "7fe450922d8b4d31ac8bda4a1767d903"
	configDir := t.TempDir()
	if err := os.WriteFile(ClientIDPath(configDir), []byte(valid+"\n"), 0o600); err != nil {
		t.Fatalf("write client id: %v", err)
	}

	service := newServiceForTest(t, ServiceOptions{ConfigDir: configDir})
	if got := service.ClientID(); got != valid {
		t.Fatalf("ClientID() = %q, want the stored value", got)
	}
	if got := service.ConfiguredClientID(); got != valid {
		t.Fatalf("ConfiguredClientID() = %q", got)
	}
	status := service.Status()
	if status.ClientID != valid || status.ConfiguredClientID != valid {
		t.Fatalf("Status() client IDs = %q/%q", status.ClientID, status.ConfiguredClientID)
	}

	// An explicit override wins over the stored file.
	override := newServiceForTest(t, ServiceOptions{ConfigDir: configDir, ClientID: DefaultClientID})
	if got := override.ClientID(); got != DefaultClientID {
		t.Fatalf("explicit ClientID() = %q", got)
	}

	// The configured value is re-read on each call.
	updated := "0123456789abcdef0123456789abcdef"
	if err := os.WriteFile(ClientIDPath(configDir), []byte(updated), 0o600); err != nil {
		t.Fatalf("rewrite client id: %v", err)
	}
	if got := service.ConfiguredClientID(); got != updated {
		t.Fatalf("ConfiguredClientID() after change = %q", got)
	}
	if got := service.Status().ConfiguredClientID; got != updated {
		t.Fatalf("Status().ConfiguredClientID after change = %q", got)
	}
	if got := service.Status().ClientID; got != valid {
		t.Fatalf("Status().ClientID changed without a restart: %q", got)
	}
}

func TestServiceInvalidClientIDFallsBack(t *testing.T) {
	configDir := t.TempDir()
	if err := os.WriteFile(ClientIDPath(configDir), []byte("not-a-client-id"), 0o600); err != nil {
		t.Fatalf("write client id: %v", err)
	}
	service := newServiceForTest(t, ServiceOptions{ConfigDir: configDir})
	if got := service.ClientID(); got != DefaultClientID {
		t.Fatalf("ClientID() = %q, want the default", got)
	}
	if got := service.ConfiguredClientID(); got != "" {
		t.Fatalf("ConfiguredClientID() = %q, want empty", got)
	}
}

func TestServiceStatusRejectsSessionFromAnotherClient(t *testing.T) {
	tests := []struct {
		name     string
		clientID string
	}{
		{name: "another app", clientID: "another-client"},
		{name: "session without a Client ID", clientID: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configDir := t.TempDir()
			store := NewTokenStore(filepath.Join(configDir, "tokens.json"))
			if err := store.Save(Tokens{
				AccessToken:  "access",
				RefreshToken: "refresh",
				Expiry:       time.Now().Add(time.Hour),
				ClientID:     test.clientID,
				Account:      "Old Listener",
			}); err != nil {
				t.Fatalf("Save() error = %v", err)
			}
			service := newServiceForTest(t, ServiceOptions{ConfigDir: configDir})
			if service.LoggedIn() {
				t.Fatal("LoggedIn() = true for an invalid session")
			}
			status := service.Status()
			if status.LoggedIn {
				t.Fatal("Status().LoggedIn = true for an invalid session")
			}
			if status.Account != "" {
				t.Fatal("Status().Account shows the old account for an invalid session")
			}
			if status.ClientID != DefaultClientID {
				t.Fatalf("Status().ClientID = %q, want %q", status.ClientID, DefaultClientID)
			}
		})
	}
}

func TestServiceStopIsIdempotent(t *testing.T) {
	service := newServiceForTest(t, ServiceOptions{})
	if err := service.Stop(context.Background()); err != nil {
		t.Fatalf("first Stop() error = %v", err)
	}
	if err := service.Stop(context.Background()); err != nil {
		t.Fatalf("second Stop() error = %v", err)
	}
}

func TestServiceStatusTracksLastError(t *testing.T) {
	service := newServiceForTest(t, ServiceOptions{})
	if err := service.EnsureStarted(context.Background()); !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("EnsureStarted() error = %v, want ErrLoginRequired", err)
	}
	status := service.Status()
	if status.LastError != CodeLoginRequired {
		t.Fatalf("Status().LastError = %q, want %q", status.LastError, CodeLoginRequired)
	}

	if err := service.Logout(); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if got := service.Status().LastError; got != "" {
		t.Fatalf("Status().LastError after Logout() = %q", got)
	}
}

func TestKeepAvailablePreference(t *testing.T) {
	service := newServiceForTest(t, ServiceOptions{})
	if service.KeepAvailable() {
		t.Fatal("KeepAvailable() = true by default, want false")
	}
	service.SetKeepAvailable(true)
	if !service.KeepAvailable() {
		t.Fatal("KeepAvailable() = false after SetKeepAvailable(true)")
	}
	service.SetKeepAvailable(false)
	if service.KeepAvailable() {
		t.Fatal("KeepAvailable() = true after SetKeepAvailable(false)")
	}
}
