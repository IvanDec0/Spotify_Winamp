package player

import (
	"testing"

	"cadence/internal/diagnostics"
)

func TestEmitWithoutAppDoesNotRecordEvent(t *testing.T) {
	service := NewService(NewDemoProvider())
	recorder := diagnostics.NewRecorder()
	service.SetRecorder(recorder)

	service.emit(PlayerState{Provider: "demo", Status: StatusPaused, Queue: []QueueItem{}})

	if events := recorder.Snapshot().Events; len(events) != 0 {
		t.Fatalf("events = %+v, want none without an application", events)
	}
}

func TestEmitWithoutRecorderIsNoop(t *testing.T) {
	service := NewService(NewDemoProvider())
	service.SetRecorder(nil)
	service.emit(PlayerState{Provider: "demo"})
}
