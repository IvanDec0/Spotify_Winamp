package artcache

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestCacheFetchesAndReusesImage(t *testing.T) {
	var requests atomic.Int32
	image := []byte("\x89PNG\r\n\x1a\nfake-image")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		writer.Header().Set("Content-Type", "image/png")
		_, _ = writer.Write(image)
	}))
	defer server.Close()

	cache := New(server.Client())
	key := cache.Register(server.URL + "/cover.png")
	if len(key) != 64 {
		t.Fatalf("key length = %d", len(key))
	}
	data, contentType, err := cache.Get(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(image) || contentType != "image/png" {
		t.Fatalf("image = %q, %q", data, contentType)
	}
	if _, _, err := cache.Get(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 1 {
		t.Fatalf("upstream requests = %d, want 1", requests.Load())
	}
}

func TestCacheRejectsUnsupportedContentAndOversizedImages(t *testing.T) {
	t.Run("content type", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Set("Content-Type", "text/html")
			_, _ = writer.Write([]byte("<html>"))
		}))
		defer server.Close()
		cache := New(server.Client())
		key := cache.Register(server.URL + "/cover")
		if _, _, err := cache.Get(context.Background(), key); err == nil {
			t.Fatal("Get() error = nil")
		}
	})

	t.Run("size", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Set("Content-Type", "image/png")
			_, _ = writer.Write([]byte(strings.Repeat("x", 64)))
		}))
		defer server.Close()
		cache := New(server.Client())
		cache.maxImage = 16
		key := cache.Register(server.URL + "/cover")
		if _, _, err := cache.Get(context.Background(), key); err == nil {
			t.Fatal("Get() error = nil")
		}
	})
}

func TestCacheRegistersOnlyHTTPURLs(t *testing.T) {
	cache := New(nil)
	for _, raw := range []string{"", "file:///etc/passwd", "javascript:alert(1)", "/local.png"} {
		if key := cache.Register(raw); key != "" {
			t.Fatalf("Register(%q) = %q", raw, key)
		}
	}
}

func TestCacheServesOpaqueKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "image/webp")
		_, _ = writer.Write([]byte("RIFFxxxxWEBP"))
	}))
	defer server.Close()
	cache := New(server.Client())
	key := cache.Register(server.URL + "/cover.webp")

	request := httptest.NewRequest(http.MethodGet, "/art/"+key, nil)
	response := httptest.NewRecorder()
	cache.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Header().Get("Content-Type") != "image/webp" {
		t.Fatalf("content type = %q", response.Header().Get("Content-Type"))
	}
	if strings.Contains(response.Body.String(), server.URL) {
		t.Fatal("response leaked the remote URL")
	}

	request = httptest.NewRequest(http.MethodGet, "/art/not-a-key", nil)
	response = httptest.NewRecorder()
	cache.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("unknown key status = %d", response.Code)
	}
}
