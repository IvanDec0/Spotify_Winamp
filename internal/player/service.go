package player

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// StateEvent is emitted whenever the player state changes and on a short
// polling interval so the frontend stays close to the provider clock.
const StateEvent = "player:state"

const emitInterval = 500 * time.Millisecond

// Service exposes the provider to the frontend. It never contains or forwards
// Spotify tokens: only PlayerState reaches the webview.
type Service struct {
	provider Provider

	mu   sync.Mutex
	app  *application.App
	stop chan struct{}
}

func NewService(provider Provider) *Service {
	return &Service{provider: provider}
}

func (s *Service) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.mu.Lock()
	s.app = application.Get()
	s.mu.Unlock()
	s.startTicker()
	return nil
}

func (s *Service) ServiceShutdown() error {
	s.stopTicker()
	return nil
}

func (s *Service) GetState() (PlayerState, error) {
	return s.provider.State(context.Background())
}

func (s *Service) GetDevices() ([]Device, error) {
	state, err := s.provider.State(context.Background())
	return state.Devices, err
}

func (s *Service) GetQueue() ([]QueueItem, error) {
	state, err := s.provider.State(context.Background())
	return state.Queue, err
}

func (s *Service) Play() (PlayerState, error) {
	return s.mutate(func(ctx context.Context) error {
		return s.provider.Play(ctx)
	})
}

func (s *Service) Pause() (PlayerState, error) {
	return s.mutate(func(ctx context.Context) error {
		return s.provider.Pause(ctx)
	})
}

func (s *Service) TogglePlay() (PlayerState, error) {
	return s.mutate(func(ctx context.Context) error {
		return s.provider.Toggle(ctx)
	})
}

func (s *Service) Next() (PlayerState, error) {
	return s.mutate(func(ctx context.Context) error {
		return s.provider.Next(ctx)
	})
}

func (s *Service) Previous() (PlayerState, error) {
	return s.mutate(func(ctx context.Context) error {
		return s.provider.Previous(ctx)
	})
}

func (s *Service) Seek(positionMS int) (PlayerState, error) {
	return s.mutate(func(ctx context.Context) error {
		return s.provider.Seek(ctx, positionMS)
	})
}

func (s *Service) SetVolume(volume int) (PlayerState, error) {
	return s.mutate(func(ctx context.Context) error {
		return s.provider.SetVolume(ctx, volume)
	})
}

func (s *Service) SetShuffle(enabled bool) (PlayerState, error) {
	return s.mutate(func(ctx context.Context) error {
		return s.provider.SetShuffle(ctx, enabled)
	})
}

func (s *Service) SetRepeat(mode string) (PlayerState, error) {
	return s.mutate(func(ctx context.Context) error {
		return s.provider.SetRepeat(ctx, RepeatMode(mode))
	})
}

func (s *Service) Transfer(deviceID string) (PlayerState, error) {
	return s.mutate(func(ctx context.Context) error {
		return s.provider.Transfer(ctx, deviceID)
	})
}

func (s *Service) mutate(action func(context.Context) error) (PlayerState, error) {
	ctx := context.Background()
	actionErr := action(ctx)
	state, stateErr := s.provider.State(ctx)
	if stateErr == nil {
		s.emit(state)
	}
	return state, errors.Join(actionErr, stateErr)
}

func (s *Service) startTicker() {
	s.mu.Lock()
	if s.stop != nil {
		s.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	s.stop = stop
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(emitInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				state, err := s.provider.State(context.Background())
				if err == nil {
					s.emit(state)
				}
			case <-stop:
				return
			}
		}
	}()
}

func (s *Service) stopTicker() {
	s.mu.Lock()
	stop := s.stop
	s.stop = nil
	s.mu.Unlock()
	if stop != nil {
		close(stop)
	}
}

func (s *Service) emit(state PlayerState) {
	s.mu.Lock()
	app := s.app
	s.mu.Unlock()
	if app != nil {
		app.Event.Emit(StateEvent, state)
	}
}
