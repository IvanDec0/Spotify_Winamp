package playerapi

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"cadence/internal/diagnostics"
	"cadence/internal/playback"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// streamingProvider embeds the demo provider and adds the Streamer contract.
type streamingProvider struct {
	*playback.DemoProvider

	streamCalls atomic.Int32
	queueCalls  atomic.Int32
	started     chan struct{}
}

func newStreamingProvider() *streamingProvider {
	return &streamingProvider{
		DemoProvider: playback.NewDemoProvider(),
		started:      make(chan struct{}, 1),
	}
}

func (p *streamingProvider) Stream(ctx context.Context, _ func(playback.PlayerState)) error {
	p.streamCalls.Add(1)
	select {
	case p.started <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return ctx.Err()
}

func (p *streamingProvider) Queue(ctx context.Context) ([]playback.QueueItem, error) {
	p.queueCalls.Add(1)
	return p.DemoProvider.Queue(ctx)
}

func waitForStream(t *testing.T, provider *streamingProvider) {
	t.Helper()
	select {
	case <-provider.started:
	case <-time.After(3 * time.Second):
		t.Fatal("the provider stream did not start")
	}
}

func TestEmitWithoutAppDoesNotRecordEvent(t *testing.T) {
	service := NewService(playback.NewDemoProvider())
	recorder := diagnostics.NewRecorder()
	service.SetRecorder(recorder)

	service.emit(PlayerState{Provider: "demo", Status: playback.StatusPaused, Queue: []QueueItem{}})

	if events := recorder.Snapshot().Events; len(events) != 0 {
		t.Fatalf("events = %+v, want none without an application", events)
	}
}

func TestEmitWithoutRecorderIsNoop(t *testing.T) {
	service := NewService(playback.NewDemoProvider())
	service.SetRecorder(nil)
	service.emit(PlayerState{Provider: "demo"})
}

func TestShouldEmitDropsOnlyExactDuplicates(t *testing.T) {
	service := NewService(playback.NewDemoProvider())
	base := PlayerState{Provider: "demo", Revision: 4, ProgressMS: 10, CapturedAt: 100}
	if !service.shouldEmit(base) {
		t.Fatal("the first revision was dropped")
	}
	if service.shouldEmit(base) {
		t.Fatal("an exact duplicate was emitted")
	}

	advanced := base
	advanced.ProgressMS = 20
	if !service.shouldEmit(advanced) {
		t.Fatal("a demo tick with a repeated revision was dropped")
	}

	newer := PlayerState{Provider: "demo", Revision: 5, ProgressMS: 20, CapturedAt: 200}
	if !service.shouldEmit(newer) {
		t.Fatal("a newer revision was dropped")
	}
	if !service.shouldEmit(PlayerState{Provider: "demo", Revision: 0}) {
		t.Fatal("a revision-less demo tick was dropped")
	}
}

func TestGetQueueUsesProviderQueue(t *testing.T) {
	provider := newStreamingProvider()
	service := NewService(provider)

	queue, err := service.GetQueue()
	if err != nil {
		t.Fatal(err)
	}
	if len(queue) != 6 {
		t.Fatalf("queue length = %d, want 6", len(queue))
	}
	if calls := provider.queueCalls.Load(); calls != 1 {
		t.Fatalf("Queue calls = %d, want 1", calls)
	}
}

func TestServiceStartupStartsAndShutdownStopsTheStream(t *testing.T) {
	provider := newStreamingProvider()
	service := NewService(provider)

	if err := service.ServiceStartup(context.Background(), application.ServiceOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForStream(t, provider)

	if err := service.ServiceShutdown(); err != nil {
		t.Fatal(err)
	}
	calls := provider.streamCalls.Load()
	time.Sleep(50 * time.Millisecond)
	if got := provider.streamCalls.Load(); got != calls {
		t.Fatalf("stream restarted after shutdown: %d -> %d calls", calls, got)
	}
}

func TestSetProviderRestartsTheStream(t *testing.T) {
	first := newStreamingProvider()
	second := newStreamingProvider()
	service := NewService(first)
	if err := service.ServiceStartup(context.Background(), application.ServiceOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForStream(t, first)

	service.SetProvider(second)
	waitForStream(t, second)
	if got := first.streamCalls.Load(); got != 1 {
		t.Fatalf("old provider stream calls = %d, want 1", got)
	}

	if err := service.ServiceShutdown(); err != nil {
		t.Fatal(err)
	}
}
