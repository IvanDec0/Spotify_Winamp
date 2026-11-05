package diagnostics

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func TestRecorderConcurrentObserves(t *testing.T) {
	recorder := NewRecorder()
	const workers = 8
	const iterations = 100

	var wait sync.WaitGroup
	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for range iterations {
				recorder.ObserveEngineRequest("GET", "/status", 200, 3, 5, 2*time.Millisecond)
				recorder.ObserveArtwork("hit", 0)
				recorder.ObserveEvent("player:state", 20)
			}
		}()
	}
	wait.Wait()

	snapshot := recorder.Snapshot()
	if got, want := snapshot.EngineTotal.Requests, int64(workers*iterations); got != want {
		t.Fatalf("engine_total.requests = %d, want %d", got, want)
	}
	if got, want := snapshot.EngineTotal.BytesIn, int64(workers*iterations*3); got != want {
		t.Fatalf("engine_total.bytes_in = %d, want %d", got, want)
	}
	if got, want := snapshot.EngineTotal.BytesOut, int64(workers*iterations*5); got != want {
		t.Fatalf("engine_total.bytes_out = %d, want %d", got, want)
	}
	if got, want := snapshot.Artwork["hit"].Count, int64(workers*iterations); got != want {
		t.Fatalf("artwork hit count = %d, want %d", got, want)
	}
	if got, want := snapshot.Events["player:state"].Count, int64(workers*iterations); got != want {
		t.Fatalf("event count = %d, want %d", got, want)
	}
	if got, want := snapshot.Events["player:state"].Bytes, int64(workers*iterations*20); got != want {
		t.Fatalf("event bytes = %d, want %d", got, want)
	}
}

func TestSnapshotShape(t *testing.T) {
	recorder := NewRecorder()
	recorder.ObserveEngineRequest("GET", "/status", 200, 10, 20, 3*time.Millisecond)
	recorder.ObserveEngineRequest("GET", "/status", 500, 0, 30, time.Millisecond)
	recorder.ObserveArtwork("served", 1024)
	recorder.ObserveEvent("player:state", 128)

	snapshot := recorder.Snapshot()
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	for _, key := range []string{"uptime_seconds", "goroutines", "engine", "engine_total", "artwork", "events"} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("snapshot JSON missing %q: %s", key, encoded)
		}
	}

	route := snapshot.Engine["GET /status"]
	if route.Requests != 2 || route.Errors != 1 {
		t.Fatalf("route = %+v, want 2 requests and 1 error", route)
	}
	if route.BytesIn != 10 || route.BytesOut != 50 {
		t.Fatalf("route bytes = %d/%d, want 10/50", route.BytesIn, route.BytesOut)
	}
	if route.LatencyMS < 4 {
		t.Fatalf("route latency = %f ms, want at least 4", route.LatencyMS)
	}
	if snapshot.EngineTotal != route {
		t.Fatalf("engine_total = %+v, want %+v", snapshot.EngineTotal, route)
	}
	if snapshot.Artwork["served"] != (ArtworkStats{Count: 1, Bytes: 1024}) {
		t.Fatalf("artwork = %+v", snapshot.Artwork["served"])
	}
	if snapshot.Events["player:state"] != (EventStats{Count: 1, Bytes: 128}) {
		t.Fatalf("events = %+v", snapshot.Events["player:state"])
	}
	if snapshot.Goroutines <= 0 {
		t.Fatalf("goroutines = %d, want positive", snapshot.Goroutines)
	}
}

func TestSnapshotIsACopy(t *testing.T) {
	recorder := NewRecorder()
	recorder.ObserveEngineRequest("GET", "/status", 200, 1, 2, time.Millisecond)
	recorder.ObserveArtwork("hit", 0)
	recorder.ObserveEvent("player:state", 1)

	snapshot := recorder.Snapshot()
	snapshot.Engine["GET /status"] = EngineRouteStats{Requests: 99}
	delete(snapshot.Artwork, "hit")
	snapshot.Events["player:state"] = EventStats{Count: 99}

	fresh := recorder.Snapshot()
	if fresh.Engine["GET /status"].Requests != 1 {
		t.Fatal("snapshot engine map aliases recorder state")
	}
	if fresh.Artwork["hit"].Count != 1 {
		t.Fatal("snapshot artwork map aliases recorder state")
	}
	if fresh.Events["player:state"].Count != 1 {
		t.Fatal("snapshot events map aliases recorder state")
	}
}

func TestNilRecorderIsNoop(t *testing.T) {
	var recorder *Recorder
	recorder.ObserveEngineRequest("GET", "/status", 200, 1, 2, time.Millisecond)
	recorder.ObserveArtwork("hit", 3)
	recorder.ObserveEvent("player:state", 4)

	snapshot := recorder.Snapshot()
	if snapshot.UptimeSeconds != 0 || snapshot.EngineTotal.Requests != 0 {
		t.Fatalf("nil snapshot = %+v, want zero values", snapshot)
	}
	if snapshot.Engine == nil || snapshot.Artwork == nil || snapshot.Events == nil {
		t.Fatalf("nil snapshot maps must be non-nil: %+v", snapshot)
	}
	if len(snapshot.Engine) != 0 || len(snapshot.Artwork) != 0 || len(snapshot.Events) != 0 {
		t.Fatalf("nil snapshot maps must be empty: %+v", snapshot)
	}
}

func TestZeroValueRecorderWorks(t *testing.T) {
	recorder := &Recorder{}
	recorder.ObserveEvent("player:state", 1)
	snapshot := recorder.Snapshot()
	if snapshot.Events["player:state"].Count != 1 {
		t.Fatalf("events = %+v", snapshot.Events)
	}
}
