package webplayer

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// maxTokenFileSize bounds the persisted token document. A real Spotify
	// token response is a few kilobytes; anything larger is corrupt or hostile.
	maxTokenFileSize = 64 << 10

	tokenFileMode = 0o600
	tokenDirMode  = 0o700
)

// Tokens is the persisted Spotify OAuth session. Token values are secrets and
// must never be logged, returned to the frontend or included in errors.
type Tokens struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	Scope        string    `json:"scope"`
	Expiry       time.Time `json:"expiry"`
}

// TokenStore persists one Spotify OAuth session on disk.
type TokenStore struct {
	path string
}

// NewTokenStore returns a store rooted at an explicit token file path.
func NewTokenStore(path string) *TokenStore {
	return &TokenStore{path: path}
}

// Path returns the token file path.
func (s *TokenStore) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// Load reads the stored session. A missing file returns ErrNoTokens.
func (s *TokenStore) Load() (Tokens, error) {
	if s == nil || s.path == "" {
		return Tokens{}, ErrNoTokens
	}
	info, err := os.Stat(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Tokens{}, ErrNoTokens
	}
	if err != nil {
		return Tokens{}, fmt.Errorf("read the Spotify session: %w", err)
	}
	if info.Size() > maxTokenFileSize {
		return Tokens{}, codedError(CodeInvalidArgument, "the Spotify session file is too large", nil)
	}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Tokens{}, ErrNoTokens
	}
	if err != nil {
		return Tokens{}, fmt.Errorf("read the Spotify session: %w", err)
	}
	// Best effort: an older file may have been created with broader
	// permissions before the store existed.
	_ = os.Chmod(s.path, tokenFileMode)

	var tokens Tokens
	if err := json.Unmarshal(data, &tokens); err != nil {
		return Tokens{}, codedError(CodeInvalidArgument, "the Spotify session file is not valid", err)
	}
	return tokens, nil
}

// Save writes the session atomically with owner-only permissions.
func (s *TokenStore) Save(tokens Tokens) error {
	if s == nil || s.path == "" {
		return codedError(CodeInvalidArgument, "the Spotify session path is empty", nil)
	}
	data, err := json.Marshal(tokens)
	if err != nil {
		return fmt.Errorf("encode the Spotify session: %w", err)
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, tokenDirMode); err != nil {
		return fmt.Errorf("create the Spotify session directory: %w", err)
	}
	if dir != "." {
		if err := os.Chmod(dir, tokenDirMode); err != nil {
			return fmt.Errorf("secure the Spotify session directory: %w", err)
		}
	}

	temp, err := os.CreateTemp(dir, ".tokens-*.tmp")
	if err != nil {
		return fmt.Errorf("create a temporary Spotify session file: %w", err)
	}
	tempName := temp.Name()
	defer func() {
		_ = os.Remove(tempName)
	}()

	if err := temp.Chmod(tokenFileMode); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempName, s.path); err != nil {
		return fmt.Errorf("replace the Spotify session file: %w", err)
	}
	return nil
}

// Clear removes the stored session. A missing file is not an error.
func (s *TokenStore) Clear() error {
	if s == nil || s.path == "" {
		return nil
	}
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove the Spotify session: %w", err)
	}
	return nil
}
