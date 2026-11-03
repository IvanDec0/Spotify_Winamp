package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func seededClient(t *testing.T, baseURL string, client *http.Client) *Client {
	t.Helper()
	tokens := NewTokenSource("client", "refresh", "https://token.invalid", client, nil)
	tokens.Seed("access-token", "refresh-token", time.Now().Add(time.Hour))
	return NewClientWithBase(tokens, baseURL, client)
}

func TestClientPlaybackResponses(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantFound  bool
		wantCode   string
		retryAfter time.Duration
	}{
		{
			name:      "success",
			status:    http.StatusOK,
			body:      `{"is_playing":true,"progress_ms":1000,"device":{"id":"d","name":"Desk","type":"speaker","is_active":true,"volume_percent":50},"item":{"id":"t","name":"Track","duration_ms":2000,"artists":[{"name":"Artist"}],"album":{"name":"Album"}},"repeat_state":"off","shuffle_state":false}`,
			wantFound: true,
		},
		{name: "no content", status: http.StatusNoContent, wantFound: false},
		{name: "forbidden", status: http.StatusForbidden, body: `{"error":{"status":403,"message":"Premium required"}}`, wantCode: "forbidden"},
		{name: "not found", status: http.StatusNotFound, body: `{"error":{"status":404,"message":"No active device"}}`, wantCode: "not_found"},
		{name: "rate limited", status: http.StatusTooManyRequests, body: `{"error":{"status":429,"message":"slow down"}}`, wantCode: "rate_limited", retryAfter: 7 * time.Second},
		{name: "server", status: http.StatusBadGateway, body: `{"error":{"status":502,"message":"bad"}}`, wantCode: "server_error"},
		{name: "malformed", status: http.StatusOK, body: `{`, wantCode: "invalid_response"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Header.Get("Authorization") != "Bearer access-token" {
					t.Errorf("authorization = %q", request.Header.Get("Authorization"))
				}
				writer.Header().Set("Content-Type", "application/json")
				if test.retryAfter > 0 {
					writer.Header().Set("Retry-After", "7")
				}
				writer.WriteHeader(test.status)
				_, _ = writer.Write([]byte(test.body))
			}))
			defer server.Close()
			client := seededClient(t, server.URL, server.Client())
			playback, found, err := client.Playback(context.Background())
			if test.wantCode != "" {
				var apiErr *APIError
				if !errors.As(err, &apiErr) || apiErr.Code != test.wantCode {
					t.Fatalf("error = %v, want APIError %q", err, test.wantCode)
				}
				if test.retryAfter > 0 && apiErr.RetryAfter != test.retryAfter {
					t.Fatalf("retry after = %v", apiErr.RetryAfter)
				}
				return
			}
			if err != nil {
				t.Fatalf("Playback() error = %v", err)
			}
			if found != test.wantFound {
				t.Fatalf("found = %v", found)
			}
			if found && (playback.Item == nil || playback.Item.Name != "Track") {
				t.Fatalf("playback = %#v", playback)
			}
		})
	}
}

func TestClientRetriesAfter401(t *testing.T) {
	var apiCalls atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		call := apiCalls.Add(1)
		if call == 1 {
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{"error":{"status":401,"message":"expired"}}`))
			return
		}
		if request.Header.Get("Authorization") != "Bearer refreshed" {
			t.Errorf("authorization = %q", request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"is_playing":false}`))
	}))
	defer api.Close()

	tokenServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"access_token": "refreshed",
			"expires_in":   3600,
		})
	}))
	defer tokenServer.Close()

	tokens := NewTokenSource("client", "refresh", tokenServer.URL, tokenServer.Client(), nil)
	tokens.Seed("expired", "refresh", time.Now().Add(time.Hour))
	client := NewClientWithBase(tokens, api.URL, api.Client())
	if _, _, err := client.Playback(context.Background()); err != nil {
		t.Fatalf("Playback() error = %v", err)
	}
	if apiCalls.Load() != 2 {
		t.Fatalf("API calls = %d", apiCalls.Load())
	}
}

func TestClientControlsAndTransfer(t *testing.T) {
	type capturedRequest struct {
		method string
		path   string
		query  string
		body   string
	}
	requests := make(chan capturedRequest, 16)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body strings.Builder
		if request.Body != nil {
			_, _ = io.Copy(&body, request.Body)
		}
		requests <- capturedRequest{
			method: request.Method,
			path:   request.URL.Path,
			query:  request.URL.RawQuery,
			body:   body.String(),
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	client := seededClient(t, server.URL, server.Client())

	if err := client.Seek(context.Background(), 42); err != nil {
		t.Fatal(err)
	}
	if err := client.SetVolume(context.Background(), 55); err != nil {
		t.Fatal(err)
	}
	if err := client.SetShuffle(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if err := client.SetRepeat(context.Background(), "track"); err != nil {
		t.Fatal(err)
	}
	if err := client.Transfer(context.Background(), "device-1"); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		method string
		path   string
		query  string
	}{
		{http.MethodPut, "/me/player/seek", "position_ms=42"},
		{http.MethodPut, "/me/player/volume", "volume_percent=55"},
		{http.MethodPut, "/me/player/shuffle", "state=true"},
		{http.MethodPut, "/me/player/repeat", "state=track"},
		{http.MethodPut, "/me/player", ""},
	}
	for _, expected := range cases {
		request := <-requests
		if request.method != expected.method || request.path != expected.path || request.query != expected.query {
			t.Fatalf("request = %s %s?%s, want %s %s?%s", request.method, request.path, request.query, expected.method, expected.path, expected.query)
		}
		if expected.path == "/me/player" && request.body != "" {
			var body struct {
				DeviceIDs []string `json:"device_ids"`
			}
			if err := json.Unmarshal([]byte(request.body), &body); err != nil {
				t.Fatal(err)
			}
			if len(body.DeviceIDs) != 1 || body.DeviceIDs[0] != "device-1" {
				t.Fatalf("body = %#v", body)
			}
		}
	}
}

func TestClientRejectsMalformedTokenlessResponses(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"access_token": "refreshed",
			"expires_in":   3600,
		})
	}))
	defer tokenServer.Close()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"error":"invalid_token"}`))
	}))
	defer server.Close()
	tokens := NewTokenSource("client", "refresh", tokenServer.URL, tokenServer.Client(), nil)
	tokens.Seed("access-token", "refresh-token", time.Now().Add(time.Hour))
	client := NewClientWithBase(tokens, server.URL, server.Client())
	_, _, err := client.Playback(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(err.Error(), "access-token") {
		t.Fatal("error leaked the access token")
	}
}
