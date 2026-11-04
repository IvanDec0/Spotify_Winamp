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
	handler := diagnosticsHandler(recorder)

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
	var snapshot diagnostics.Snapshot
	if err := json.Unmarshal(response.Body.Bytes(), &snapshot); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if snapshot.Engine["GET /status"].Requests != 1 {
		t.Fatalf("engine requests = %+v", snapshot.Engine)
	}
}

func TestDiagnosticsHandlerRejectsUnknownPaths(t *testing.T) {
	handler := diagnosticsHandler(diagnostics.NewRecorder())

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
