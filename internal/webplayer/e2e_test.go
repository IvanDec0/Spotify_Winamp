//go:build linux && webplayer_e2e

package webplayer

import (
	"context"
	"encoding/binary"
	"os"
	"os/exec"
	"testing"
	"time"
)

// TestE2EPlayback exercises the real account, token store, runtime, helper,
// control socket, Web API and audio output. Run it after:
//
//	cadence -web-login
//	cadence -web-install
//	CADENCE_WEBPLAYER_E2E=1 go test -tags webplayer_e2e -run TestE2EPlayback -v ./internal/webplayer
func TestE2EPlayback(t *testing.T) {
	if os.Getenv("CADENCE_WEBPLAYER_E2E") != "1" {
		t.Skip("set CADENCE_WEBPLAYER_E2E=1 to run the real-account test")
	}
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		t.Skip("no X11 or Wayland display")
	}

	service, err := NewService(ServiceOptions{ClientID: DefaultClientID})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	if !service.LoggedIn() {
		t.Fatal("no Spotify session; run `cadence -web-login` first")
	}
	if !service.RuntimeInstalled() {
		t.Fatal("no runtime; run `cadence -web-install` first")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if err := service.EnsureStarted(ctx); err != nil {
		t.Fatalf("ensure started: %v", err)
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer stopCancel()
		_ = service.Stop(stopCtx)
	}()
	t.Logf("SDK device %s", service.DeviceID())
	if service.DeviceID() == "" {
		t.Fatal("the helper did not report a device id")
	}

	const track = "spotify:track:4uLU6hMCjMI75M1A2tKUQC"
	var playErr error
	for attempt := 0; attempt < 10; attempt++ {
		playErr = service.Provider().PlayContext(ctx, track, "")
		if playErr == nil {
			break
		}
		t.Logf("play attempt %d: %v", attempt+1, playErr)
		time.Sleep(time.Second)
	}
	if playErr != nil {
		t.Fatalf("play context: %v", playErr)
	}

	deadline := time.Now().Add(30 * time.Second)
	for {
		player, err := service.API().Player(ctx)
		if err != nil {
			t.Fatalf("player: %v", err)
		}
		if player != nil && player.IsPlaying && player.Device != nil {
			if player.Device.ID != service.DeviceID() {
				t.Fatalf("active device = %s, want %s", player.Device.ID, service.DeviceID())
			}
			name := ""
			if player.Item != nil {
				name = player.Item.Name
			}
			t.Logf("playing %q on %s at %d ms", name, player.Device.Name, player.ProgressMS)
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("playback did not become active on the SDK device")
		}
		time.Sleep(500 * time.Millisecond)
	}

	time.Sleep(3 * time.Second)
	output, err := exec.Command(
		"timeout", "3", "parec",
		"--format=s16le", "--rate=44100", "--channels=1",
		"--latency-msec=20", "--device=@DEFAULT_MONITOR@", "--raw",
	).Output()
	if err != nil {
		// timeout(1) kills parec with 124 after the capture window; the
		// samples it produced are still valid.
		exitErr, ok := err.(*exec.ExitError)
		if !ok || exitErr.ExitCode() != 124 {
			t.Fatalf("capture audio: %v", err)
		}
	}
	if len(output) < 2 {
		t.Fatal("captured no audio samples")
	}
	peak := 0
	for index := 0; index+1 < len(output); index += 2 {
		value := int(int16(binary.LittleEndian.Uint16(output[index : index+2])))
		if value < 0 {
			value = -value
		}
		if value > peak {
			peak = value
		}
	}
	if peak < 200 {
		t.Fatalf("captured audio is silent (peak=%d)", peak)
	}
	t.Logf("audio peak=%d over %d bytes", peak, len(output))
}
