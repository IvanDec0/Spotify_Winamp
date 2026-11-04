package playerapi

import (
	"testing"

	"cadence/internal/diagnostics"
	"cadence/internal/playback"
)

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
