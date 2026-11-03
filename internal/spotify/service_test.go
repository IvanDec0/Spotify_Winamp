package spotify

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"cadence/internal/artcache"
	"cadence/internal/keyring"
	"cadence/internal/player"
	"cadence/internal/settings"
)

func newTestTokenServer(t *testing.T, refresh string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"access_token":  "access-token",
			"refresh_token": refresh,
			"expires_in":    3600,
		})
	}))
	t.Cleanup(server.Close)
	return server
}

func newTestAPIServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	return server
}

func testBrowserOpener(t *testing.T) func(string) error {
	t.Helper()
	return func(authURL string) error {
		parsed, err := url.Parse(authURL)
		if err != nil {
			return err
		}
		redirect, err := url.Parse(parsed.Query().Get("redirect_uri"))
		if err != nil {
			return err
		}
		query := redirect.Query()
		query.Set("code", "test-code")
		query.Set("state", parsed.Query().Get("state"))
		redirect.RawQuery = query.Encode()
		response, err := http.Get(redirect.String())
		if err != nil {
			return err
		}
		defer response.Body.Close()
		return nil
	}
}

func waitForAuth(t *testing.T, service *Service, status AuthStatus) AuthState {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		state := service.GetAuthState()
		if state.Status == status {
			return state
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("auth state = %#v, want %q", service.GetAuthState(), status)
	return AuthState{}
}

func newTestService(t *testing.T, keyringStore keyring.Store, oauth *OAuth, apiBase string) (*Service, *player.Service, *settings.Store) {
	t.Helper()
	store := settings.NewStore(t.TempDir() + "/config.json")
	if err := store.UpdateSpotify(func(config *settings.SpotifyState) error {
		config.ClientID = "client-id"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	demo := player.NewDemoProvider()
	playerService := player.NewService(demo)
	service := NewService(Options{
		Store:       store,
		Keyring:     keyringStore,
		Player:      playerService,
		Demo:        demo,
		Art:         artcache.New(nil),
		OAuth:       oauth,
		APIBaseURL:  apiBase,
		OpenBrowser: testBrowserOpener(t),
	})
	return service, playerService, store
}

func TestServiceLoginUseDemoUseSpotifyLogout(t *testing.T) {
	tokenServer := newTestTokenServer(t, "refresh-token")
	apiServer := newTestAPIServer(t)
	keyringStore := keyring.NewMemoryStore()
	oauth := &OAuth{ListenAddress: "127.0.0.1:0", TokenURL: tokenServer.URL}
	service, playerService, store := newTestService(t, keyringStore, oauth, apiServer.URL)

	if err := service.Login(); err != nil {
		t.Fatal(err)
	}
	state := waitForAuth(t, service, AuthAuthenticated)
	if state.PlayerMode != "spotify" {
		t.Fatalf("player mode = %q", state.PlayerMode)
	}
	if token, err := keyringStore.GetRefreshToken(); err != nil || token != "refresh-token" {
		t.Fatalf("refresh token = %q, %v", token, err)
	}
	playerState, err := playerService.GetState()
	if err != nil {
		t.Fatal(err)
	}
	if playerState.Provider != "spotify" {
		t.Fatalf("provider = %q", playerState.Provider)
	}

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "access-token") || strings.Contains(string(data), "refresh-token") {
		t.Fatalf("auth state leaked a token: %s", data)
	}

	if err := service.UseDemo(); err != nil {
		t.Fatal(err)
	}
	if state := service.GetAuthState(); state.PlayerMode != "demo" {
		t.Fatalf("demo mode = %#v", state)
	}
	playerState, err = playerService.GetState()
	if err != nil {
		t.Fatal(err)
	}
	if playerState.Provider != "demo" {
		t.Fatalf("demo provider = %q", playerState.Provider)
	}
	if err := service.UseSpotify(); err != nil {
		t.Fatal(err)
	}
	if err := service.Logout(); err != nil {
		t.Fatal(err)
	}
	state = service.GetAuthState()
	if state.Status != AuthUnauthenticated || state.PlayerMode != "demo" {
		t.Fatalf("logout state = %#v", state)
	}
	if _, err := keyringStore.GetRefreshToken(); err == nil {
		t.Fatal("refresh token still present after logout")
	}
	config, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if config.Spotify.ClientID != "client-id" {
		t.Fatalf("client id = %q", config.Spotify.ClientID)
	}
}

func TestServiceRestoresSessionAndHonorsDemoMode(t *testing.T) {
	tokenServer := newTestTokenServer(t, "rotated-token")
	apiServer := newTestAPIServer(t)
	keyringStore := keyring.NewMemoryStore()
	if err := keyringStore.SetRefreshToken("refresh-token"); err != nil {
		t.Fatal(err)
	}
	oauth := &OAuth{ListenAddress: "127.0.0.1:0", TokenURL: tokenServer.URL}
	service, playerService, store := newTestService(t, keyringStore, oauth, apiServer.URL)
	if err := store.UpdateSpotify(func(config *settings.SpotifyState) error {
		config.DemoMode = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	service.restore()
	state := waitForAuth(t, service, AuthAuthenticated)
	if state.PlayerMode != "demo" {
		t.Fatalf("restored demo mode = %#v", state)
	}
	playerState, err := playerService.GetState()
	if err != nil {
		t.Fatal(err)
	}
	if playerState.Provider != "demo" {
		t.Fatalf("provider = %q", playerState.Provider)
	}
	if token, _ := keyringStore.GetRefreshToken(); token != "rotated-token" {
		t.Fatalf("rotated token = %q", token)
	}
	if err := service.UseSpotify(); err != nil {
		t.Fatal(err)
	}
	playerState, err = playerService.GetState()
	if err != nil {
		t.Fatal(err)
	}
	if playerState.Provider != "spotify" {
		t.Fatalf("provider after UseSpotify = %q", playerState.Provider)
	}
}

func TestServiceRestoreRevokedToken(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(writer).Encode(map[string]string{"error": "invalid_grant"})
	}))
	defer tokenServer.Close()
	keyringStore := keyring.NewMemoryStore()
	_ = keyringStore.SetRefreshToken("revoked-token")
	oauth := &OAuth{ListenAddress: "127.0.0.1:0", TokenURL: tokenServer.URL}
	service, _, _ := newTestService(t, keyringStore, oauth, "http://api.invalid")

	service.restore()
	state := waitForAuth(t, service, AuthError)
	if !strings.Contains(strings.ToLower(state.Error), "expired") {
		t.Fatalf("error = %q", state.Error)
	}
	if _, err := keyringStore.GetRefreshToken(); err == nil {
		t.Fatal("revoked token still present")
	}
}

func TestServiceKeyringUnavailable(t *testing.T) {
	keyringStore := keyring.NewMemoryStoreWithError(errString("OS keyring is unavailable: no Secret Service"))
	service, _, _ := newTestService(t, keyringStore, &OAuth{ListenAddress: "127.0.0.1:0"}, "http://api.invalid")
	service.restore()
	state := waitForAuth(t, service, AuthError)
	if state.KeyringAvailable || !strings.Contains(state.Error, "keyring") {
		t.Fatalf("state = %#v", state)
	}
}

func TestServiceSetClientIDLogsOut(t *testing.T) {
	tokenServer := newTestTokenServer(t, "refresh-token")
	apiServer := newTestAPIServer(t)
	keyringStore := keyring.NewMemoryStore()
	service, _, store := newTestService(t, keyringStore, &OAuth{ListenAddress: "127.0.0.1:0", TokenURL: tokenServer.URL}, apiServer.URL)
	if err := service.Login(); err != nil {
		t.Fatal(err)
	}
	waitForAuth(t, service, AuthAuthenticated)
	if err := service.SetClientID("new-client-id"); err != nil {
		t.Fatal(err)
	}
	state := service.GetAuthState()
	if state.Status != AuthUnauthenticated || state.ClientID != "new-client-id" {
		t.Fatalf("state = %#v", state)
	}
	if _, err := keyringStore.GetRefreshToken(); err == nil {
		t.Fatal("refresh token still present after client id change")
	}
	config, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if config.Spotify.ClientID != "new-client-id" {
		t.Fatalf("client id = %q", config.Spotify.ClientID)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
