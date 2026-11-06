package webplayer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

type fakeTokenSource struct {
	token  string
	expiry time.Time
	err    error
}

func (f fakeTokenSource) AccessToken(context.Context) (string, time.Time, error) {
	return f.token, f.expiry, f.err
}

type fakeCallbackHandler struct {
	code   string
	state  string
	called int
	err    error
}

func (f *fakeCallbackHandler) HandleCallback(_ context.Context, code, state string) error {
	f.called++
	f.code = code
	f.state = state
	return f.err
}

func serverBase(t *testing.T, server *Server) (string, string) {
	t.Helper()
	parsed, err := url.Parse(server.PlayerURL())
	if err != nil {
		t.Fatalf("parse player URL: %v", err)
	}
	return parsed.Scheme + "://" + parsed.Host, parsed.Query().Get("nonce")
}

func getURL(t *testing.T, rawURL string) (*http.Response, []byte) {
	t.Helper()
	response, err := http.Get(rawURL)
	if err != nil {
		t.Fatalf("GET %s: %v", rawURL, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read %s: %v", rawURL, err)
	}
	return response, body
}

func TestServerPlayer(t *testing.T) {
	server, err := NewServer(ServerOptions{
		Nonce:  "test-nonce",
		Page:   []byte("<!doctype html><title>player</title>"),
		Script: []byte(`window.glue = true;`),
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	defer server.Close()
	base, nonce := serverBase(t, server)
	if nonce != "test-nonce" {
		t.Fatalf("nonce = %q", nonce)
	}

	response, _ := getURL(t, base+"/player")
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("missing nonce status = %d, want 403", response.StatusCode)
	}
	response, _ = getURL(t, base+"/player?nonce=wrong")
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("wrong nonce status = %d, want 403", response.StatusCode)
	}

	response, body := getURL(t, base+"/player?nonce="+url.QueryEscape(nonce))
	if response.StatusCode != http.StatusOK {
		t.Fatalf("player status = %d", response.StatusCode)
	}
	if string(body) != "<!doctype html><title>player</title>" {
		t.Fatalf("player body = %q", body)
	}
	if got := response.Header.Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("content type = %q", got)
	}
	if got := response.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache control = %q", got)
	}
	if got := response.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("content type options = %q", got)
	}
	if got := response.Header.Get("Referrer-Policy"); got != "no-referrer" {
		t.Fatalf("referrer policy = %q", got)
	}
	const wantCSP = "default-src 'none'; script-src 'self' https://sdk.scdn.co; frame-src https://sdk.scdn.co; connect-src 'self' https://*.spotify.com wss://*.spotify.com https://*.scdn.co https://*.spotifycdn.com https://*.akamaized.net; img-src https://*.scdn.co https://*.spotifycdn.com data:; media-src https://*.scdn.co https://*.spotifycdn.com https://*.akamaized.net blob:; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"
	if got := response.Header.Get("Content-Security-Policy"); got != wantCSP {
		t.Fatalf("CSP = %q", got)
	}

	response, body = getURL(t, base+"/player.js")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("script status = %d", response.StatusCode)
	}
	if string(body) != "window.glue = true;" {
		t.Fatalf("script body = %q", body)
	}
	if got := response.Header.Get("Content-Type"); got != "application/javascript; charset=utf-8" {
		t.Fatalf("script content type = %q", got)
	}
}

func TestServerToken(t *testing.T) {
	expiry := time.Now().Add(time.Hour)
	server, err := NewServer(ServerOptions{
		Nonce:  "test-nonce",
		Page:   []byte("page"),
		Tokens: fakeTokenSource{token: "access-token-value", expiry: expiry},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	defer server.Close()
	base, _ := serverBase(t, server)

	response, _ := getURL(t, base+"/token")
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("missing nonce status = %d, want 403", response.StatusCode)
	}

	response, body := getURL(t, base+"/token?nonce=test-nonce")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("token status = %d: %s", response.StatusCode, body)
	}
	if got := response.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache control = %q", got)
	}
	var payload struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode token: %v", err)
	}
	if payload.AccessToken != "access-token-value" {
		t.Fatalf("access token = %q", payload.AccessToken)
	}
	if payload.TokenType != "Bearer" {
		t.Fatalf("token type = %q", payload.TokenType)
	}
	if payload.ExpiresIn < 3590 || payload.ExpiresIn > 3600 {
		t.Fatalf("expires_in = %d", payload.ExpiresIn)
	}
}

func TestServerTokenNotReady(t *testing.T) {
	server, err := NewServer(ServerOptions{Nonce: "test-nonce", Page: []byte("page")})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	defer server.Close()
	base, _ := serverBase(t, server)

	response, body := getURL(t, base+"/token?nonce=test-nonce")
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", response.StatusCode)
	}
	var payload map[string]string
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if payload["error"] != "not ready" {
		t.Fatalf("error = %q", payload["error"])
	}
}

func TestServerTokenSourceFailureDoesNotLeak(t *testing.T) {
	server, err := NewServer(ServerOptions{
		Nonce:  "test-nonce",
		Page:   []byte("page"),
		Tokens: fakeTokenSource{err: errors.New("refresh token BQsecretBQsecretBQsecret")},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	defer server.Close()
	base, _ := serverBase(t, server)

	response, body := getURL(t, base+"/token?nonce=test-nonce")
	if response.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", response.StatusCode)
	}
	if strings.Contains(string(body), "BQsecret") {
		t.Fatalf("token source error leaked into the response: %s", body)
	}
}

func TestServerCallback(t *testing.T) {
	t.Run("not available", func(t *testing.T) {
		server, err := NewServer(ServerOptions{Nonce: "test-nonce", Page: []byte("page")})
		if err != nil {
			t.Fatalf("new server: %v", err)
		}
		defer server.Close()
		base, _ := serverBase(t, server)

		response, _ := getURL(t, base+"/callback?code=abc&state=state")
		if response.StatusCode != http.StatusNotImplemented {
			t.Fatalf("status = %d, want 501", response.StatusCode)
		}
	})

	t.Run("spotify error", func(t *testing.T) {
		handler := &fakeCallbackHandler{}
		server, err := NewServer(ServerOptions{Nonce: "test-nonce", Page: []byte("page"), Callback: handler})
		if err != nil {
			t.Fatalf("new server: %v", err)
		}
		defer server.Close()
		base, _ := serverBase(t, server)

		response, _ := getURL(t, base+"/callback?error=access_denied&state=state")
		if response.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", response.StatusCode)
		}
		if handler.called != 0 {
			t.Fatal("the callback handler ran for an error redirect")
		}
	})

	t.Run("missing code", func(t *testing.T) {
		handler := &fakeCallbackHandler{}
		server, err := NewServer(ServerOptions{Nonce: "test-nonce", Page: []byte("page"), Callback: handler})
		if err != nil {
			t.Fatalf("new server: %v", err)
		}
		defer server.Close()
		base, _ := serverBase(t, server)

		response, _ := getURL(t, base+"/callback?state=state")
		if response.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", response.StatusCode)
		}
	})

	t.Run("success", func(t *testing.T) {
		handler := &fakeCallbackHandler{}
		server, err := NewServer(ServerOptions{Nonce: "test-nonce", Page: []byte("page"), Callback: handler})
		if err != nil {
			t.Fatalf("new server: %v", err)
		}
		defer server.Close()
		base, _ := serverBase(t, server)

		response, body := getURL(t, base+"/callback?code=secret-code&state=state-1")
		if response.StatusCode != http.StatusOK {
			t.Fatalf("status = %d: %s", response.StatusCode, body)
		}
		if handler.called != 1 || handler.code != "secret-code" || handler.state != "state-1" {
			t.Fatalf("handler saw code=%q state=%q calls=%d", handler.code, handler.state, handler.called)
		}
		if strings.Contains(string(body), "secret-code") {
			t.Fatalf("the callback page echoed the authorization code: %s", body)
		}
	})

	t.Run("handler failure", func(t *testing.T) {
		handler := &fakeCallbackHandler{err: errors.New("exchange failed")}
		server, err := NewServer(ServerOptions{Nonce: "test-nonce", Page: []byte("page"), Callback: handler})
		if err != nil {
			t.Fatalf("new server: %v", err)
		}
		defer server.Close()
		base, _ := serverBase(t, server)

		response, body := getURL(t, base+"/callback?code=abc&state=state")
		if response.StatusCode != http.StatusBadGateway {
			t.Fatalf("status = %d, want 502", response.StatusCode)
		}
		if strings.Contains(string(body), "exchange failed") {
			t.Fatalf("the handler error leaked into the page: %s", body)
		}
	})
}

func TestServerMethodsAndUnknownPaths(t *testing.T) {
	server, err := NewServer(ServerOptions{Nonce: "test-nonce", Page: []byte("page")})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	defer server.Close()
	base, _ := serverBase(t, server)

	response, err := http.Post(base+"/player?nonce=test-nonce", "text/plain", strings.NewReader(""))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", response.StatusCode)
	}

	response, _ = getURL(t, base+"/unknown")
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown path status = %d, want 404", response.StatusCode)
	}
}

func TestServerGeneratesNonce(t *testing.T) {
	server, err := NewServer(ServerOptions{Page: []byte("page")})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	defer server.Close()
	base, nonce := serverBase(t, server)
	if len(nonce) < 40 {
		t.Fatalf("generated nonce is too short: %d characters", len(nonce))
	}
	response, _ := getURL(t, base+"/player?nonce="+url.QueryEscape(nonce))
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
}

func TestServerCloseIsIdempotent(t *testing.T) {
	server, err := NewServer(ServerOptions{Nonce: "test-nonce", Page: []byte("page")})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	if err := server.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := server.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
	base, _ := serverBase(t, server)
	if _, err := http.Get(base + "/player?nonce=test-nonce"); err == nil {
		t.Fatal("the server still answers after Close")
	}
}
