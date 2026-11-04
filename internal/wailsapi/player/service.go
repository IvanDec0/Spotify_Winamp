package playerapi

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"time"

	"cadence/internal/diagnostics"
	"cadence/internal/playback"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// StateEvent is emitted whenever the player state changes. Providers that
// implement playback.Streamer push changes from the engine event stream; other
// providers fall back to a polling interval.
const StateEvent = "player:state"

const (
	fallbackEmitInterval = 500 * time.Millisecond
	initialStreamBackoff = 500 * time.Millisecond
	maxStreamBackoff     = 10 * time.Second
	healthyStreamRun     = 10 * time.Second
)

// Service exposes the provider to the frontend. It never contains or forwards
// Spotify tokens: only PlayerState reaches the webview.
type Service struct {
	provider Provider

	mu           sync.Mutex
	app          *application.App
	recorder     *diagnostics.Recorder
	cancel       context.CancelFunc
	done         chan struct{}
	lastRevision uint64
	lastState    PlayerState
}

func NewService(provider Provider) *Service {
	return &Service{provider: provider}
}

// SetProvider switches the active provider and restarts the state transport.
//
//wails:ignore
func (s *Service) SetProvider(provider Provider) {
	if provider == nil {
		return
	}
	s.stopStream()
	s.mu.Lock()
	s.provider = provider
	s.lastRevision = 0
	s.lastState = PlayerState{}
	s.mu.Unlock()
	s.startStream()
}

// SetRecorder attaches an optional diagnostics recorder. A nil recorder
// disables event payload accounting.
//
//wails:ignore
func (s *Service) SetRecorder(recorder *diagnostics.Recorder) {
	s.mu.Lock()
	s.recorder = recorder
	s.mu.Unlock()
}

func (s *Service) providerSnapshot() Provider {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.provider
}

func (s *Service) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.mu.Lock()
	s.app = application.Get()
	s.lastRevision = 0
	s.lastState = PlayerState{}
	s.mu.Unlock()
	s.startStream()
	return nil
}

func (s *Service) ServiceShutdown() error {
	s.stopStream()
	return nil
}

func (s *Service) GetState() (PlayerState, error) {
	provider := s.providerSnapshot()
	return provider.State(context.Background())
}

func (s *Service) GetDevices() ([]Device, error) {
	state, err := s.providerSnapshot().State(context.Background())
	return state.Devices, err
}

func (s *Service) GetQueue() ([]QueueItem, error) {
	return s.providerSnapshot().Queue(context.Background())
}

func (s *Service) Play() (PlayerState, error) {
	return s.mutate(func(ctx context.Context, provider Provider) error {
		return provider.Play(ctx)
	})
}

func (s *Service) Pause() (PlayerState, error) {
	return s.mutate(func(ctx context.Context, provider Provider) error {
		return provider.Pause(ctx)
	})
}

func (s *Service) TogglePlay() (PlayerState, error) {
	return s.mutate(func(ctx context.Context, provider Provider) error {
		return provider.Toggle(ctx)
	})
}

func (s *Service) Next() (PlayerState, error) {
	return s.mutate(func(ctx context.Context, provider Provider) error {
		return provider.Next(ctx)
	})
}

func (s *Service) Previous() (PlayerState, error) {
	return s.mutate(func(ctx context.Context, provider Provider) error {
		return provider.Previous(ctx)
	})
}

func (s *Service) Seek(positionMS int) (PlayerState, error) {
	return s.mutate(func(ctx context.Context, provider Provider) error {
		return provider.Seek(ctx, positionMS)
	})
}

func (s *Service) SetVolume(volume int) (PlayerState, error) {
	return s.mutate(func(ctx context.Context, provider Provider) error {
		return provider.SetVolume(ctx, volume)
	})
}

func (s *Service) SetShuffle(enabled bool) (PlayerState, error) {
	return s.mutate(func(ctx context.Context, provider Provider) error {
		return provider.SetShuffle(ctx, enabled)
	})
}

func (s *Service) SetRepeat(mode string) (PlayerState, error) {
	return s.mutate(func(ctx context.Context, provider Provider) error {
		return provider.SetRepeat(ctx, RepeatMode(mode))
	})
}

func (s *Service) Transfer(deviceID string) (PlayerState, error) {
	return s.mutate(func(ctx context.Context, provider Provider) error {
		return provider.Transfer(ctx, deviceID)
	})
}

func (s *Service) mutate(action func(context.Context, Provider) error) (PlayerState, error) {
	ctx := context.Background()
	provider := s.providerSnapshot()
	actionErr := action(ctx, provider)
	state, stateErr := provider.State(ctx)
	if stateErr == nil {
		s.emit(state)
	}
	return state, errors.Join(actionErr, stateErr)
}

// startStream starts the state transport for the current provider. Streamed
// providers reconnect with exponential backoff; the rest keep the fallback
// polling ticker so the demo player keeps working.
func (s *Service) startStream() {
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return
	}
	provider := s.provider
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.cancel = cancel
	s.done = done
	s.mu.Unlock()

	go s.runStream(ctx, provider, done)
}

func (s *Service) runStream(ctx context.Context, provider Provider, done chan struct{}) {
	defer close(done)

	streamer, ok := provider.(playback.Streamer)
	if !ok {
		s.runFallback(ctx, provider)
		return
	}

	backoff := initialStreamBackoff
	for {
		started := time.Now()
		_ = streamer.Stream(ctx, s.emit)
		if ctx.Err() != nil {
			return
		}
		if time.Since(started) >= healthyStreamRun {
			backoff = initialStreamBackoff
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if backoff < maxStreamBackoff {
			backoff *= 2
			if backoff > maxStreamBackoff {
				backoff = maxStreamBackoff
			}
		}
	}
}

func (s *Service) runFallback(ctx context.Context, provider Provider) {
	if state, err := provider.State(ctx); err == nil {
		s.emit(state)
	}
	ticker := time.NewTicker(fallbackEmitInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			state, err := provider.State(ctx)
			if err == nil {
				s.emit(state)
			}
		}
	}
}

func (s *Service) stopStream() {
	s.mu.Lock()
	cancel := s.cancel
	done := s.done
	s.cancel = nil
	s.done = nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

// shouldEmit reports whether a state is new. A state is a duplicate only when
// it repeats the last revision and the payload is identical, which is what an
// action and the stream racing each other produce. Demo ticks keep a repeated
// revision while the progress advances, so they must still be emitted.
func (s *Service) shouldEmit(state PlayerState) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if state.Revision > 0 && state.Revision == s.lastRevision && reflect.DeepEqual(state, s.lastState) {
		return false
	}
	if state.Revision > 0 {
		s.lastRevision = state.Revision
	}
	s.lastState = state
	return true
}

func (s *Service) emit(state PlayerState) {
	if !s.shouldEmit(state) {
		return
	}
	s.mu.Lock()
	app := s.app
	recorder := s.recorder
	s.mu.Unlock()
	if app != nil {
		app.Event.Emit(StateEvent, state)
		if recorder != nil {
			if payload, err := json.Marshal(state); err == nil {
				recorder.ObserveEvent(StateEvent, len(payload))
			}
		}
	}
}
