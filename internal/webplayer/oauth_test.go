package webplayer

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeTokenEndpoint struct {
	mu            sync.Mutex
	forms         []url.Values
	accessToken   string
	refreshToken  string
	expiresIn     int64
	rotate        bool
	invalidGrant  bool
	exchangeError bool
}

func (f *fakeTokenEndpoint) handler(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := request.ParseForm(); err != nil {
		http.Error(writer, "bad form", http.StatusBadRequest)
		return
	}
	form := request.PostForm
	f.mu.Lock()
	f.forms = append(f.forms, form)
	accessToken := f.accessToken
	refreshToken := f.refreshToken
	expiresIn := f.expiresIn
	rotate := f.rotate
	invalidGrant := f.invalidGrant
	exchangeError := f.exchangeError
	f.mu.Unlock()

	writeError := func(status int, code string) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(status)
		_ = json.NewEncoder(writer).Encode(map[string]string{"error": code})
	}
	writeTokens := func() {
		payload := map[string]any{
			"access_token": accessToken,
			"token_type":   "Bearer",
			"scope":        strings.Join(DefaultScopes, " "),
			"expires_in":   expiresIn,
		}
		if refreshToken != "" {
			payload["refresh_token"] = refreshToken
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(payload)
	}

	switch form.Get("grant_type") {
	case "authorization_code":
		if exchangeError {
			writeError(http.StatusBadRequest, "invalid_client")
			return
		}
		writeTokens()
	case "refresh_token":
		if invalidGrant {
			writeError(http.StatusBadRequest, "invalid_grant")
			return
		}
		if !rotate {
			refreshToken = ""
		}
		writeTokens()
	default:
		writeError(http.StatusBadRequest, "unsupported_grant_type")
	}
}

func (f *fakeTokenEndpoint) formCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.forms)
}

func (f *fakeTokenEndpoint) form(index int) url.Values {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.forms[index]
}

func newFakeTokenServer(t *testing.T, endpoint *fakeTokenEndpoint) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(endpoint.handler))
	t.Cleanup(server.Close)
	return server
}

func oauthForTest(t *testing.T, tokenURL string, openURL func(string) error) (*OAuth, *TokenStore, *time.Time) {
	t.Helper()
	now := time.Date(2026, time.November, 6, 12, 0, 0, 0, time.UTC)
	store := NewTokenStore(filepath.Join(t.TempDir(), "tokens.json"))
	oauth, err := NewOAuth(OAuthOptions{
		ClientID:    "client-id",
		RedirectURI: "http://127.0.0.1:8888/callback",
		Store:       store,
		TokenURL:    tokenURL,
		OpenURL:     openURL,
		Now:         func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewOAuth() error = %v", err)
	}
	return oauth, store, &now
}

func TestOAuthLoginPKCE(t *testing.T) {
	endpoint := &fakeTokenEndpoint{
		accessToken:  "access-1",
		refreshToken: "refresh-1",
		expiresIn:    3600,
	}
	server := newFakeTokenServer(t, endpoint)

	var opened string
	var oauth *OAuth
	oauth, store, now := oauthForTest(t, server.URL, func(raw string) error {
		opened = raw
		target, err := url.Parse(raw)
		if err != nil {
			return err
		}
		return oauth.HandleCallback(context.Background(), "auth-code", target.Query().Get("state"))
	})

	if err := oauth.Login(context.Background()); err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if opened == "" {
		t.Fatal("the authorize URL was not opened")
	}
	target, err := url.Parse(opened)
	if err != nil {
		t.Fatalf("parse authorize URL: %v", err)
	}
	query := target.Query()
	if query.Get("response_type") != "code" {
		t.Fatalf("response_type = %q", query.Get("response_type"))
	}
	if query.Get("client_id") != "client-id" {
		t.Fatalf("client_id = %q", query.Get("client_id"))
	}
	if query.Get("redirect_uri") != "http://127.0.0.1:8888/callback" {
		t.Fatalf("redirect_uri = %q", query.Get("redirect_uri"))
	}
	if query.Get("scope") != strings.Join(DefaultScopes, " ") {
		t.Fatalf("scope = %q", query.Get("scope"))
	}
	if query.Get("code_challenge_method") != "S256" {
		t.Fatalf("code_challenge_method = %q", query.Get("code_challenge_method"))
	}
	if query.Get("show_dialog") != "false" {
		t.Fatalf("show_dialog = %q", query.Get("show_dialog"))
	}
	if strings.Contains(opened, "access_token") || strings.Contains(opened, "refresh_token") {
		t.Fatalf("the authorize URL carries a token: %s", opened)
	}

	if endpoint.formCount() != 1 {
		t.Fatalf("token requests = %d, want 1", endpoint.formCount())
	}
	form := endpoint.form(0)
	if form.Get("grant_type") != "authorization_code" || form.Get("code") != "auth-code" {
		t.Fatalf("token form = %#v", form)
	}
	if form.Get("code_verifier") == "" {
		t.Fatal("the token exchange has no code_verifier")
	}
	if got := pkceChallenge(form.Get("code_verifier")); got != query.Get("code_challenge") {
		t.Fatalf("PKCE challenge mismatch: got %q want %q", got, query.Get("code_challenge"))
	}

	tokens, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if tokens.AccessToken != "access-1" || tokens.RefreshToken != "refresh-1" {
		t.Fatalf("tokens = %#v", tokens)
	}
	wantExpiry := now.Add(time.Hour)
	if !tokens.Expiry.Equal(wantExpiry) {
		t.Fatalf("expiry = %v, want %v", tokens.Expiry, wantExpiry)
	}
	if !oauth.LoggedIn() {
		t.Fatal("LoggedIn() = false after a successful login")
	}
}

func TestOAuthStateMismatch(t *testing.T) {
	endpoint := &fakeTokenEndpoint{accessToken: "access", refreshToken: "refresh", expiresIn: 3600}
	server := newFakeTokenServer(t, endpoint)
	var oauth *OAuth
	oauth, store, _ := oauthForTest(t, server.URL, func(raw string) error {
		return oauth.HandleCallback(context.Background(), "auth-code", "wrong-state")
	})

	if err := oauth.Login(context.Background()); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("Login() error = %v, want ErrInvalidState", err)
	}
	if endpoint.formCount() != 0 {
		t.Fatal("the token endpoint ran for a mismatched state")
	}
	if _, err := store.Load(); !errors.Is(err, ErrNoTokens) {
		t.Fatalf("tokens were stored after a mismatched state: %v", err)
	}

	if err := oauth.HandleCallback(context.Background(), "code", "state"); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("HandleCallback() without a pending login = %v, want ErrInvalidState", err)
	}
}

func TestOAuthExchangeFailure(t *testing.T) {
	endpoint := &fakeTokenEndpoint{exchangeError: true}
	server := newFakeTokenServer(t, endpoint)
	var oauth *OAuth
	oauth, store, _ := oauthForTest(t, server.URL, func(raw string) error {
		target, err := url.Parse(raw)
		if err != nil {
			return err
		}
		return oauth.HandleCallback(context.Background(), "auth-code", target.Query().Get("state"))
	})

	if err := oauth.Login(context.Background()); err == nil {
		t.Fatal("Login() succeeded after a failed exchange")
	}
	if _, err := store.Load(); !errors.Is(err, ErrNoTokens) {
		t.Fatalf("tokens were stored after a failed exchange: %v", err)
	}
}

func TestOAuthRefreshAndRotation(t *testing.T) {
	endpoint := &fakeTokenEndpoint{
		accessToken:  "access-2",
		refreshToken: "refresh-2",
		expiresIn:    1800,
		rotate:       true,
	}
	server := newFakeTokenServer(t, endpoint)
	oauth, store, now := oauthForTest(t, server.URL, func(string) error { return nil })

	if err := store.Save(Tokens{
		AccessToken:  "access-1",
		RefreshToken: "refresh-1",
		TokenType:    "Bearer",
		Scope:        strings.Join(DefaultScopes, " "),
		Expiry:       now.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	access, expiry, err := oauth.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if access != "access-2" {
		t.Fatalf("access token = %q", access)
	}
	if !expiry.Equal(now.Add(30 * time.Minute)) {
		t.Fatalf("expiry = %v", expiry)
	}
	tokens, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if tokens.RefreshToken != "refresh-2" {
		t.Fatalf("refresh token = %q, want the rotated value", tokens.RefreshToken)
	}
	form := endpoint.form(0)
	if form.Get("grant_type") != "refresh_token" || form.Get("refresh_token") != "refresh-1" {
		t.Fatalf("refresh form = %#v", form)
	}
	if form.Get("client_id") != "client-id" {
		t.Fatalf("client_id = %q", form.Get("client_id"))
	}

	// A refresh response without a new refresh token keeps the previous one.
	endpoint.mu.Lock()
	endpoint.rotate = false
	endpoint.accessToken = "access-3"
	endpoint.mu.Unlock()
	if _, _, err := oauth.Refresh(context.Background()); err != nil {
		t.Fatalf("second Refresh() error = %v", err)
	}
	tokens, err = store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if tokens.RefreshToken != "refresh-2" {
		t.Fatalf("refresh token = %q, want the previous value", tokens.RefreshToken)
	}
}

func TestOAuthAccessTokenUsesValidToken(t *testing.T) {
	endpoint := &fakeTokenEndpoint{accessToken: "new", refreshToken: "new-refresh", expiresIn: 3600}
	server := newFakeTokenServer(t, endpoint)
	oauth, store, now := oauthForTest(t, server.URL, func(string) error { return nil })

	if err := store.Save(Tokens{
		AccessToken:  "current",
		RefreshToken: "refresh",
		Expiry:       now.Add(2 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	access, _, err := oauth.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken() error = %v", err)
	}
	if access != "current" {
		t.Fatalf("access token = %q", access)
	}
	if endpoint.formCount() != 0 {
		t.Fatal("a valid token triggered a refresh")
	}

	// Inside the 60-second margin the token is refreshed.
	if err := store.Save(Tokens{
		AccessToken:  "current",
		RefreshToken: "refresh",
		Expiry:       now.Add(30 * time.Second),
	}); err != nil {
		t.Fatal(err)
	}
	access, _, err = oauth.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken() error = %v", err)
	}
	if access != "new" {
		t.Fatalf("access token = %q", access)
	}
	if endpoint.formCount() != 1 {
		t.Fatalf("token requests = %d, want 1", endpoint.formCount())
	}
}

func TestOAuthInvalidGrantClearsStore(t *testing.T) {
	endpoint := &fakeTokenEndpoint{invalidGrant: true}
	server := newFakeTokenServer(t, endpoint)
	oauth, store, _ := oauthForTest(t, server.URL, func(string) error { return nil })
	if err := store.Save(Tokens{AccessToken: "access", RefreshToken: "refresh"}); err != nil {
		t.Fatal(err)
	}

	if _, _, err := oauth.Refresh(context.Background()); !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("Refresh() error = %v, want ErrLoginRequired", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrNoTokens) {
		t.Fatalf("the store was not cleared: %v", err)
	}
	if oauth.LoggedIn() {
		t.Fatal("LoggedIn() = true after invalid_grant")
	}
}

func TestOAuthLoginInProgress(t *testing.T) {
	endpoint := &fakeTokenEndpoint{accessToken: "access", refreshToken: "refresh", expiresIn: 3600}
	server := newFakeTokenServer(t, endpoint)
	opened := make(chan string, 1)
	release := make(chan struct{})
	var oauth *OAuth
	oauth, _, _ = oauthForTest(t, server.URL, func(raw string) error {
		opened <- raw
		<-release
		return nil
	})

	loginErr := make(chan error, 1)
	go func() {
		loginErr <- oauth.Login(context.Background())
	}()

	var authorizeURL string
	select {
	case authorizeURL = <-opened:
	case <-time.After(3 * time.Second):
		t.Fatal("the authorize URL was never opened")
	}
	if err := oauth.Login(context.Background()); !errors.Is(err, ErrLoginInProgress) {
		t.Fatalf("second Login() = %v, want ErrLoginInProgress", err)
	}

	target, err := url.Parse(authorizeURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := oauth.HandleCallback(context.Background(), "auth-code", target.Query().Get("state")); err != nil {
		t.Fatalf("HandleCallback() error = %v", err)
	}
	close(release)
	select {
	case err := <-loginErr:
		if err != nil {
			t.Fatalf("first Login() error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the first login never returned")
	}
}

func TestOAuthLoginTimeout(t *testing.T) {
	endpoint := &fakeTokenEndpoint{}
	server := newFakeTokenServer(t, endpoint)
	now := time.Now()
	store := NewTokenStore(filepath.Join(t.TempDir(), "tokens.json"))
	oauth, err := NewOAuth(OAuthOptions{
		ClientID:     "client-id",
		RedirectURI:  "http://127.0.0.1:8888/callback",
		Store:        store,
		TokenURL:     server.URL,
		OpenURL:      func(string) error { return nil },
		Now:          func() time.Time { return now },
		LoginTimeout: 30 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := oauth.Login(context.Background()); !errors.Is(err, ErrLoginTimeout) {
		t.Fatalf("Login() error = %v, want ErrLoginTimeout", err)
	}
}

func TestOAuthOpenFailureAndLogout(t *testing.T) {
	endpoint := &fakeTokenEndpoint{accessToken: "access", refreshToken: "refresh", expiresIn: 3600}
	server := newFakeTokenServer(t, endpoint)
	var oauth *OAuth
	oauth, store, _ := oauthForTest(t, server.URL, func(raw string) error {
		target, err := url.Parse(raw)
		if err != nil {
			return err
		}
		return oauth.HandleCallback(context.Background(), "auth-code", target.Query().Get("state"))
	})

	oauthRef := oauth
	oauthRef2, _, _ := oauthForTest(t, server.URL, func(string) error {
		return errors.New("xdg-open is unavailable")
	})
	if err := oauthRef2.Login(context.Background()); err == nil {
		t.Fatal("Login() succeeded when the browser could not open")
	}
	if err := oauthRef2.HandleCallback(context.Background(), "code", "state"); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("the failed login left a pending attempt: %v", err)
	}

	if err := oauthRef.Login(context.Background()); err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if err := oauthRef.Logout(); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrNoTokens) {
		t.Fatalf("Logout() left tokens: %v", err)
	}
	if oauthRef.LoggedIn() {
		t.Fatal("LoggedIn() = true after Logout()")
	}
}
