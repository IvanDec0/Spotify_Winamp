package audioapi

import (
	"context"
	"errors"
	"log"
	"time"

	"cadence/internal/webplayer"
)

const (
	webLoginTimeout     = 10 * time.Minute
	webLogoutTimeout    = 30 * time.Second
	webInstallTimeout   = 15 * time.Minute
	webStartTimeout     = 3 * time.Minute
	webStopTimeout      = 30 * time.Second
	webProvisionTimeout = 30 * time.Minute
	webUnavailableText  = "the web player is not configured"
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
	ConfiguredClientID() string
	SetClientID(string) error
	SetKeepAvailable(bool)
	KeepAvailable() bool
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

// provisionWeb prepares the web backend in the background at startup: it
// installs the Chromium runtime when missing and signs in when no session is
// stored. The helper itself is started lazily by the first Play/PlayContext/
// Transfer, unless the user opted into keeping the device available. Failures
// stay in the web player status; this path never blocks the application start.
func (s *Service) provisionWeb(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, webProvisionTimeout)
	defer cancel()
	log.Printf("web player automatic setup: preparing the runtime, session and helper")
	if err := s.provisionWebPlayer(ctx); err != nil {
		if !errors.Is(err, context.Canceled) {
			log.Printf("web player automatic setup: %v", err)
		}
		return
	}
	log.Printf("web player automatic setup: finished")
}

// provisionWebPlayer runs the provisioning steps in order. Installing first
// means the automatic login, which uses the same Chromium runtime, does not
// download it a second time.
func (s *Service) provisionWebPlayer(ctx context.Context) error {
	if s.web == nil {
		return errors.New(webUnavailableText)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// Zero-touch first run: when the user has no personal Spotify app yet,
	// create one through the existing developer dashboard automation. The
	// helper may show a login window once; after that the app is created and
	// its Client ID is applied in memory, so the web login uses it immediately.
	if s.developer != nil && !s.disableAutoProvision && s.web.ConfiguredClientID() == "" {
		log.Printf("web player automatic setup: creating a Spotify app")
		clientID, err := s.developer.EnsureWebApp(ctx)
		if err != nil {
			log.Printf("web player automatic setup: cannot create a Spotify app: %v", err)
		} else if err := s.web.SetClientID(clientID); err != nil {
			log.Printf("web player automatic setup: cannot apply the Spotify app: %v", err)
		}
	}
	status := s.web.Status()
	if !status.RuntimeInstalled {
		if _, err := s.web.InstallRuntime(ctx); err != nil {
			return err
		}
		status = s.web.Status()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !status.LoggedIn {
		if err := s.web.Login(ctx); err != nil {
			if errors.Is(err, webplayer.ErrLoginInProgress) {
				// A manual login already owns the flow; starting now would fail
				// with login required, so let that flow finish on its own.
				return nil
			}
			return err
		}
		status = s.web.Status()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if status.Running {
		return nil
	}
	// Do not start the helper here. Chromium is the largest resident cost and
	// the SDK device is only needed when the user actually plays something.
	// EnsureStarted is called by Play, PlayContext and Transfer("").
	if s.web.KeepAvailable() {
		return s.web.EnsureStarted(ctx)
	}
	return nil
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
