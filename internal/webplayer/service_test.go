package webplayer

import (
	"context"
	"errors"
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
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	service := newServiceForTest(t, ServiceOptions{ConfigDir: configDir})
	status := service.Status()
	if !status.LoggedIn {
		t.Fatal("Status().LoggedIn = false with a valid token file")
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

func TestServiceStopIsIdempotent(t *testing.T) {
	service := newServiceForTest(t, ServiceOptions{})
	if err := service.Stop(context.Background()); err != nil {
		t.Fatalf("first Stop() error = %v", err)
	}
	if err := service.Stop(context.Background()); err != nil {
		t.Fatalf("second Stop() error = %v", err)
	}
}
