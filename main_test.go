package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cadence/internal/diagnostics"
)

func TestDiagnosticsHandlerServesSnapshot(t *testing.T) {
	recorder := diagnostics.NewRecorder()
	recorder.ObserveEngineRequest("GET", "/status", 200, 0, 42, time.Millisecond)
	handler := diagnosticsHandler(recorder, func() string { return "v9.9.9-test" })

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/debug/stats", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	if got := response.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	var payload struct {
		diagnostics.Snapshot
		EngineVersion string `json:"engine_version"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if payload.EngineVersion != "v9.9.9-test" {
		t.Fatalf("engine_version = %q, want v9.9.9-test", payload.EngineVersion)
	}
	if payload.Engine["GET /status"].Requests != 1 {
		t.Fatalf("engine requests = %+v", payload.Engine)
	}
}

func TestDiagnosticsHandlerRejectsUnknownPaths(t *testing.T) {
	handler := diagnosticsHandler(diagnostics.NewRecorder(), func() string { return "" })

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/debug/other", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/debug/stats", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", response.Code)
	}
}
