// Package visualizerapi adapts the monitor analyzer to the Wails frontend.
package visualizerapi

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"

	"cadence/internal/visualizer"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// FrameEvent is emitted for every analyzed monitor frame.
const FrameEvent = "visualizer:frame"

type captureFunc func(context.Context, func(visualizer.Frame)) error

// Service owns the capture lifecycle. The capture runs only while at least one
// window reports itself active, and stops as soon as the last one deactivates.
type Service struct {
	mu      sync.Mutex
	app     *application.App
	clients map[string]struct{}
	capture captureFunc
	cancel  context.CancelFunc
	done    chan struct{}
}

// NewService creates the Wails-facing visualizer service.
func NewService() *Service {
	return &Service{
		clients: make(map[string]struct{}),
		capture: visualizer.Capture,
	}
}

// SetActive registers or unregisters a window. Duplicate calls are ignored.
// Capture starts on the first active client and stops on the last.
func (s *Service) SetActive(clientID string, active bool) error {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return errors.New("visualizer: client id is required")
	}

	s.mu.Lock()
	if active {
		if _, exists := s.clients[clientID]; exists {
			s.mu.Unlock()
			return nil
		}
		s.clients[clientID] = struct{}{}
		s.startCaptureLocked()
		s.mu.Unlock()
		return nil
	}

	if _, exists := s.clients[clientID]; !exists {
		s.mu.Unlock()
		return nil
	}
	delete(s.clients, clientID)
	var cancel context.CancelFunc
	var done chan struct{}
	if len(s.clients) == 0 {
		cancel, done = s.takeCaptureLocked()
	}
	s.mu.Unlock()

	stopCapture(cancel, done)
	return nil
}

func (s *Service) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	s.mu.Lock()
	s.app = application.Get()
	s.mu.Unlock()
	return nil
}

func (s *Service) ServiceShutdown() error {
	s.mu.Lock()
	s.clients = make(map[string]struct{})
	s.app = nil
	cancel, done := s.takeCaptureLocked()
	s.mu.Unlock()

	stopCapture(cancel, done)
	return nil
}

// startCaptureLocked starts one capture goroutine. The caller must hold the
// mutex. When a capture failed earlier the fields are nil again, so the next
// client transition retries.
func (s *Service) startCaptureLocked() {
	if s.cancel != nil {
		return
	}
	capture := s.capture
	if capture == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.cancel = cancel
	s.done = done
	go s.run(ctx, done, capture)
}

func (s *Service) run(ctx context.Context, done chan struct{}, capture captureFunc) {
	defer close(done)
	if err := capture(ctx, s.emit); err != nil && ctx.Err() == nil {
		// Do not crash the application when parec or the monitor is missing.
		// Clients stay active so a later pause/play transition can retry.
		log.Printf("visualizer: capture stopped: %v", err)
	}
	s.mu.Lock()
	if s.done == done {
		s.cancel = nil
		s.done = nil
	}
	s.mu.Unlock()
}

// takeCaptureLocked detaches the running capture so it can be stopped without
// holding the mutex while the goroutine finishes.
func (s *Service) takeCaptureLocked() (context.CancelFunc, chan struct{}) {
	cancel := s.cancel
	done := s.done
	s.cancel = nil
	s.done = nil
	return cancel, done
}

func stopCapture(cancel context.CancelFunc, done chan struct{}) {
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (s *Service) emit(frame visualizer.Frame) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.app == nil {
		return
	}
	s.app.Event.Emit(FrameEvent, frame)
}
