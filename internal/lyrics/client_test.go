package lyrics

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func testTrack() Track {
	return Track{
		Title:      "Neon Tide",
		Artists:    []string{"Aurora"},
		Album:      "Liquid Glass",
		DurationMS: 212_000,
		URI:        "spotify:track:preview",
	}
}

func TestFetchSyncedLyricsAndCache(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.URL.Path != "/get" {
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
		if got := request.URL.Query().Get("duration"); got != "212" {
			t.Fatalf("duration = %q; want 212", got)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"id": 1,
			"trackName": "Neon Tide",
			"artistName": "Aurora",
			"albumName": "Liquid Glass",
			"duration": 212,
			"instrumental": false,
			"plainLyrics": "plain",
			"syncedLyrics": "[00:01.00]Hello\n[00:02.50]World"
		}`))
	}))
	defer server.Close()

	cacheDir := t.TempDir()
	client := NewClient(Options{
		BaseURL:  server.URL,
		CacheDir: cacheDir,
		Now:      func() time.Time { return time.Unix(100, 0) },
	})
	result, err := client.Fetch(context.Background(), testTrack())
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if !result.Available || !result.Synced || len(result.Lines) != 2 {
		t.Fatalf("Fetch() = %+v; want synced lyrics", result)
	}
	if result.Lines[0].TimeMS != 1000 || result.Lines[0].Text != "Hello" {
		t.Fatalf("first line = %+v", result.Lines[0])
	}
	if _, err := client.Fetch(context.Background(), testTrack()); err != nil {
		t.Fatalf("cached Fetch() error = %v", err)
	}
	if requests.Load() != 1 {
		t.Fatalf("provider requests = %d; want 1 (cache hit)", requests.Load())
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("cache entries = %d, err = %v; want 1", len(entries), err)
	}
}

func TestFetchFallsBackToSearch(t *testing.T) {
	var getRequests, searchRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/get":
			getRequests.Add(1)
			http.NotFound(writer, request)
		case "/search":
			searchRequests.Add(1)
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`[{
				"id": 2,
				"trackName": "Neon Tide",
				"artistName": "Aurora",
				"albumName": "Liquid Glass",
				"duration": 212,
				"instrumental": false,
				"plainLyrics": "One\nTwo",
				"syncedLyrics": ""
			}]`))
		default:
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(Options{BaseURL: server.URL})
	result, err := client.Fetch(context.Background(), testTrack())
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if result.Synced || len(result.Lines) != 2 || result.Lines[0].TimeMS != -1 {
		t.Fatalf("Fetch() = %+v; want plain lines", result)
	}
	if getRequests.Load() != 1 || searchRequests.Load() != 1 {
		t.Fatalf("requests get=%d search=%d", getRequests.Load(), searchRequests.Load())
	}
}

func TestFetchNotFoundCachesNegativeResult(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		http.NotFound(writer, request)
	}))
	defer server.Close()

	now := time.Unix(100, 0)
	client := NewClient(Options{
		BaseURL:  server.URL,
		CacheDir: t.TempDir(),
		Now:      func() time.Time { return now },
	})
	if _, err := client.Fetch(context.Background(), testTrack()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Fetch() error = %v; want ErrNotFound", err)
	}
	if _, err := client.Fetch(context.Background(), testTrack()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cached Fetch() error = %v; want ErrNotFound", err)
	}
	if requests.Load() != 2 {
		// /get and /search are both tried on the first call; the second call is cached.
		t.Fatalf("provider requests = %d; want 2", requests.Load())
	}
}
