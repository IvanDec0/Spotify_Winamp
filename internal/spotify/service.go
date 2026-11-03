package spotify

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"cadence/internal/artcache"
	"cadence/internal/keyring"
	"cadence/internal/player"
	"cadence/internal/settings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// AuthEvent is emitted whenever the Spotify account state changes.
const AuthEvent = "spotify:auth"

const authTimeout = 5 * time.Minute

// AuthStatus is the account state shown by the onboarding panel.
type AuthStatus string

const (
	AuthChecking        AuthStatus = "checking"
	AuthUnauthenticated AuthStatus = "unauthenticated"
	AuthAuthenticating  AuthStatus = "authenticating"
	AuthAuthenticated   AuthStatus = "authenticated"
	AuthError           AuthStatus = "error"
)

// AuthState is token-free account information safe for the frontend.
type AuthState struct {
	Status           AuthStatus `json:"status"`
	ClientID         string     `json:"clientId"`
	RedirectURI      string     `json:"redirectUri"`
	KeyringAvailable bool       `json:"keyringAvailable"`
	PlayerMode       string     `json:"playerMode"`
	Error            string     `json:"error"`
}

// Options wires the Spotify service to the rest of the application.
type Options struct {
	Store       *settings.Store
	Keyring     keyring.Store
	Player      *player.Service
	Demo        player.Provider
	Art         *artcache.Cache
	ForceDemo   bool
	OpenBrowser func(string) error
	OAuth       *OAuth
	APIBaseURL  string
}

// Service owns the Spotify session. It never exposes access or refresh tokens.
type Service struct {
	store       *settings.Store
	keyring     keyring.Store
	player      *player.Service
	demo        player.Provider
	art         *artcache.Cache
	forceDemo   bool
	openBrowser func(string) error
	oauthBase   *OAuth
	apiBaseURL  string

	mu            sync.Mutex
	app           *application.App
	state         AuthState
	clientID      string
	provider      *Provider
	tokenSource   *TokenSource
	cancelAuth    context.CancelFunc
	loginInFlight bool
	authAttempt   uint64
}

func NewService(options Options) *Service {
	service := &Service{
		store:       options.Store,
		keyring:     options.Keyring,
		player:      options.Player,
		demo:        options.Demo,
		art:         options.Art,
		forceDemo:   options.ForceDemo,
		openBrowser: options.OpenBrowser,
		oauthBase:   options.OAuth,
		apiBaseURL:  options.APIBaseURL,
	}
	service.state = AuthState{
		Status:           AuthChecking,
		RedirectURI:      DefaultRedirectURI,
		KeyringAvailable: true,
		PlayerMode:       "demo",
	}
	if options.Store != nil {
		if config, err := options.Store.Load(); err == nil {
			service.clientID = strings.TrimSpace(config.Spotify.ClientID)
		}
	}
	if env := strings.TrimSpace(os.Getenv("CADENCE_SPOTIFY_CLIENT_ID")); env != "" {
		service.clientID = env
	}
	service.state.ClientID = service.clientID
	return service
}

func (s *Service) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	s.mu.Lock()
	s.app = application.Get()
	s.mu.Unlock()
	go s.restore()
	return nil
}

func (s *Service) ServiceShutdown() error {
	s.mu.Lock()
	if s.cancelAuth != nil {
		s.cancelAuth()
	}
	s.mu.Unlock()
	return nil
}

// GetAuthState returns the current, token-free account state.
func (s *Service) GetAuthState() AuthState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// SetClientID persists the public Spotify Client ID. Changing an existing ID
// clears the old session because refresh tokens are issued per client.
func (s *Service) SetClientID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("Spotify Client ID cannot be empty")
	}
	if len(id) > 128 || strings.IndexFunc(id, func(r rune) bool { return r == ' ' || r == '\t' || r == '\n' }) >= 0 {
		return errors.New("Spotify Client ID contains invalid characters")
	}

	s.mu.Lock()
	if id == s.clientID {
		s.mu.Unlock()
		return s.store.UpdateSpotify(func(config *settings.SpotifyState) error {
			config.ClientID = id
			return nil
		})
	}
	s.authAttempt++
	if s.cancelAuth != nil {
		s.cancelAuth()
		s.cancelAuth = nil
	}
	s.loginInFlight = false
	s.clientID = id
	s.state.ClientID = id
	s.state.Status = AuthUnauthenticated
	s.state.Error = ""
	s.state.PlayerMode = "demo"
	s.provider = nil
	s.tokenSource = nil
	if s.player != nil && s.demo != nil {
		s.player.SetProvider(s.demo)
	}
	s.mu.Unlock()

	_ = s.keyring.DeleteRefreshToken()
	if err := s.store.UpdateSpotify(func(config *settings.SpotifyState) error {
		config.ClientID = id
		config.DemoMode = false
		return nil
	}); err != nil {
		return err
	}
	s.emitAuth()
	return nil
}

// Login starts Authorization Code + PKCE in the system browser and returns
// immediately. The frontend observes spotify:auth for completion.
func (s *Service) Login() error {
	s.mu.Lock()
	if s.loginInFlight {
		s.mu.Unlock()
		return errors.New("Spotify authorization is already in progress")
	}
	if s.clientID == "" {
		s.mu.Unlock()
		return errors.New("set a Spotify Client ID before connecting")
	}
	if err := s.keyring.Available(); err != nil {
		s.state.KeyringAvailable = false
		s.state.Status = AuthError
		s.state.Error = err.Error()
		s.mu.Unlock()
		s.emitAuth()
		return err
	}
	s.authAttempt++
	attempt := s.authAttempt
	ctx, cancel := context.WithTimeout(context.Background(), authTimeout)
	s.cancelAuth = cancel
	s.loginInFlight = true
	s.state.Status = AuthAuthenticating
	s.state.Error = ""
	s.state.KeyringAvailable = true
	s.state.PlayerMode = "demo"
	clientID := s.clientID
	s.mu.Unlock()
	s.emitAuth()

	go s.finishLogin(ctx, attempt, clientID)
	return nil
}

func (s *Service) finishLogin(ctx context.Context, attempt uint64, clientID string) {
	oauth := OAuth{
		ClientID:      clientID,
		ListenAddress: DefaultListenAddress,
		RedirectURI:   DefaultRedirectURI,
	}
	if s.oauthBase != nil {
		oauth = *s.oauthBase
		oauth.ClientID = clientID
		if oauth.ListenAddress == "" {
			oauth.ListenAddress = DefaultListenAddress
		}
	}
	openBrowser := s.openBrowser
	if openBrowser == nil {
		openBrowser = func(target string) error {
			app := application.Get()
			if app == nil {
				return errors.New("application is not ready")
			}
			return app.Browser.OpenURL(target)
		}
	}
	tokens, err := oauth.Login(ctx, openBrowser)
	if err != nil {
		s.failLogin(attempt, err)
		return
	}
	s.mu.Lock()
	stale := attempt != s.authAttempt || ctx.Err() != nil
	s.mu.Unlock()
	if stale {
		return
	}
	if err := s.keyring.SetRefreshToken(tokens.RefreshToken); err != nil {
		s.failLogin(attempt, fmt.Errorf("could not save the Spotify session: %w", err))
		return
	}
	tokenSource := NewTokenSource(clientID, tokens.RefreshToken, oauth.TokenURL, oauth.HTTPClient, func(token string) error {
		return s.keyring.SetRefreshToken(token)
	})
	tokenSource.Seed(tokens.AccessToken, tokens.RefreshToken, tokens.Expiry)
	provider := NewProvider(s.newClient(tokenSource, oauth.HTTPClient), s.art, s.handleAuthProblem)

	s.mu.Lock()
	if attempt != s.authAttempt || ctx.Err() != nil {
		s.mu.Unlock()
		_ = s.keyring.DeleteRefreshToken()
		return
	}
	s.tokenSource = tokenSource
	s.provider = provider
	s.loginInFlight = false
	s.cancelAuth = nil
	s.state.Status = AuthAuthenticated
	s.state.Error = ""
	s.state.PlayerMode = "spotify"
	s.state.KeyringAvailable = true
	s.mu.Unlock()
	if s.player != nil {
		s.player.SetProvider(provider)
	}
	_ = s.store.UpdateSpotify(func(config *settings.SpotifyState) error {
		config.ClientID = clientID
		config.DemoMode = false
		return nil
	})
	s.emitAuth()
}

func (s *Service) failLogin(attempt uint64, err error) {
	s.mu.Lock()
	if attempt != s.authAttempt {
		s.mu.Unlock()
		return
	}
	s.loginInFlight = false
	if s.cancelAuth != nil {
		s.cancelAuth()
		s.cancelAuth = nil
	}
	s.state.Status = AuthError
	s.state.Error = authErrorMessage(err)
	s.state.PlayerMode = "demo"
	s.mu.Unlock()
	if s.player != nil && s.demo != nil {
		s.player.SetProvider(s.demo)
	}
	s.emitAuth()
}

func (s *Service) restore() {
	s.mu.Lock()
	clientID := s.clientID
	forceDemo := s.forceDemo
	s.mu.Unlock()

	if forceDemo {
		s.mu.Lock()
		s.state.Status = AuthUnauthenticated
		s.state.PlayerMode = "demo"
		s.mu.Unlock()
		s.emitAuth()
		return
	}

	if err := s.keyring.Available(); err != nil {
		s.mu.Lock()
		s.state.KeyringAvailable = false
		s.state.Status = AuthError
		s.state.Error = err.Error()
		s.state.PlayerMode = "demo"
		s.mu.Unlock()
		s.emitAuth()
		return
	}
	refreshToken, err := s.keyring.GetRefreshToken()
	if errors.Is(err, keyring.ErrNotFound) {
		s.mu.Lock()
		s.state.Status = AuthUnauthenticated
		s.state.PlayerMode = "demo"
		s.mu.Unlock()
		s.emitAuth()
		return
	}
	if err != nil {
		s.mu.Lock()
		s.state.Status = AuthError
		s.state.Error = authErrorMessage(err)
		s.state.PlayerMode = "demo"
		s.mu.Unlock()
		s.emitAuth()
		return
	}
	if clientID == "" {
		s.mu.Lock()
		s.state.Status = AuthUnauthenticated
		s.state.PlayerMode = "demo"
		s.state.Error = "Spotify is authorized but the Client ID is missing. Enter it in the account panel."
		s.mu.Unlock()
		s.emitAuth()
		return
	}

	tokenURL := ""
	var httpClient = (*http.Client)(nil)
	if s.oauthBase != nil {
		tokenURL = s.oauthBase.TokenURL
		httpClient = s.oauthBase.HTTPClient
	}
	tokenSource := NewTokenSource(clientID, refreshToken, tokenURL, httpClient, func(token string) error {
		return s.keyring.SetRefreshToken(token)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := tokenSource.AccessToken(ctx); err != nil {
		if errors.Is(err, ErrAuthRevoked) {
			_ = s.keyring.DeleteRefreshToken()
			s.mu.Lock()
			s.state.Status = AuthError
			s.state.Error = "Spotify session expired. Connect again."
			s.state.PlayerMode = "demo"
			s.mu.Unlock()
			s.emitAuth()
			return
		}
		s.mu.Lock()
		s.state.Status = AuthError
		s.state.Error = authErrorMessage(err)
		s.state.PlayerMode = "demo"
		s.mu.Unlock()
		s.emitAuth()
		return
	}
	provider := NewProvider(s.newClient(tokenSource, httpClient), s.art, s.handleAuthProblem)

	config, configErr := s.store.Load()
	demoMode := configErr == nil && config.Spotify.DemoMode
	s.mu.Lock()
	if s.authAttempt != 0 {
		s.mu.Unlock()
		return
	}
	s.tokenSource = tokenSource
	s.provider = provider
	s.state.Status = AuthAuthenticated
	s.state.Error = ""
	s.state.KeyringAvailable = true
	if demoMode {
		s.state.PlayerMode = "demo"
	} else {
		s.state.PlayerMode = "spotify"
	}
	s.mu.Unlock()
	if !demoMode && s.player != nil {
		s.player.SetProvider(provider)
	}
	s.emitAuth()
}

// Logout removes the refresh token and returns to the demo player.
func (s *Service) Logout() error {
	s.mu.Lock()
	s.authAttempt++
	if s.cancelAuth != nil {
		s.cancelAuth()
		s.cancelAuth = nil
	}
	s.loginInFlight = false
	s.provider = nil
	s.tokenSource = nil
	s.state.Status = AuthUnauthenticated
	s.state.Error = ""
	s.state.PlayerMode = "demo"
	s.mu.Unlock()
	if s.player != nil && s.demo != nil {
		s.player.SetProvider(s.demo)
	}
	_ = s.keyring.DeleteRefreshToken()
	_ = s.store.UpdateSpotify(func(config *settings.SpotifyState) error {
		config.DemoMode = false
		return nil
	})
	s.emitAuth()
	return nil
}

// UseDemo keeps the Spotify session but switches playback to the demo player.
func (s *Service) UseDemo() error {
	s.mu.Lock()
	s.state.PlayerMode = "demo"
	s.mu.Unlock()
	if s.player != nil && s.demo != nil {
		s.player.SetProvider(s.demo)
	}
	_ = s.store.UpdateSpotify(func(config *settings.SpotifyState) error {
		config.DemoMode = true
		return nil
	})
	s.emitAuth()
	return nil
}

// UseSpotify switches back to an authenticated Spotify session.
func (s *Service) UseSpotify() error {
	s.mu.Lock()
	if s.state.Status != AuthAuthenticated || s.provider == nil {
		s.mu.Unlock()
		return errors.New("connect Spotify before using it")
	}
	provider := s.provider
	s.state.PlayerMode = "spotify"
	s.mu.Unlock()
	if s.player != nil {
		s.player.SetProvider(provider)
	}
	_ = s.store.UpdateSpotify(func(config *settings.SpotifyState) error {
		config.DemoMode = false
		return nil
	})
	s.emitAuth()
	return nil
}

func (s *Service) handleAuthProblem(err error) {
	s.mu.Lock()
	if s.state.Status == AuthUnauthenticated && s.provider == nil {
		s.mu.Unlock()
		return
	}
	s.authAttempt++
	s.provider = nil
	s.tokenSource = nil
	s.loginInFlight = false
	if s.cancelAuth != nil {
		s.cancelAuth()
		s.cancelAuth = nil
	}
	s.state.Status = AuthError
	s.state.Error = "Spotify session expired. Connect again."
	s.state.PlayerMode = "demo"
	s.mu.Unlock()
	if s.player != nil && s.demo != nil {
		s.player.SetProvider(s.demo)
	}
	_ = s.keyring.DeleteRefreshToken()
	s.emitAuth()
}

func (s *Service) newClient(tokens *TokenSource, httpClient *http.Client) *Client {
	if s.apiBaseURL != "" {
		return NewClientWithBase(tokens, s.apiBaseURL, httpClient)
	}
	return NewClient(tokens, httpClient)
}

func (s *Service) emitAuth() {
	s.mu.Lock()
	app := s.app
	state := s.state
	s.mu.Unlock()
	if app != nil {
		app.Event.Emit(AuthEvent, state)
	}
}

func authErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, ErrAuthRevoked) {
		return "Spotify authorization was revoked. Connect again."
	}
	if errors.Is(err, ErrNotAuthenticated) {
		return "Spotify is not connected."
	}
	message := err.Error()
	if len(message) > 400 {
		message = message[:400]
	}
	return message
}
