package audioapi

import (
	"context"
	"errors"
	"time"

	"cadence/internal/webplayer"
)

const (
	webLoginTimeout    = 10 * time.Minute
	webLogoutTimeout   = 30 * time.Second
	webInstallTimeout  = 15 * time.Minute
	webStartTimeout    = 3 * time.Minute
	webStopTimeout     = 30 * time.Second
	webUnavailableText = "the web player is not configured"
)

// WebBackend is the subset of *webplayer.Service the audio adapter uses.
// *webplayer.Service satisfies it structurally.
type WebBackend interface {
	Login(context.Context) error
	Logout() error
	InstallRuntime(context.Context) (webplayer.Install, error)
	EnsureStarted(context.Context) error
	Stop(context.Context) error
	ClientID() string
	Status() webplayer.Status
}

// GetWebPlayerStatus returns the web player state without starting or
// downloading anything.
func (s *Service) GetWebPlayerStatus() WebPlayerStatus {
	return s.webPlayerStatus()
}

// ConnectWebPlayer starts the Spotify OAuth flow through the system browser.
func (s *Service) ConnectWebPlayer() (WebPlayerStatus, error) {
	return s.runWebAction(webLoginTimeout, func(ctx context.Context) error {
		return s.web.Login(ctx)
	})
}

// DisconnectWebPlayer removes the stored Spotify session and stops the helper.
func (s *Service) DisconnectWebPlayer() (WebPlayerStatus, error) {
	return s.runWebAction(webLogoutTimeout, func(context.Context) error {
		return s.web.Logout()
	})
}

// InstallWebPlayerRuntime downloads and verifies the helper runtime.
func (s *Service) InstallWebPlayerRuntime() (WebPlayerStatus, error) {
	if s.web == nil {
		return s.webPlayerStatus(), errors.New(webUnavailableText)
	}
	ctx, cancel := context.WithTimeout(context.Background(), webInstallTimeout)
	defer cancel()
	_, err := s.web.InstallRuntime(ctx)
	return s.webPlayerStatus(), err
}

// StartWebPlayer starts the helper when a session and runtime are available.
func (s *Service) StartWebPlayer() (WebPlayerStatus, error) {
	return s.runWebAction(webStartTimeout, func(ctx context.Context) error {
		return s.web.EnsureStarted(ctx)
	})
}

// StopWebPlayer stops the helper without clearing the session.
func (s *Service) StopWebPlayer() (WebPlayerStatus, error) {
	return s.runWebAction(webStopTimeout, func(ctx context.Context) error {
		return s.web.Stop(ctx)
	})
}

// webPlayerStatus maps the domain status to the generated frontend model. The
// action methods return it even when the action fails so the settings UI can
// surface the stored error code.
func (s *Service) webPlayerStatus() WebPlayerStatus {
	if s.web == nil {
		return WebPlayerStatus{Available: false}
	}
	status := s.web.Status()
	restartRequired := status.ConfiguredClientID != "" && status.ConfiguredClientID != status.ClientID
	return WebPlayerStatus{
		Available:        true,
		LoggedIn:         status.LoggedIn,
		Running:          status.Running,
		RuntimeInstalled: status.RuntimeInstalled,
		RuntimeVersion:   status.RuntimeVersion,
		DeviceID:         status.DeviceID,
		Account:          status.Account,
		LastError:        status.LastError,
		ClientID:         status.ClientID,
		RestartRequired:  restartRequired,
	}
}

func (s *Service) runWebAction(
	timeout time.Duration,
	run func(context.Context) error,
) (WebPlayerStatus, error) {
	if s.web == nil {
		return s.webPlayerStatus(), errors.New(webUnavailableText)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err := run(ctx)
	return s.webPlayerStatus(), err
}
