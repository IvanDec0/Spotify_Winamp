package audioapi

import (
	"context"
	"errors"
	"time"
)

const developerTimeout = 10 * time.Minute

// DeveloperBackend is the subset of *developer.Service the audio adapter uses.
// *developer.Service satisfies it structurally.
type DeveloperBackend interface {
	EnsureWebApp(context.Context) (string, error)
	EnsureSoloistKey(context.Context, bool) (string, error)
}

// ConfigureWebPlayerApp creates or finds the Cadence Spotify Developer app,
// stores its Client ID and invalidates a session bound to the previous app.
// The action is headless.
func (s *Service) ConfigureWebPlayerApp() (WebPlayerStatus, error) {
	if s.developer == nil {
		return s.webPlayerStatus(), errors.New("the developer dashboard automation is not configured")
	}
	if s.web == nil {
		return s.webPlayerStatus(), errors.New(webUnavailableText)
	}
	ctx, cancel := context.WithTimeout(context.Background(), developerTimeout)
	defer cancel()
	clientID, err := s.developer.EnsureWebApp(ctx)
	if err != nil {
		return s.webPlayerStatus(), err
	}

	restartRequired := false
	if clientID != s.web.ClientID() {
		restartRequired = true
		if err := s.web.Logout(); err != nil {
			status := s.webPlayerStatus()
			status.ClientID = clientID
			status.RestartRequired = true
			return status, err
		}
	}
	status := s.webPlayerStatus()
	status.ClientID = clientID
	status.RestartRequired = restartRequired || status.RestartRequired
	return status, nil
}

// GenerateSoloistAPIKey creates or rotates a Soloist API key through the
// Spotify Developer dashboard and stores it locally. The key is never returned
// to the frontend or logged.
func (s *Service) GenerateSoloistAPIKey(rotate bool) (AudioState, error) {
	if s.developer == nil {
		return s.GetState(), errors.New("the developer dashboard automation is not configured")
	}
	if s.soloist == nil {
		return s.GetState(), errors.New("Spotify Soloist is not enabled")
	}
	ctx, cancel := context.WithTimeout(context.Background(), developerTimeout)
	defer cancel()
	key, err := s.developer.EnsureSoloistKey(ctx, rotate)
	if err != nil {
		return s.GetState(), err
	}
	err = s.soloist.SaveAPIKey(key)
	s.emitMerged()
	return s.GetState(), err
}
