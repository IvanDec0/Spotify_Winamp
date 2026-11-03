package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestTokenSourceCachesRefreshesAndPersistsRotation(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		index := calls.Add(1)
		if err := request.ParseForm(); err != nil {
			t.Errorf("ParseForm: %v", err)
		}
		if request.Form.Get("grant_type") != "refresh_token" {
			t.Errorf("grant_type = %q", request.Form.Get("grant_type"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"access_token":  "access-" + strconv.Itoa(int(index)),
			"refresh_token": "rotated-" + strconv.Itoa(int(index)),
			"expires_in":    3600,
		})
	}))
	defer server.Close()

	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	var persisted string
	source := NewTokenSource("client", "refresh-original", server.URL, server.Client(), func(token string) error {
		persisted = token
		return nil
	})
	source.SetNow(func() time.Time { return now })

	token, err := source.AccessToken(context.Background())
	if err != nil || token != "access-1" {
		t.Fatalf("AccessToken() = %q, %v", token, err)
	}
	if token, err := source.AccessToken(context.Background()); err != nil || token != "access-1" {
		t.Fatalf("cached AccessToken() = %q, %v", token, err)
	}
	if calls.Load() != 1 {
		t.Fatalf("refresh calls = %d", calls.Load())
	}
	if persisted != "rotated-1" {
		t.Fatalf("persisted token = %q", persisted)
	}

	now = now.Add(2 * time.Hour)
	token, err = source.AccessToken(context.Background())
	if err != nil || token != "access-2" {
		t.Fatalf("expired AccessToken() = %q, %v", token, err)
	}
	if calls.Load() != 2 {
		t.Fatalf("refresh calls = %d", calls.Load())
	}
}

func TestTokenSourceRevokedAndMissing(t *testing.T) {
	t.Run("missing refresh token", func(t *testing.T) {
		source := NewTokenSource("client", "", "https://example.test", nil, nil)
		if _, err := source.AccessToken(context.Background()); !errors.Is(err, ErrNotAuthenticated) {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("invalid grant", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(writer).Encode(map[string]string{"error": "invalid_grant"})
		}))
		defer server.Close()
		source := NewTokenSource("client", "refresh", server.URL, server.Client(), nil)
		if _, err := source.AccessToken(context.Background()); !errors.Is(err, ErrAuthRevoked) {
			t.Fatalf("error = %v", err)
		}
	})
}
