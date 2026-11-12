package webplayer

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultClientID is the public PKCE client used by Cadence. It can be
	// overridden for development with CADENCE_SPOTIFY_CLIENT_ID.
	DefaultClientID = "8914d701933445d2800395f85d115fed"

	defaultAuthorizeURL = "https://accounts.spotify.com/authorize"
	defaultTokenURL     = "https://accounts.spotify.com/api/token"

	defaultLoginTimeout = 5 * time.Minute
	tokenRequestTimeout = 30 * time.Second
	accessTokenMargin   = 60 * time.Second
	maxTokenResponse    = 1 << 20
)

// DefaultScopes are the Spotify permissions the web player needs.
var DefaultScopes = []string{
	"streaming",
	"user-read-playback-state",
	"user-modify-playback-state",
	"user-read-currently-playing",
	"user-read-email",
	"user-read-private",
}

// OAuthOptions configures the Spotify Authorization Code + PKCE flow.
type OAuthOptions struct {
	ClientID     string
	RedirectURI  string
	Scopes       []string
	Store        *TokenStore
	HTTPClient   *http.Client
	AuthorizeURL string
	TokenURL     string
	OpenURL      func(string) error
	Now          func() time.Time
	LoginTimeout time.Duration
}

// OAuth owns the Spotify login flow and the persisted session. It never
// exposes token values.
type OAuth struct {
	clientID     string
	redirectURI  string
	scopes       []string
	store        *TokenStore
	client       *http.Client
	authorizeURL string
	tokenURL     string
	openURL      func(string) error
	now          func() time.Time
	loginTimeout time.Duration

	mu       sync.Mutex
	pending  *loginAttempt
	clientMu sync.RWMutex

	refreshMu sync.Mutex
}

type loginAttempt struct {
	state    string
	verifier string
	done     chan error
}

var (
	_ TokenSource     = (*OAuth)(nil)
	_ CallbackHandler = (*OAuth)(nil)
)

type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	Scope            string `json:"scope"`
	ExpiresIn        int64  `json:"expires_in"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// NewOAuth validates the options and applies production defaults.
func NewOAuth(opts OAuthOptions) (*OAuth, error) {
	clientID := strings.TrimSpace(opts.ClientID)
	if clientID == "" {
		return nil, codedError(CodeInvalidArgument, "the Spotify client ID is empty", nil)
	}
	redirectURI := strings.TrimSpace(opts.RedirectURI)
	if redirectURI == "" {
		return nil, codedError(CodeInvalidArgument, "the Spotify redirect URI is empty", nil)
	}
	if opts.Store == nil {
		return nil, codedError(CodeInvalidArgument, "the Spotify token store is not configured", nil)
	}
	scopes := append([]string(nil), opts.Scopes...)
	if len(scopes) == 0 {
		scopes = append([]string(nil), DefaultScopes...)
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{Timeout: tokenRequestTimeout}
	}
	if opts.AuthorizeURL == "" {
		opts.AuthorizeURL = defaultAuthorizeURL
	}
	if opts.TokenURL == "" {
		opts.TokenURL = defaultTokenURL
	}
	if opts.OpenURL == nil {
		opts.OpenURL = openInBrowser
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.LoginTimeout <= 0 {
		opts.LoginTimeout = defaultLoginTimeout
	}
	return &OAuth{
		clientID:     clientID,
		redirectURI:  redirectURI,
		scopes:       scopes,
		store:        opts.Store,
		client:       opts.HTTPClient,
		authorizeURL: opts.AuthorizeURL,
		tokenURL:     opts.TokenURL,
		openURL:      opts.OpenURL,
		now:          opts.Now,
		loginTimeout: opts.LoginTimeout,
	}, nil
}

// SetClientID switches the Spotify app used by subsequent login and refresh
// operations. The current session is bound to the previous Client ID and will
// be treated as signed out.
func (o *OAuth) SetClientID(clientID string) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return
	}
	o.clientMu.Lock()
	o.clientID = clientID
	o.clientMu.Unlock()
}

func (o *OAuth) currentClientID() string {
	o.clientMu.RLock()
	defer o.clientMu.RUnlock()
	return o.clientID
}

// Login starts one Authorization Code + PKCE login. It returns
// ErrLoginInProgress when another login is already waiting and ErrLoginTimeout
// when the browser callback never arrives.
func (o *OAuth) Login(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	verifier, err := randomToken(64)
	if err != nil {
		return codedError(CodeLoginRequired, "cannot create the Spotify login challenge", err)
	}
	state, err := randomToken(32)
	if err != nil {
		return codedError(CodeLoginRequired, "cannot create the Spotify login state", err)
	}
	attempt := &loginAttempt{
		state:    state,
		verifier: verifier,
		done:     make(chan error, 1),
	}

	o.mu.Lock()
	if o.pending != nil {
		o.mu.Unlock()
		return ErrLoginInProgress
	}
	o.pending = attempt
	o.mu.Unlock()

	authorize := o.authorizeURLFor(state, pkceChallenge(verifier))
	// The authorize URL query carries the PKCE challenge and state. It is
	// handed to the opener only and never logged.
	log.Printf("webplayer: starting the Spotify authorization flow")
	if err := o.openURL(authorize); err != nil {
		if o.clearPending(attempt) {
			return fmt.Errorf("cannot open the Spotify login page: %w", err)
		}
		// A custom opener may have completed the callback before reporting an
		// error; prefer the real result in that case.
		return <-attempt.done
	}

	timer := time.NewTimer(o.loginTimeout)
	defer timer.Stop()
	select {
	case err := <-attempt.done:
		return err
	case <-timer.C:
		if o.clearPending(attempt) {
			return ErrLoginTimeout
		}
		select {
		case err := <-attempt.done:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	case <-ctx.Done():
		if o.clearPending(attempt) {
			return ctx.Err()
		}
		select {
		case err := <-attempt.done:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// HandleCallback completes the pending login with an authorization code. The
// exchange is synchronous so the callback page can render success or failure.
func (o *OAuth) HandleCallback(ctx context.Context, code, state string) error {
	o.mu.Lock()
	attempt := o.pending
	if attempt == nil {
		o.mu.Unlock()
		return ErrInvalidState
	}
	if subtle.ConstantTimeCompare([]byte(state), []byte(attempt.state)) != 1 {
		o.mu.Unlock()
		return ErrInvalidState
	}
	o.pending = nil
	o.mu.Unlock()

	if ctx == nil {
		ctx = context.Background()
	}
	// The HTTP request may be cancelled when the browser tab closes; the code
	// exchange must finish so the session is not lost.
	exchangeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), tokenRequestTimeout)
	defer cancel()
	err := o.exchange(exchangeCtx, code, attempt.verifier)
	attempt.done <- err
	return err
}

// Logout removes the stored session.
func (o *OAuth) Logout() error {
	if o == nil || o.store == nil {
		return nil
	}
	return o.store.Clear()
}

// LoggedIn reports whether a refreshable session is stored.
func (o *OAuth) LoggedIn() bool {
	if o == nil || o.store == nil {
		return false
	}
	tokens, err := o.store.Load()
	return err == nil && tokens.RefreshToken != "" && tokens.ClientID == o.currentClientID()
}

// AccessToken returns the current access token, refreshing it when it expires
// within the safety margin.
func (o *OAuth) AccessToken(ctx context.Context) (string, time.Time, error) {
	if o == nil || o.store == nil {
		return "", time.Time{}, ErrLoginRequired
	}
	tokens, err := o.store.Load()
	if err != nil {
		if errors.Is(err, ErrNoTokens) {
			return "", time.Time{}, ErrLoginRequired
		}
		return "", time.Time{}, err
	}
	if tokens.ClientID != o.currentClientID() {
		return "", time.Time{}, ErrLoginRequired
	}
	if tokens.AccessToken != "" && tokens.Expiry.After(o.now().Add(accessTokenMargin)) {
		return tokens.AccessToken, tokens.Expiry, nil
	}
	return o.Refresh(ctx)
}

// Refresh forces a token refresh and returns the new access token.
func (o *OAuth) Refresh(ctx context.Context) (string, time.Time, error) {
	o.refreshMu.Lock()
	defer o.refreshMu.Unlock()

	tokens, err := o.store.Load()
	if err != nil {
		if errors.Is(err, ErrNoTokens) {
			return "", time.Time{}, ErrLoginRequired
		}
		return "", time.Time{}, err
	}
	if tokens.RefreshToken == "" || tokens.ClientID != o.currentClientID() {
		return "", time.Time{}, ErrLoginRequired
	}

	response, err := o.requestToken(ctx, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {tokens.RefreshToken},
		"client_id":     {o.currentClientID()},
	})
	if err != nil {
		if errors.Is(err, ErrLoginRequired) {
			_ = o.store.Clear()
		}
		return "", time.Time{}, err
	}
	if response.AccessToken == "" {
		return "", time.Time{}, codedError(CodeAPI, "Spotify returned an empty access token", nil)
	}

	refreshed := Tokens{
		AccessToken:  response.AccessToken,
		RefreshToken: tokens.RefreshToken,
		TokenType:    tokens.TokenType,
		Scope:        tokens.Scope,
		Expiry:       o.now().Add(time.Duration(response.ExpiresIn) * time.Second),
		ClientID:     tokens.ClientID,
		Account:      tokens.Account,
	}
	if response.RefreshToken != "" {
		refreshed.RefreshToken = response.RefreshToken
	}
	if response.TokenType != "" {
		refreshed.TokenType = response.TokenType
	}
	if response.Scope != "" {
		refreshed.Scope = response.Scope
	}
	if refreshed.TokenType == "" {
		refreshed.TokenType = "Bearer"
	}
	if err := o.store.Save(refreshed); err != nil {
		return "", time.Time{}, err
	}
	return refreshed.AccessToken, refreshed.Expiry, nil
}

func (o *OAuth) exchange(ctx context.Context, code, verifier string) error {
	response, err := o.requestToken(ctx, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {o.redirectURI},
		"client_id":     {o.currentClientID()},
		"code_verifier": {verifier},
	})
	if err != nil {
		return err
	}
	if response.AccessToken == "" || response.RefreshToken == "" {
		return codedError(CodeAPI, "Spotify returned an incomplete session", nil)
	}
	tokens := Tokens{
		AccessToken:  response.AccessToken,
		RefreshToken: response.RefreshToken,
		TokenType:    response.TokenType,
		Scope:        response.Scope,
		Expiry:       o.now().Add(time.Duration(response.ExpiresIn) * time.Second),
		ClientID:     o.currentClientID(),
	}
	if tokens.TokenType == "" {
		tokens.TokenType = "Bearer"
	}
	if tokens.Scope == "" {
		tokens.Scope = strings.Join(o.scopes, " ")
	}
	if err := o.store.Save(tokens); err != nil {
		return err
	}
	return nil
}

func (o *OAuth) requestToken(ctx context.Context, form url.Values) (tokenResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, o.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return tokenResponse{}, codedError(CodeAPI, "cannot prepare the Spotify token request", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")

	response, err := o.client.Do(request)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return tokenResponse{}, ctxErr
		}
		return tokenResponse{}, codedError(CodeAPI, "cannot reach the Spotify token endpoint", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, maxTokenResponse))
	if err != nil {
		return tokenResponse{}, codedError(CodeAPI, "cannot read the Spotify token response", err)
	}
	var payload tokenResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return tokenResponse{}, codedError(CodeAPI, "Spotify returned an invalid token response", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if payload.Error == "invalid_grant" {
			return tokenResponse{}, ErrLoginRequired
		}
		return tokenResponse{}, codedError(CodeAPI, "Spotify rejected the token request", nil)
	}
	return payload, nil
}

func (o *OAuth) authorizeURLFor(state, challenge string) string {
	target, err := url.Parse(o.authorizeURL)
	if err != nil {
		// NewOAuth receives a caller-provided URL; fall back to the default.
		target, _ = url.Parse(defaultAuthorizeURL)
	}
	query := target.Query()
	query.Set("client_id", o.currentClientID())
	query.Set("response_type", "code")
	query.Set("redirect_uri", o.redirectURI)
	query.Set("scope", strings.Join(o.scopes, " "))
	query.Set("code_challenge_method", "S256")
	query.Set("code_challenge", challenge)
	query.Set("state", state)
	query.Set("show_dialog", "false")
	target.RawQuery = query.Encode()
	return target.String()
}

// clearPending removes attempt if it is still the active login and reports
// whether it did.
func (o *OAuth) clearPending(attempt *loginAttempt) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.pending != attempt {
		return false
	}
	o.pending = nil
	return true
}

func randomToken(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// openInBrowser starts xdg-open without waiting for it. Output is discarded so
// no browser diagnostics ever reach Cadence's log.
func openInBrowser(target string) error {
	command := exec.Command("xdg-open", target)
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	if err := command.Start(); err != nil {
		return err
	}
	go func() {
		_ = command.Wait()
	}()
	return nil
}
