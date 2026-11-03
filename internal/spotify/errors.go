package spotify

import (
	"errors"
	"fmt"
	"time"
)

// ErrAuthRevoked means Spotify rejected the refresh token. The user must log
// in again.
var ErrAuthRevoked = errors.New("Spotify authorization was revoked")

// ErrNotAuthenticated means no usable Spotify session exists.
var ErrNotAuthenticated = errors.New("Spotify is not connected")

// APIError is a sanitized Spotify Web API failure.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
	RetryAfter time.Duration
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("Spotify API returned status %d", e.StatusCode)
}

// IsRateLimit reports whether the API asked Cadence to slow down.
func (e *APIError) IsRateLimit() bool {
	return e.StatusCode == 429
}
