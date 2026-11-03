package spotify

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultListenAddress is the fixed loopback address registered in the
	// Spotify Developer Dashboard.
	DefaultListenAddress = "127.0.0.1:43821"
	// DefaultRedirectURI must match the dashboard entry exactly.
	DefaultRedirectURI = "http://127.0.0.1:43821/callback"

	defaultAuthorizeURL = "https://accounts.spotify.com/authorize"
	defaultTokenURL     = "https://accounts.spotify.com/api/token"

	authScopes = "user-read-playback-state user-modify-playback-state user-read-currently-playing"
)

// Tokens are the result of an OAuth exchange. They are never sent to the
// frontend and must never be logged.
type Tokens struct {
	AccessToken  string
	RefreshToken string
	Expiry       time.Time
}

// OAuth implements Authorization Code + PKCE for a public Spotify client.
type OAuth struct {
	ClientID      string
	ListenAddress string
	RedirectURI   string
	AuthorizeURL  string
	TokenURL      string
	HTTPClient    *http.Client
	Now           func() time.Time
}

type callbackResult struct {
	code string
	err  error
}

// Login starts the loopback server, opens the system browser and waits for the
// authorization callback. The PKCE verifier and state are held in memory only.
func (o *OAuth) Login(ctx context.Context, openBrowser func(string) error) (Tokens, error) {
	clientID := strings.TrimSpace(o.ClientID)
	if clientID == "" {
		return Tokens{}, errors.New("a Spotify Client ID is required")
	}
	if openBrowser == nil {
		return Tokens{}, errors.New("a browser opener is required")
	}

	listenAddress := o.ListenAddress
	if listenAddress == "" {
		listenAddress = DefaultListenAddress
	}
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		return Tokens{}, fmt.Errorf("cannot listen on %s for the Spotify callback: %w. Close the process using this port or change the registered redirect URI", listenAddress, err)
	}
	defer listener.Close()

	redirectURI := o.RedirectURI
	if redirectURI == "" {
		port := listener.Addr().(*net.TCPAddr).Port
		redirectURI = fmt.Sprintf("http://127.0.0.1:%d/callback", port)
	}

	verifier, err := randomURLToken(64)
	if err != nil {
		return Tokens{}, err
	}
	state, err := randomURLToken(32)
	if err != nil {
		return Tokens{}, err
	}
	challenge := codeChallenge(verifier)

	authorizeURL := o.AuthorizeURL
	if authorizeURL == "" {
		authorizeURL = defaultAuthorizeURL
	}
	authURL, err := url.Parse(authorizeURL)
	if err != nil {
		return Tokens{}, fmt.Errorf("parse Spotify authorization URL: %w", err)
	}
	query := authURL.Query()
	query.Set("client_id", clientID)
	query.Set("response_type", "code")
	query.Set("redirect_uri", redirectURI)
	query.Set("state", state)
	query.Set("scope", authScopes)
	query.Set("code_challenge_method", "S256")
	query.Set("code_challenge", challenge)
	authURL.RawQuery = query.Encode()

	result := make(chan callbackResult, 1)
	var once sync.Once
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(writer http.ResponseWriter, request *http.Request) {
		query := request.URL.Query()
		if subtle.ConstantTimeCompare([]byte(query.Get("state")), []byte(state)) != 1 {
			writeCallbackPage(writer, http.StatusBadRequest, "Authorization failed", "The authorization response was invalid. Return to Cadence and try again.")
			once.Do(func() {
				result <- callbackResult{err: errors.New("Spotify callback state did not match")}
			})
			return
		}
		if query.Get("error") != "" {
			writeCallbackPage(writer, http.StatusBadRequest, "Authorization cancelled", "Spotify did not authorize Cadence. You can close this tab and try again.")
			once.Do(func() {
				result <- callbackResult{err: errors.New("Spotify authorization was denied")}
			})
			return
		}
		code := query.Get("code")
		if code == "" {
			writeCallbackPage(writer, http.StatusBadRequest, "Authorization failed", "Spotify did not return an authorization code.")
			once.Do(func() {
				result <- callbackResult{err: errors.New("Spotify callback did not include a code")}
			})
			return
		}
		writeCallbackPage(writer, http.StatusOK, "Cadence is connected", "You can close this tab and return to Cadence.")
		once.Do(func() {
			result <- callbackResult{code: code}
		})
	})
	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
	}
	serverErrors := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	if err := openBrowser(authURL.String()); err != nil {
		return Tokens{}, fmt.Errorf("open the Spotify authorization page: %w", err)
	}

	var callback callbackResult
	select {
	case <-ctx.Done():
		return Tokens{}, errors.New("Spotify authorization timed out")
	case err := <-serverErrors:
		return Tokens{}, fmt.Errorf("Spotify callback server failed: %w", err)
	case callback = <-result:
	}
	if callback.err != nil {
		return Tokens{}, callback.err
	}
	return o.exchangeCode(ctx, clientID, redirectURI, verifier, callback.code)
}

func (o *OAuth) exchangeCode(ctx context.Context, clientID, redirectURI, verifier, code string) (Tokens, error) {
	tokenURL := o.TokenURL
	if tokenURL == "" {
		tokenURL = defaultTokenURL
	}
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {clientID},
		"code_verifier": {verifier},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return Tokens{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	client := o.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	response, err := client.Do(request)
	if err != nil {
		return Tokens{}, fmt.Errorf("exchange the Spotify authorization code: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if err != nil {
		return Tokens{}, fmt.Errorf("read the Spotify token response: %w", err)
	}
	var payload tokenResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return Tokens{}, fmt.Errorf("invalid Spotify token response: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		if payload.Error == "invalid_grant" || payload.Error == "invalid_client" {
			return Tokens{}, ErrAuthRevoked
		}
		return Tokens{}, fmt.Errorf("Spotify token endpoint returned status %d", response.StatusCode)
	}
	if payload.AccessToken == "" || payload.RefreshToken == "" {
		return Tokens{}, errors.New("Spotify token response did not include both tokens")
	}
	now := time.Now
	if o.Now != nil {
		now = o.Now
	}
	return Tokens{
		AccessToken:  payload.AccessToken,
		RefreshToken: payload.RefreshToken,
		Expiry:       now().Add(time.Duration(payload.ExpiresIn) * time.Second),
	}, nil
}

func writeCallbackPage(writer http.ResponseWriter, status int, title, message string) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_, _ = io.WriteString(writer, `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>`+
		html.EscapeString(title)+`</title><style>body{margin:0;min-height:100vh;display:grid;place-items:center;background:#090b0f;color:#e8edf4;font:16px/1.5 system-ui,sans-serif}main{max-width:34rem;padding:2rem}h1{margin:0 0 .6rem;font-size:1.35rem}p{margin:0;color:#8a94a6}a{color:#5ef0ff}</style></head><body><main><h1>`+
		html.EscapeString(title)+`</h1><p>`+html.EscapeString(message)+`</p></main></body></html>`)
}

type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int    `json:"expires_in"`
	TokenType        string `json:"token_type"`
	Scope            string `json:"scope"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func randomURLToken(bytesCount int) (string, error) {
	buffer := make([]byte, bytesCount)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate PKCE randomness: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func codeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
