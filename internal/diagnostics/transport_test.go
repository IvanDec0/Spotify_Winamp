package diagnostics

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWrapTransportRecordsRequestAndResponse(t *testing.T) {
	requestBody := []byte(`{"uri":"spotify:track:1"}`)
	responseBody := []byte(`{"ok":true}`)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		received, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		if !bytes.Equal(received, requestBody) {
			t.Errorf("request body = %q, want %q", received, requestBody)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(responseBody)
	}))
	defer server.Close()

	recorder := NewRecorder()
	client := &http.Client{Transport: WrapTransport(http.DefaultTransport, recorder)}
	response, err := client.Post(server.URL+"/player/play", "application/json", bytes.NewReader(requestBody))
	if err != nil {
		t.Fatal(err)
	}
	received, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(received, responseBody) {
		t.Fatalf("response body = %q, want %q", received, responseBody)
	}

	snapshot := recorder.Snapshot()
	route, ok := snapshot.Engine["POST /player/play"]
	if !ok {
		t.Fatalf("route missing from snapshot: %+v", snapshot.Engine)
	}
	if route.Requests != 1 || route.Errors != 0 {
		t.Fatalf("route requests/errors = %d/%d, want 1/0", route.Requests, route.Errors)
	}
	if route.BytesIn != int64(len(requestBody)) {
		t.Fatalf("bytes_in = %d, want %d", route.BytesIn, len(requestBody))
	}
	if route.BytesOut != int64(len(responseBody)) {
		t.Fatalf("bytes_out = %d, want %d", route.BytesOut, len(responseBody))
	}
	if snapshot.EngineTotal != route {
		t.Fatalf("engine_total = %+v, want %+v", snapshot.EngineTotal, route)
	}
}

func TestWrapTransportCountsStatusErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Error(writer, "engine failure", http.StatusInternalServerError)
	}))
	defer server.Close()

	recorder := NewRecorder()
	client := &http.Client{Transport: WrapTransport(http.DefaultTransport, recorder)}
	response, err := client.Get(server.URL + "/status")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()

	route := recorder.Snapshot().Engine["GET /status"]
	if route.Requests != 1 || route.Errors != 1 {
		t.Fatalf("route requests/errors = %d/%d, want 1/1", route.Requests, route.Errors)
	}
}

func TestWrapTransportCountsTransportErrors(t *testing.T) {
	recorder := NewRecorder()
	base := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
	})
	client := &http.Client{Transport: WrapTransport(base, recorder)}

	if _, err := client.Get("http://127.0.0.1:1/status"); err == nil {
		t.Fatal("expected transport error")
	}
	route := recorder.Snapshot().Engine["GET /status"]
	if route.Requests != 1 || route.Errors != 1 {
		t.Fatalf("route requests/errors = %d/%d, want 1/1", route.Requests, route.Errors)
	}
	if route.BytesOut != 0 {
		t.Fatalf("bytes_out = %d, want 0", route.BytesOut)
	}
}

func TestWrapTransportCountsUnknownLengthBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("first-"))
		writer.(http.Flusher).Flush()
		_, _ = writer.Write([]byte("second"))
	}))
	defer server.Close()

	recorder := NewRecorder()
	client := &http.Client{Transport: WrapTransport(http.DefaultTransport, recorder)}
	response, err := client.Get(server.URL + "/status")
	if err != nil {
		t.Fatal(err)
	}
	if response.ContentLength != -1 {
		t.Fatalf("ContentLength = %d, want -1", response.ContentLength)
	}
	received, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if string(received) != "first-second" {
		t.Fatalf("body = %q", received)
	}

	route := recorder.Snapshot().Engine["GET /status"]
	if route.BytesOut != int64(len("first-second")) {
		t.Fatalf("bytes_out = %d, want %d", route.BytesOut, len("first-second"))
	}
}

func TestWrapTransportCountsBytesActuallyRead(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("abcdefghij"))
	}))
	defer server.Close()

	recorder := NewRecorder()
	client := &http.Client{Transport: WrapTransport(http.DefaultTransport, recorder)}
	response, err := client.Get(server.URL + "/status")
	if err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 4)
	if _, err := io.ReadFull(response.Body, buffer); err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()

	route := recorder.Snapshot().Engine["GET /status"]
	if route.BytesOut != 4 {
		t.Fatalf("bytes_out = %d, want 4", route.BytesOut)
	}
}

func TestWrapTransportNilRecorderReturnsBase(t *testing.T) {
	if got := WrapTransport(http.DefaultTransport, nil); got != http.DefaultTransport {
		t.Fatal("nil recorder must return the base transport unchanged")
	}
	if got := WrapTransport(nil, nil); got != http.DefaultTransport {
		t.Fatal("nil base must fall back to http.DefaultTransport")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
