package playback

import (
	"context"
	"testing"
	"time"
)

func newTestProvider(t *testing.T) (*DemoProvider, *time.Time) {
	t.Helper()
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	provider := newDemoProvider(func() time.Time { return now })
	return provider, &now
}

func mustState(t *testing.T, provider *DemoProvider) PlayerState {
	t.Helper()
	state, err := provider.State(context.Background())
	if err != nil {
		t.Fatalf("State() error = %v", err)
	}
	return state
}

func TestDemoProviderStartsPlaying(t *testing.T) {
	provider, _ := newTestProvider(t)
	state := mustState(t, provider)

	if state.Status != StatusPlaying {
		t.Fatalf("status = %q, want %q", state.Status, StatusPlaying)
	}
	if state.ProgressMS != 38_000 {
		t.Fatalf("progress = %d, want 38000", state.ProgressMS)
	}
	if state.Provider != demoProviderName {
		t.Fatalf("provider = %q, want %q", state.Provider, demoProviderName)
	}
	if !state.CanSeek || !state.CanControl {
		t.Fatal("demo state should be seekable and controllable")
	}
}

func TestDemoProviderPlayPauseAndClock(t *testing.T) {
	provider, now := newTestProvider(t)

	*now = now.Add(5 * time.Second)
	if got := mustState(t, provider).ProgressMS; got != 43_000 {
		t.Fatalf("progress after 5s = %d, want 43000", got)
	}

	if err := provider.Pause(context.Background()); err != nil {
		t.Fatalf("Pause() error = %v", err)
	}
	*now = now.Add(5 * time.Second)
	if got := mustState(t, provider).ProgressMS; got != 43_000 {
		t.Fatalf("paused progress = %d, want 43000", got)
	}

	if err := provider.Play(context.Background()); err != nil {
		t.Fatalf("Play() error = %v", err)
	}
	*now = now.Add(2 * time.Second)
	if got := mustState(t, provider).ProgressMS; got != 45_000 {
		t.Fatalf("resumed progress = %d, want 45000", got)
	}
}

func TestDemoProviderTrackNavigation(t *testing.T) {
	provider, _ := newTestProvider(t)
	first := mustState(t, provider).Track.ID

	if err := provider.Next(context.Background()); err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	second := mustState(t, provider)
	if second.Track.ID == first {
		t.Fatalf("Next() did not change track: %q", second.Track.ID)
	}
	if second.ProgressMS != 0 {
		t.Fatalf("next progress = %d, want 0", second.ProgressMS)
	}

	if err := provider.Previous(context.Background()); err != nil {
		t.Fatalf("Previous() error = %v", err)
	}
	if got := mustState(t, provider).Track.ID; got != first {
		t.Fatalf("Previous() track = %q, want %q", got, first)
	}
}

func TestDemoProviderPreviousRestartsAfterThreeSeconds(t *testing.T) {
	provider, _ := newTestProvider(t)
	first := mustState(t, provider).Track.ID

	if err := provider.Seek(context.Background(), 10_000); err != nil {
		t.Fatalf("Seek() error = %v", err)
	}
	if err := provider.Previous(context.Background()); err != nil {
		t.Fatalf("Previous() error = %v", err)
	}

	state := mustState(t, provider)
	if state.Track.ID != first {
		t.Fatalf("track = %q, want %q", state.Track.ID, first)
	}
	if state.ProgressMS != 0 {
		t.Fatalf("progress = %d, want 0", state.ProgressMS)
	}
}

func TestDemoProviderSeekAndVolumeClamp(t *testing.T) {
	provider, _ := newTestProvider(t)
	if err := provider.Pause(context.Background()); err != nil {
		t.Fatalf("Pause() error = %v", err)
	}

	if err := provider.Seek(context.Background(), -500); err != nil {
		t.Fatalf("Seek() error = %v", err)
	}
	if got := mustState(t, provider).ProgressMS; got != 0 {
		t.Fatalf("negative seek progress = %d, want 0", got)
	}

	if err := provider.Seek(context.Background(), 999_999); err != nil {
		t.Fatalf("Seek() error = %v", err)
	}
	state := mustState(t, provider)
	if state.ProgressMS != state.DurationMS {
		t.Fatalf("over-range seek progress = %d, want %d", state.ProgressMS, state.DurationMS)
	}

	if err := provider.SetVolume(context.Background(), 150); err != nil {
		t.Fatalf("SetVolume() error = %v", err)
	}
	if got := mustState(t, provider).Volume; got != 100 {
		t.Fatalf("volume clamp = %d, want 100", got)
	}
}

func TestDemoProviderShuffleRepeatAndTransfer(t *testing.T) {
	provider, _ := newTestProvider(t)

	if err := provider.SetShuffle(context.Background(), true); err != nil {
		t.Fatalf("SetShuffle() error = %v", err)
	}
	if err := provider.SetRepeat(context.Background(), RepeatTrack); err != nil {
		t.Fatalf("SetRepeat() error = %v", err)
	}
	if err := provider.Transfer(context.Background(), "living-room"); err != nil {
		t.Fatalf("Transfer() error = %v", err)
	}

	state := mustState(t, provider)
	if !state.Shuffle {
		t.Fatal("shuffle = false, want true")
	}
	if state.Repeat != RepeatTrack {
		t.Fatalf("repeat = %q, want %q", state.Repeat, RepeatTrack)
	}
	if state.Device.ID != "living-room" {
		t.Fatalf("active device = %q, want living-room", state.Device.ID)
	}
	if state.Volume != 38 {
		t.Fatalf("transferred volume = %d, want 38", state.Volume)
	}
}

func TestDemoProviderQueueIsOrderedAndCopied(t *testing.T) {
	provider, _ := newTestProvider(t)
	state := mustState(t, provider)

	if len(state.Queue) != len(provider.tracks) {
		t.Fatalf("queue length = %d, want %d", len(state.Queue), len(provider.tracks))
	}
	if !state.Queue[0].IsCurrent {
		t.Fatal("first queue item should be current")
	}
	if !state.Queue[1].IsNext {
		t.Fatal("second queue item should be next")
	}
	if state.Queue[0].Track.ID != state.Track.ID {
		t.Fatalf("queue current = %q, track = %q", state.Queue[0].Track.ID, state.Track.ID)
	}

	state.Queue[0].Track.Artists[0] = "Mutated"
	state.Devices[0].Name = "Mutated"
	next := mustState(t, provider)
	if next.Queue[0].Track.Artists[0] == "Mutated" {
		t.Fatal("queue artists were not copied")
	}
	if next.Devices[0].Name == "Mutated" {
		t.Fatal("devices were not copied")
	}
}

func TestDemoProviderAdvancesAtTrackEnd(t *testing.T) {
	provider, now := newTestProvider(t)
	first := mustState(t, provider).Track.ID

	*now = now.Add(252 * time.Second)
	state := mustState(t, provider)
	if state.Track.ID == first {
		t.Fatal("provider did not advance at track end")
	}
	if state.Status != StatusPlaying {
		t.Fatalf("status = %q, want playing", state.Status)
	}
}

func TestDemoProviderStopsAtEndWithoutRepeat(t *testing.T) {
	provider, now := newTestProvider(t)
	provider.current = len(provider.tracks) - 1
	provider.basePositionMS = provider.tracks[provider.current].DurationMS - 500
	provider.lastTick = *now

	*now = now.Add(time.Second)
	state := mustState(t, provider)
	if state.Status != StatusPaused {
		t.Fatalf("status = %q, want paused", state.Status)
	}
	if state.ProgressMS != state.DurationMS {
		t.Fatalf("progress = %d, want %d", state.ProgressMS, state.DurationMS)
	}
}

func TestDemoProviderRepeatContextWrapsQueue(t *testing.T) {
	provider, now := newTestProvider(t)
	provider.current = len(provider.tracks) - 1
	provider.basePositionMS = provider.tracks[provider.current].DurationMS - 500
	provider.lastTick = *now
	provider.repeat = RepeatContext

	*now = now.Add(time.Second)
	state := mustState(t, provider)
	if state.Track.ID != provider.tracks[0].ID {
		t.Fatalf("track = %q, want first track after wrap", state.Track.ID)
	}
}

func TestDemoProviderRevisionChangesOnControls(t *testing.T) {
	provider, _ := newTestProvider(t)
	before := mustState(t, provider).Revision

	if err := provider.SetVolume(context.Background(), 50); err != nil {
		t.Fatalf("SetVolume() error = %v", err)
	}
	after := mustState(t, provider).Revision
	if after <= before {
		t.Fatalf("revision = %d, want > %d", after, before)
	}
}
