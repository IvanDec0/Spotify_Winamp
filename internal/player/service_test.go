package player

import (
	"testing"

	"cadence/internal/diagnostics"
)

func TestEmitRecordsEventPayload(t *testing.T) {
	service := NewService(NewDemoProvider())
	recorder := diagnostics.NewRecorder()
	service.SetRecorder(recorder)

	service.emit(PlayerState{Provider: "demo", Status: StatusPaused, Queue: []QueueItem{}})

	events := recorder.Snapshot().Events
	if events[StateEvent].Count != 1 {
		t.Fatalf("events = %+v, want one %s", events, StateEvent)
	}
	if events[StateEvent].Bytes == 0 {
		t.Fatalf("event bytes = 0, want the marshalled payload size")
	}
}

func TestEmitWithoutRecorderIsNoop(t *testing.T) {
	service := NewService(NewDemoProvider())
	service.SetRecorder(nil)
	service.emit(PlayerState{Provider: "demo"})
}
