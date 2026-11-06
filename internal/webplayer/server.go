package webplayer

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// playerCSP is the Content Security Policy for the SDK page. It allows the
// same-origin glue, the Spotify player SDK, the SDK's embedded player iframe,
// the local origin for its token fetch, and the Spotify media/CDN hosts the
// SDK needs, and nothing else.
const playerCSP = "default-src 'none'; script-src 'self' https://sdk.scdn.co; frame-src https://sdk.scdn.co; connect-src 'self' https://*.spotify.com wss://*.spotify.com https://*.scdn.co https://*.spotifycdn.com https://*.akamaized.net; img-src https://*.scdn.co https://*.spotifycdn.com data:; media-src https://*.scdn.co https://*.spotifycdn.com https://*.akamaized.net blob:; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

const (
	nonceBytes         = 32
	serverReadTimeout  = 5 * time.Second
	serverWriteTimeout = 10 * time.Second
)

// TokenSource supplies a short-lived access token to the SDK page. It is
// never logged and never stored by this package.
type TokenSource interface {
	AccessToken(ctx context.Context) (string, time.Time, error)
}

// CallbackHandler receives an OAuth authorization code from the system
// browser redirect.
type CallbackHandler interface {
	HandleCallback(ctx context.Context, code, state string) error
}

// ServerOptions configures the loopback HTTP server.
type ServerOptions struct {
	Nonce    string
	Page     []byte
	Script   []byte
	Tokens   TokenSource
	Callback CallbackHandler
}

// Server serves the player page, the token endpoint and the OAuth callback on
// 127.0.0.1 only.
type Server struct {
	nonce    string
	page     []byte
	script   []byte
	tokens   TokenSource
	callback CallbackHandler

	listener net.Listener
	http     *http.Server

	closeOnce sync.Once
	closeErr  error
}

// NewServer binds 127.0.0.1:0 and starts serving. A missing nonce is
// generated with crypto/rand.
func NewServer(opts ServerOptions) (*Server, error) {
	nonce := opts.Nonce
	if nonce == "" {
		generated, err := generateNonce()
		if err != nil {
			return nil, fmt.Errorf("webplayer: cannot generate a nonce: %w", err)
		}
		nonce = generated
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("webplayer: cannot bind the local HTTP server: %w", err)
	}
	server := &Server{
		nonce:    nonce,
		page:     append([]byte(nil), opts.Page...),
		script:   append([]byte(nil), opts.Script...),
		tokens:   opts.Tokens,
		callback: opts.Callback,
		listener: listener,
	}
	server.http = &http.Server{
		Handler:           http.HandlerFunc(server.handle),
		ReadHeaderTimeout: serverReadTimeout,
		WriteTimeout:      serverWriteTimeout,
	}
	go func() {
		_ = server.http.Serve(listener)
	}()
	return server, nil
}

// PlayerURL is the URL the helper window loads.
func (s *Server) PlayerURL() string {
	return "http://" + s.listener.Addr().String() + "/player?nonce=" + url.QueryEscape(s.nonce)
}

// Close stops the server. It is safe to call more than once.
func (s *Server) Close() error {
	s.closeOnce.Do(func() {
		s.closeErr = s.http.Close()
	})
	return s.closeErr
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	switch r.URL.Path {
	case "/player":
		s.handlePlayer(w, r)
	case "/player.js":
		s.handleScript(w, r)
	case "/token":
		s.handleToken(w, r)
	case "/callback":
		s.handleCallback(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handlePlayer(w http.ResponseWriter, r *http.Request) {
	if !s.nonceOK(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", playerCSP)
	_, _ = w.Write(s.page)
}

// handleScript serves the page glue from the same origin so the page needs no
// inline script and the Content Security Policy can stay strict. The script is
// public and static; it carries no nonce or token.
func (s *Server) handleScript(w http.ResponseWriter, r *http.Request) {
	if len(s.script) == 0 {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(s.script)
}

func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	if !s.nonceOK(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if s.tokens == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "not ready"})
		return
	}
	token, expiry, err := s.tokens.AccessToken(r.Context())
	if err != nil {
		// Never include the error: token sources may wrap credential data.
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "token unavailable"})
		return
	}
	expiresIn := int64(time.Until(expiry).Seconds())
	if expiresIn < 0 {
		expiresIn = 0
	}
	writeJSON(w, http.StatusOK, struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   expiresIn,
	})
}

func (s *Server) handleCallback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if s.callback == nil {
		writeHTML(w, http.StatusNotImplemented, "Not available",
			"Spotify authorization is not available yet.")
		return
	}
	if query.Get("error") != "" {
		writeHTML(w, http.StatusBadRequest, "Authorization failed",
			"Spotify did not authorize Cadence. Return to Cadence and try again.")
		return
	}
	code := query.Get("code")
	if code == "" {
		writeHTML(w, http.StatusBadRequest, "Authorization failed",
			"Cadence did not receive an authorization code. Return to Cadence and try again.")
		return
	}
	if err := s.callback.HandleCallback(r.Context(), code, query.Get("state")); err != nil {
		writeHTML(w, http.StatusBadGateway, "Authorization failed",
			"Cadence could not complete the Spotify authorization. Return to Cadence and try again.")
		return
	}
	writeHTML(w, http.StatusOK, "Authorization complete",
		"Spotify is connected. You can close this tab and return to Cadence.")
}

func (s *Server) nonceOK(r *http.Request) bool {
	got := r.URL.Query().Get("nonce")
	return subtle.ConstantTimeCompare([]byte(got), []byte(s.nonce)) == 1
}

func generateNonce() (string, error) {
	buffer := make([]byte, nonceBytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func writeHTML(w http.ResponseWriter, status int, title, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.WriteHeader(status)
	fmt.Fprintf(w, "<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\"><title>%s</title></head><body><main><h1>%s</h1><p>%s</p></main></body></html>",
		html.EscapeString(title), html.EscapeString(title), html.EscapeString(message))
}
