//go:build linux && webplayer_smoke

package webplayer

import (
	"context"
	"os"
	"testing"
	"time"

	"cadence/internal/webplayer/helper"
)

// smokeTokens returns a deliberately invalid token. The SDK must reach the
// authentication step and report authentication_error, which proves the page,
// the CSP, the SDK iframe and the preload bridge all work.
type smokeTokens struct{}

func (smokeTokens) AccessToken(context.Context) (string, time.Time, error) {
	return "cadence-smoke-invalid-token", time.Now().Add(time.Hour), nil
}

// TestSmokeRealRuntime exercises the production path: download and verify the
// pinned castlabs runtime, materialize the embedded helper, launch Electron and
// wait for a control event. Run it with:
//
//	go test -tags webplayer_smoke -run TestSmokeRealRuntime -v ./internal/webplayer
func TestSmokeRealRuntime(t *testing.T) {
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		t.Skip("no X11 or Wayland display")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()

	runtime := NewRuntime(RuntimeOptions{Root: t.TempDir()})
	install, err := runtime.Ensure(ctx)
	if err != nil {
		t.Fatalf("ensure runtime: %v", err)
	}
	t.Logf("runtime %s at %s", install.Version, install.Dir)

	control, err := NewControl()
	if err != nil {
		t.Fatalf("control: %v", err)
	}
	defer control.Close()

	server, err := NewServer(ServerOptions{
		Page:   helper.Page(),
		Script: helper.Script(),
		Tokens: smokeTokens{},
	})
	if err != nil {
		t.Fatalf("server: %v", err)
	}
	defer server.Close()
	t.Logf("player url %s", server.PlayerURL())

	process, err := runtime.Start(ctx, install, StartOptions{
		ControlURL: control.URL(),
		PlayerURL:  server.PlayerURL(),
	})
	if err != nil {
		t.Fatalf("start helper: %v", err)
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer stopCancel()
		_ = process.Stop(stopCtx)
		for _, line := range process.Tail() {
			t.Logf("helper: %s", line)
		}
	}()

	deadline := time.After(2 * time.Minute)
	for {
		select {
		case event, ok := <-control.Events():
			if !ok {
				t.Fatal("control events closed")
			}
			t.Logf("control event: %+v", event)
			switch {
			case event.Type == "ready":
				t.Fatalf("unexpected ready with an invalid token: %+v", event)
			case event.Type == "error" && event.Code == "authentication_error":
				verifyHelperStaysAlive(t, process)
				return
			case event.Type == "error" && (event.Code == "initialization_error" || event.Code == "account_error"):
				t.Fatalf("SDK failed before authentication: %+v", event)
			}
		case <-deadline:
			t.Fatal("timed out waiting for an SDK control event")
		}
	}
}

// verifyHelperStaysAlive catches a helper that dies after the SDK error (for
// example, a Chromium GPU/sandbox failure during startup). The window is
// deliberately longer than the point where the Phase 0 spike showed the GPU
// process settling.
func verifyHelperStaysAlive(t *testing.T, process *Process) {
	t.Helper()
	exited := make(chan error, 1)
	go func() { exited <- process.Wait() }()
	select {
	case err := <-exited:
		t.Fatalf("helper exited after the SDK error: %v", err)
	case <-time.After(20 * time.Second):
	}
}
