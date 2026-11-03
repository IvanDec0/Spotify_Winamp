package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestPKCEChallenge(t *testing.T) {
	verifier := "test-verifier"
	challenge := codeChallenge(verifier)
	if challenge == verifier || len(challenge) < 40 {
		t.Fatalf("challenge = %q", challenge)
	}
	random, err := randomURLToken(64)
	if err != nil {
		t.Fatal(err)
	}
	if len(random) < 43 || len(random) > 128 {
		t.Fatalf("verifier length = %d", len(random))
	}
}

func TestOAuthLoginFlow(t *testing.T) {
	challengeCh := make(chan string, 1)
	tokenServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("token method = %s", request.Method)
		}
		if err := request.ParseForm(); err != nil {
			t.Errorf("ParseForm: %v", err)
		}
		if request.Form.Get("grant_type") != "authorization_code" {
			t.Errorf("grant_type = %q", request.Form.Get("grant_type"))
		}
		if request.Form.Get("code") != "test-code" {
			t.Errorf("code = %q", request.Form.Get("code"))
		}
		challenge := <-challengeCh
		if got := codeChallenge(request.Form.Get("code_verifier")); got != challenge {
			t.Errorf("PKCE verifier challenge = %q, want %q", got, challenge)
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"access_token":  "access-token",
			"refresh_token": "refresh-token",
			"expires_in":    3600,
			"token_type":    "Bearer",
		})
	}))
	defer tokenServer.Close()

	oauth := &OAuth{
		ClientID:      "client-id",
		ListenAddress: "127.0.0.1:0",
		AuthorizeURL:  "https://accounts.example.test/authorize",
		TokenURL:      tokenServer.URL,
	}
	openBrowser := func(authURL string) error {
		parsed, err := url.Parse(authURL)
		if err != nil {
			return err
		}
		query := parsed.Query()
		challengeCh <- query.Get("code_challenge")
		redirect, err := url.Parse(query.Get("redirect_uri"))
		if err != nil {
			return err
		}
		callback := *redirect
		callbackQuery := callback.Query()
		callbackQuery.Set("code", "test-code")
		callbackQuery.Set("state", query.Get("state"))
		callback.RawQuery = callbackQuery.Encode()
		response, err := http.Get(callback.String())
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return errors.New("callback page failed")
		}
		return nil
	}

	tokens, err := oauth.Login(context.Background(), openBrowser)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if tokens.AccessToken != "access-token" || tokens.RefreshToken != "refresh-token" {
		t.Fatalf("tokens = %#v", tokens)
	}
	if time.Until(tokens.Expiry) < 50*time.Minute {
		t.Fatalf("expiry = %v", tokens.Expiry)
	}
}

func TestOAuthRejectsStateMismatch(t *testing.T) {
	var tokenCalls atomic.Int32
	tokenServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		tokenCalls.Add(1)
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	defer tokenServer.Close()

	oauth := &OAuth{
		ClientID:      "client-id",
		ListenAddress: "127.0.0.1:0",
		TokenURL:      tokenServer.URL,
	}
	openBrowser := func(authURL string) error {
		parsed, _ := url.Parse(authURL)
		redirect, _ := url.Parse(parsed.Query().Get("redirect_uri"))
		query := redirect.Query()
		query.Set("code", "test-code")
		query.Set("state", "wrong-state")
		redirect.RawQuery = query.Encode()
		response, err := http.Get(redirect.String())
		if err == nil {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
		}
		return err
	}
	_, err := oauth.Login(context.Background(), openBrowser)
	if err == nil || !strings.Contains(err.Error(), "state") {
		t.Fatalf("Login() error = %v", err)
	}
	if tokenCalls.Load() != 0 {
		t.Fatal("token endpoint was called after state mismatch")
	}
}

func TestOAuthTokenExchangeRevoked(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(writer).Encode(map[string]string{"error": "invalid_grant"})
	}))
	defer tokenServer.Close()

	oauth := &OAuth{ClientID: "client-id", ListenAddress: "127.0.0.1:0", TokenURL: tokenServer.URL}
	openBrowser := func(authURL string) error {
		parsed, _ := url.Parse(authURL)
		redirect, _ := url.Parse(parsed.Query().Get("redirect_uri"))
		query := redirect.Query()
		query.Set("code", "test-code")
		query.Set("state", parsed.Query().Get("state"))
		redirect.RawQuery = query.Encode()
		response, err := http.Get(redirect.String())
		if err == nil {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
		}
		return err
	}
	_, err := oauth.Login(context.Background(), openBrowser)
	if !errors.Is(err, ErrAuthRevoked) {
		t.Fatalf("Login() error = %v, want ErrAuthRevoked", err)
	}
}
