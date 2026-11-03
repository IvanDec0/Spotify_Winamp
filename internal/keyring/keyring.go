// Package keyring stores the Spotify refresh token in the operating system's
// secret service. Access tokens are deliberately never persisted here.
package keyring

import (
	"errors"
	"fmt"

	gokeyring "github.com/zalando/go-keyring"
)

const (
	serviceName = "cadence"
	userName    = "spotify-refresh-token"
)

// ErrNotFound indicates that no refresh token has been stored yet.
var ErrNotFound = errors.New("Spotify refresh token not found")

// Store is the refresh-token storage contract. Implementations must never log
// the token value.
type Store interface {
	GetRefreshToken() (string, error)
	SetRefreshToken(token string) error
	DeleteRefreshToken() error
	Available() error
}

// OSStore uses the freedesktop Secret Service through go-keyring.
type OSStore struct{}

func NewOSStore() *OSStore {
	return &OSStore{}
}

func (s *OSStore) GetRefreshToken() (string, error) {
	token, err := gokeyring.Get(serviceName, userName)
	if errors.Is(err, gokeyring.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read Spotify refresh token from the OS keyring: %w", err)
	}
	return token, nil
}

func (s *OSStore) SetRefreshToken(token string) error {
	if token == "" {
		return errors.New("refusing to store an empty refresh token")
	}
	if err := gokeyring.Set(serviceName, userName, token); err != nil {
		return fmt.Errorf("store Spotify refresh token in the OS keyring: %w", err)
	}
	return nil
}

func (s *OSStore) DeleteRefreshToken() error {
	err := gokeyring.Delete(serviceName, userName)
	if errors.Is(err, gokeyring.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("remove Spotify refresh token from the OS keyring: %w", err)
	}
	return nil
}

func (s *OSStore) Available() error {
	_, err := s.GetRefreshToken()
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("OS keyring is unavailable: %w", err)
	}
	return nil
}

// MemoryStore is a deterministic Store for tests and headless environments.
// It is not used by the application by default.
type MemoryStore struct {
	token        string
	availableErr error
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

// NewMemoryStoreWithError creates a memory store that reports an unavailable
// keyring on every operation.
func NewMemoryStoreWithError(err error) *MemoryStore {
	return &MemoryStore{availableErr: err}
}

func (s *MemoryStore) GetRefreshToken() (string, error) {
	if s.token == "" {
		return "", ErrNotFound
	}
	return s.token, nil
}

func (s *MemoryStore) SetRefreshToken(token string) error {
	if s.availableErr != nil {
		return s.availableErr
	}
	if token == "" {
		return errors.New("refusing to store an empty refresh token")
	}
	s.token = token
	return nil
}

func (s *MemoryStore) DeleteRefreshToken() error {
	s.token = ""
	return nil
}

func (s *MemoryStore) Available() error {
	return s.availableErr
}
