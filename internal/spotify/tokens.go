package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// TokenSource keeps the access token in memory and refreshes it from the
// keyring-backed refresh token. Tokens never leave the Go process.
type TokenSource struct {
	mu        sync.Mutex
	refreshMu sync.Mutex
	clientID  string
	tokenURL  string
	client    *http.Client
	now       func() time.Time
	persist   func(string) error

	accessToken  string
	refreshToken string
	expiry       time.Time
}

func NewTokenSource(clientID, refreshToken, tokenURL string, client *http.Client, persist func(string) error) *TokenSource {
	if tokenURL == "" {
		tokenURL = defaultTokenURL
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &TokenSource{
		clientID:     strings.TrimSpace(clientID),
		tokenURL:     tokenURL,
		client:       client,
		now:          time.Now,
		persist:      persist,
		refreshToken: refreshToken,
	}
}

func (s *TokenSource) SetNow(now func() time.Time) {
	if now != nil {
		s.now = now
	}
}

// Seed stores the access token returned by a fresh authorization exchange.
func (s *TokenSource) Seed(accessToken, refreshToken string, expiry time.Time) {
	s.mu.Lock()
	s.accessToken = accessToken
	s.refreshToken = refreshToken
	s.expiry = expiry
	s.mu.Unlock()
}

// Invalidate forces the next AccessToken call to refresh.
func (s *TokenSource) Invalidate() {
	s.mu.Lock()
	s.accessToken = ""
	s.expiry = time.Time{}
	s.mu.Unlock()
}

func (s *TokenSource) AccessToken(ctx context.Context) (string, error) {
	if token, ok := s.cachedToken(); ok {
		return token, nil
	}

	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	if token, ok := s.cachedToken(); ok {
		return token, nil
	}

	s.mu.Lock()
	refreshToken := s.refreshToken
	s.mu.Unlock()
	if refreshToken == "" {
		return "", ErrNotAuthenticated
	}
	tokens, err := s.refresh(ctx, refreshToken)
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	s.accessToken = tokens.AccessToken
	if tokens.RefreshToken != "" {
		s.refreshToken = tokens.RefreshToken
	}
	s.expiry = tokens.Expiry
	rotated := tokens.RefreshToken
	s.mu.Unlock()

	if rotated != "" && rotated != refreshToken && s.persist != nil {
		// A rotated token is stored best-effort. The in-memory session remains
		// usable even if the OS keyring is temporarily locked.
		_ = s.persist(rotated)
	}
	return tokens.AccessToken, nil
}

func (s *TokenSource) cachedToken() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.accessToken == "" || s.expiry.IsZero() {
		return "", false
	}
	if s.now().Add(60 * time.Second).Before(s.expiry) {
		return s.accessToken, true
	}
	return "", false
}

func (s *TokenSource) refresh(ctx context.Context, refreshToken string) (Tokens, error) {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {s.clientID},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return Tokens{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return Tokens{}, fmt.Errorf("refresh the Spotify access token: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if err != nil {
		return Tokens{}, fmt.Errorf("read the Spotify refresh response: %w", err)
	}
	var payload tokenResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return Tokens{}, fmt.Errorf("invalid Spotify refresh response: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		if payload.Error == "invalid_grant" || payload.Error == "invalid_client" {
			return Tokens{}, ErrAuthRevoked
		}
		return Tokens{}, fmt.Errorf("Spotify token endpoint returned status %d", response.StatusCode)
	}
	if payload.AccessToken == "" {
		return Tokens{}, errors.New("Spotify refresh response did not include an access token")
	}
	expiresIn := payload.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	return Tokens{
		AccessToken:  payload.AccessToken,
		RefreshToken: payload.RefreshToken,
		Expiry:       s.now().Add(time.Duration(expiresIn) * time.Second),
	}, nil
}
