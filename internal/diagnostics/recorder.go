// Package diagnostics collects optional runtime counters for the local engine,
// the artwork cache and Wails events. It has no Wails dependency and no
// package-level mutable state. A nil *Recorder is a valid no-op recorder, so
// callers never need to branch on whether diagnostics are enabled.
package diagnostics

import (
	"runtime"
	"sync"
	"time"
)

// EngineRouteStats is the accumulated cost of one engine route, keyed by
// "METHOD /path".
type EngineRouteStats struct {
	Requests  int64   `json:"requests"`
	Errors    int64   `json:"errors"`
	BytesIn   int64   `json:"bytes_in"`
	BytesOut  int64   `json:"bytes_out"`
	LatencyMS float64 `json:"latency_ms_total"`
}

// ArtworkStats is the accumulated count and byte volume for one artwork kind
// (hit, miss, error or served).
type ArtworkStats struct {
	Count int64 `json:"count"`
	Bytes int64 `json:"bytes"`
}

// EventStats is the accumulated count and payload volume for one event name.
type EventStats struct {
	Count int64 `json:"count"`
	Bytes int64 `json:"bytes"`
}

// Snapshot is a point-in-time copy of the counters. It is safe to encode as
// JSON and stable across releases.
type Snapshot struct {
	UptimeSeconds int64                       `json:"uptime_seconds"`
	Goroutines    int                         `json:"goroutines"`
	Engine        map[string]EngineRouteStats `json:"engine"`
	EngineTotal   EngineRouteStats            `json:"engine_total"`
	Artwork       map[string]ArtworkStats     `json:"artwork"`
	Events        map[string]EventStats       `json:"events"`
}

// Recorder holds all diagnostics counters. Create it with NewRecorder.
type Recorder struct {
	mu sync.Mutex

	started     time.Time
	engine      map[string]EngineRouteStats
	engineTotal EngineRouteStats
	artwork     map[string]ArtworkStats
	events      map[string]EventStats
}

// NewRecorder returns an empty recorder whose uptime starts now.
func NewRecorder() *Recorder {
	return &Recorder{
		started: time.Now(),
		engine:  make(map[string]EngineRouteStats),
		artwork: make(map[string]ArtworkStats),
		events:  make(map[string]EventStats),
	}
}

// ObserveEngineRequest records one completed engine HTTP request. status is 0
// when the transport failed before receiving a response. bytesIn is the request
// body size and bytesOut is the response body size actually read.
func (r *Recorder) ObserveEngineRequest(method, path string, status int, bytesIn, bytesOut int64, d time.Duration) {
	if r == nil {
		return
	}
	if path == "" {
		path = "/"
	}
	route := method + " " + path

	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensureLocked()

	stats := r.engine[route]
	stats.Requests++
	if status == 0 || status >= 400 {
		stats.Errors++
	}
	stats.BytesIn += bytesIn
	stats.BytesOut += bytesOut
	stats.LatencyMS += float64(d) / float64(time.Millisecond)
	r.engine[route] = stats

	total := r.engineTotal
	total.Requests++
	if status == 0 || status >= 400 {
		total.Errors++
	}
	total.BytesIn += bytesIn
	total.BytesOut += bytesOut
	total.LatencyMS += float64(d) / float64(time.Millisecond)
	r.engineTotal = total
}

// ObserveArtwork records one artwork cache observation. kind is "hit", "miss",
// "error" or "served"; bytes is only meaningful for "served".
func (r *Recorder) ObserveArtwork(kind string, bytes int64) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensureLocked()
	stats := r.artwork[kind]
	stats.Count++
	stats.Bytes += bytes
	r.artwork[kind] = stats
}

// ObserveEvent records one emitted event and its payload size.
func (r *Recorder) ObserveEvent(name string, payloadBytes int) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensureLocked()
	stats := r.events[name]
	stats.Count++
	stats.Bytes += int64(payloadBytes)
	r.events[name] = stats
}

// Snapshot returns a copy of the current counters. A nil recorder returns an
// empty snapshot with non-nil maps.
func (r *Recorder) Snapshot() Snapshot {
	if r == nil {
		return Snapshot{
			Engine:  make(map[string]EngineRouteStats),
			Artwork: make(map[string]ArtworkStats),
			Events:  make(map[string]EventStats),
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensureLocked()

	snapshot := Snapshot{
		UptimeSeconds: int64(time.Since(r.started).Seconds()),
		Goroutines:    runtime.NumGoroutine(),
		Engine:        make(map[string]EngineRouteStats, len(r.engine)),
		EngineTotal:   r.engineTotal,
		Artwork:       make(map[string]ArtworkStats, len(r.artwork)),
		Events:        make(map[string]EventStats, len(r.events)),
	}
	for route, stats := range r.engine {
		snapshot.Engine[route] = stats
	}
	for kind, stats := range r.artwork {
		snapshot.Artwork[kind] = stats
	}
	for name, stats := range r.events {
		snapshot.Events[name] = stats
	}
	return snapshot
}

// ensureLocked initialises the maps for a zero-value Recorder.
func (r *Recorder) ensureLocked() {
	if r.started.IsZero() {
		r.started = time.Now()
	}
	if r.engine == nil {
		r.engine = make(map[string]EngineRouteStats)
	}
	if r.artwork == nil {
		r.artwork = make(map[string]ArtworkStats)
	}
	if r.events == nil {
		r.events = make(map[string]EventStats)
	}
}
