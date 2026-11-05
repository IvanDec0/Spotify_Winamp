package visualizerapi

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"cadence/internal/visualizer"
)

type fakeCapture struct {
	mu      sync.Mutex
	starts  int
	fail    error
	started chan struct{}
	stopped chan struct{}
}

func newFakeCapture() *fakeCapture {
	return &fakeCapture{
		started: make(chan struct{}, 16),
		stopped: make(chan struct{}, 16),
	}
}

func (f *fakeCapture) capture(ctx context.Context, _ func(visualizer.Frame)) error {
	f.mu.Lock()
	f.starts++
	f.mu.Unlock()

	f.started <- struct{}{}
	defer func() { f.stopped <- struct{}{} }()

	if f.fail != nil {
		return f.fail
	}
	<-ctx.Done()
	return nil
}

func (f *fakeCapture) startCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.starts
}

func newTestService() (*Service, *fakeCapture) {
	fake := newFakeCapture()
	service := NewService()
	service.capture = fake.capture
	return service, fake
}

func waitSignal(t *testing.T, signal <-chan struct{}, label string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", label)
	}
}

func expectNoSignal(t *testing.T, signal <-chan struct{}, label string) {
	t.Helper()
	select {
	case <-signal:
		t.Fatalf("unexpected %s", label)
	case <-time.After(50 * time.Millisecond):
	}
}

func waitIdle(t *testing.T, service *Service) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		service.mu.Lock()
		running := service.cancel != nil
		service.mu.Unlock()
		if !running {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("capture did not become idle")
}

func TestSetActiveRequiresClientID(t *testing.T) {
	service, _ := newTestService()

	if err := service.SetActive("", true); err == nil {
		t.Fatal("expected an error for an empty client id")
	}
	if err := service.SetActive("   ", true); err == nil {
		t.Fatal("expected an error for a blank client id")
	}
}

func TestCaptureStartsOnFirstClientAndStopsOnLast(t *testing.T) {
	service, fake := newTestService()

	if err := service.SetActive("window-a", true); err != nil {
		t.Fatalf("activate window-a: %v", err)
	}
	waitSignal(t, fake.started, "first capture start")

	if err := service.SetActive("window-a", true); err != nil {
		t.Fatalf("duplicate activation: %v", err)
	}
	if err := service.SetActive("window-b", true); err != nil {
		t.Fatalf("activate window-b: %v", err)
	}
	expectNoSignal(t, fake.started, "second capture start")
	if fake.startCount() != 1 {
		t.Fatalf("captures started = %d, want 1", fake.startCount())
	}

	if err := service.SetActive("window-a", false); err != nil {
		t.Fatalf("deactivate window-a: %v", err)
	}
	expectNoSignal(t, fake.stopped, "capture stop with one client left")

	if err := service.SetActive("window-b", false); err != nil {
		t.Fatalf("deactivate window-b: %v", err)
	}
	waitSignal(t, fake.stopped, "capture stop")
}

func TestCaptureFailureLeavesClientsActiveAndAllowsRetry(t *testing.T) {
	service, fake := newTestService()
	fake.fail = errors.New("parec: monitor unavailable")

	if err := service.SetActive("window-a", true); err != nil {
		t.Fatalf("activate window-a: %v", err)
	}
	waitSignal(t, fake.started, "first capture start")
	waitSignal(t, fake.stopped, "failed capture finish")
	waitIdle(t, service)

	if err := service.SetActive("window-b", true); err != nil {
		t.Fatalf("activate window-b: %v", err)
	}
	waitSignal(t, fake.started, "capture retry")
	if fake.startCount() != 2 {
		t.Fatalf("captures started = %d, want 2", fake.startCount())
	}
	waitSignal(t, fake.stopped, "retry capture finish")
	waitIdle(t, service)

	// The failed captures never removed the clients.
	if err := service.SetActive("window-a", false); err != nil {
		t.Fatalf("deactivate window-a: %v", err)
	}
	if err := service.SetActive("window-b", false); err != nil {
		t.Fatalf("deactivate window-b: %v", err)
	}
}

func TestDeactivatingUnknownClientIsIgnored(t *testing.T) {
	service, fake := newTestService()

	if err := service.SetActive("unknown", false); err != nil {
		t.Fatalf("deactivate unknown: %v", err)
	}
	expectNoSignal(t, fake.started, "capture start")
}

func TestShutdownStopsRunningCapture(t *testing.T) {
	service, fake := newTestService()

	if err := service.SetActive("window-a", true); err != nil {
		t.Fatalf("activate: %v", err)
	}
	waitSignal(t, fake.started, "capture start")

	if err := service.ServiceShutdown(); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	waitSignal(t, fake.stopped, "capture stop on shutdown")
}
