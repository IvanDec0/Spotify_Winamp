package artwork

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"cadence/internal/diagnostics"
)

func encodedTestPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	picture := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			picture.SetRGBA(x, y, color.RGBA{
				R: uint8((x * 255) / width),
				G: uint8((y * 255) / height),
				B: 180,
				A: 255,
			})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, picture); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buffer.Bytes()
}

func imageServer(t *testing.T, payload []byte, contentType string, requests *atomic.Int32) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if requests != nil {
			requests.Add(1)
		}
		writer.Header().Set("Content-Type", contentType)
		_, _ = writer.Write(payload)
	}))
	t.Cleanup(server.Close)
	return server
}

func waitForFlightWaiters(t *testing.T, cache *Cache, key string, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		cache.mu.Lock()
		inFlight := cache.flights[key]
		waiters := 0
		if inFlight != nil {
			waiters = inFlight.waiters
		}
		cache.mu.Unlock()
		if waiters >= want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d waiters on %s", want, key)
}

func TestVariantSizesAndOriginalFallback(t *testing.T) {
	original := encodedTestPNG(t, 120, 120)
	server := imageServer(t, original, "image/png", nil)
	cache := New(server.Client())
	key := cache.Register(server.URL + "/cover.png")

	data, contentType, err := cache.Get(context.Background(), key, 64)
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "image/jpeg" {
		t.Fatalf("variant content type = %q, want image/jpeg", contentType)
	}
	decoded, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode variant: %v", err)
	}
	if got := decoded.Bounds(); got != image.Rect(0, 0, 64, 64) {
		t.Fatalf("variant bounds = %v, want 64x64", got)
	}

	originalData, originalType, err := cache.Get(context.Background(), key, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(originalData, original) || originalType != "image/png" {
		t.Fatalf("size 0 = %d bytes %q, want the original PNG", len(originalData), originalType)
	}

	unknownData, unknownType, err := cache.Get(context.Background(), key, 100)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(unknownData, original) || unknownType != "image/png" {
		t.Fatalf("unknown size = %d bytes %q, want the original PNG", len(unknownData), unknownType)
	}
}

func TestVariantDoesNotUpscale(t *testing.T) {
	original := encodedTestPNG(t, 120, 120)
	server := imageServer(t, original, "image/png", nil)
	cache := New(server.Client())
	key := cache.Register(server.URL + "/cover.png")

	data, contentType, err := cache.Get(context.Background(), key, 640)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, original) || contentType != "image/png" {
		t.Fatalf("upscale request = %d bytes %q, want the original", len(data), contentType)
	}
}

func TestVariantSingleFlight(t *testing.T) {
	original := encodedTestPNG(t, 120, 120)
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseAll := func() {
		releaseOnce.Do(func() { close(release) })
	}
	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		writer.Header().Set("Content-Type", "image/png")
		_, _ = writer.Write(original)
	}))
	defer server.Close()
	defer releaseAll()

	cache := New(server.Client())
	key := cache.Register(server.URL + "/cover.png")

	results := make(chan error, 2)
	fetch := func() {
		data, contentType, err := cache.Get(context.Background(), key, 64)
		if err == nil && (contentType != "image/jpeg" || len(data) == 0) {
			err = errors.New("unexpected variant result")
		}
		results <- err
	}

	go fetch()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("upstream fetch did not start")
	}
	go fetch()
	waitForFlightWaiters(t, cache, variantKey(key, 64), 2)
	releaseAll()

	for range 2 {
		select {
		case err := <-results:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("Get did not return")
		}
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("upstream requests = %d, want 1", got)
	}
}

func TestDiskCachePersistsVariantsWithoutNetwork(t *testing.T) {
	original := encodedTestPNG(t, 120, 120)
	var requests atomic.Int32
	server := imageServer(t, original, "image/png", &requests)
	dir := t.TempDir()

	first := NewWithOptions(Options{Client: server.Client(), DiskDir: dir, DiskLimit: 1 << 20})
	key := first.Register(server.URL + "/cover.png")
	data, contentType, err := first.Get(context.Background(), key, 64)
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "image/jpeg" {
		t.Fatalf("variant content type = %q", contentType)
	}
	path := filepath.Join(dir, key+"@64.jpg")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("variant file: %v", err)
	}
	if info.Size() != int64(len(data)) {
		t.Fatalf("variant file size = %d, want %d", info.Size(), len(data))
	}

	before := requests.Load()
	second := NewWithOptions(Options{Client: server.Client(), DiskDir: dir, DiskLimit: 1 << 20})
	if registered := second.Register(server.URL + "/cover.png"); registered != key {
		t.Fatalf("registered key = %q, want %q", registered, key)
	}
	diskData, diskType, err := second.Get(context.Background(), key, 64)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(diskData, data) || diskType != "image/jpeg" {
		t.Fatalf("disk variant = %d bytes %q, want the cached JPEG", len(diskData), diskType)
	}
	if got := requests.Load(); got != before {
		t.Fatalf("disk hit made %d upstream requests", got-before)
	}

	recorder := diagnostics.NewRecorder()
	third := NewWithOptions(Options{Client: server.Client(), DiskDir: dir, DiskLimit: 1 << 20})
	third.SetRecorder(recorder)
	third.Register(server.URL + "/cover.png")
	if _, _, err := third.Get(context.Background(), key, 64); err != nil {
		t.Fatal(err)
	}
	if got := recorder.Snapshot().Artwork["disk_hit"].Count; got != 1 {
		t.Fatalf("disk_hit count = %d, want 1", got)
	}
	if got := requests.Load(); got != before {
		t.Fatalf("disk_hit fetch made %d upstream requests", got-before)
	}
}

func TestDiskCacheEvictsOldestVariant(t *testing.T) {
	original := encodedTestPNG(t, 120, 120)
	server := imageServer(t, original, "image/png", nil)
	variant, ok := resizeJPEG(original, 64)
	if !ok {
		t.Fatal("expected a resized variant")
	}
	limit := int64(len(variant)) + 1
	dir := t.TempDir()
	cache := NewWithOptions(Options{Client: server.Client(), DiskDir: dir, DiskLimit: limit})

	keyA := cache.Register(server.URL + "/a.png")
	keyB := cache.Register(server.URL + "/b.png")
	if _, _, err := cache.Get(context.Background(), keyA, 64); err != nil {
		t.Fatal(err)
	}
	if _, _, err := cache.Get(context.Background(), keyB, 64); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, keyA+"@64.jpg")); !os.IsNotExist(err) {
		t.Fatalf("oldest variant was not evicted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, keyB+"@64.jpg")); err != nil {
		t.Fatalf("newest variant missing: %v", err)
	}
}

func TestDiskCacheEvictsOnConstruction(t *testing.T) {
	original := encodedTestPNG(t, 120, 120)
	server := imageServer(t, original, "image/png", nil)
	dir := t.TempDir()

	large := NewWithOptions(Options{Client: server.Client(), DiskDir: dir, DiskLimit: 1 << 20})
	keyA := large.Register(server.URL + "/a.png")
	keyB := large.Register(server.URL + "/b.png")
	for _, key := range []string{keyA, keyB} {
		if _, _, err := large.Get(context.Background(), key, 64); err != nil {
			t.Fatal(err)
		}
	}
	pathA := filepath.Join(dir, keyA+"@64.jpg")
	pathB := filepath.Join(dir, keyB+"@64.jpg")
	infoA, err := os.Stat(pathA)
	if err != nil {
		t.Fatal(err)
	}
	infoB, err := os.Stat(pathB)
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(pathA, old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(pathB, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}

	NewWithOptions(Options{
		Client:    server.Client(),
		DiskDir:   dir,
		DiskLimit: infoA.Size() + infoB.Size() - 1,
	})

	if _, err := os.Stat(pathA); !os.IsNotExist(err) {
		t.Fatalf("scan did not evict the oldest variant: %v", err)
	}
	if _, err := os.Stat(pathB); err != nil {
		t.Fatalf("scan removed the newest variant: %v", err)
	}
}

func TestVariantFallbackKeepsOriginalBytes(t *testing.T) {
	corrupt := []byte("not a real image")
	server := imageServer(t, corrupt, "image/jpeg", nil)
	dir := t.TempDir()
	cache := NewWithOptions(Options{Client: server.Client(), DiskDir: dir, DiskLimit: 1 << 20})
	key := cache.Register(server.URL + "/cover.jpg")

	data, contentType, err := cache.Get(context.Background(), key, 64)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, corrupt) || contentType != "image/jpeg" {
		t.Fatalf("fallback = %q %q, want the original bytes and type", data, contentType)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("failed decode wrote %d files to disk", len(entries))
	}
}

func TestDiskCacheDisabledWhenDirectoryIsUnusable(t *testing.T) {
	original := encodedTestPNG(t, 120, 120)
	server := imageServer(t, original, "image/png", nil)
	dir := filepath.Join(t.TempDir(), "art")
	if err := os.WriteFile(dir, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	cache := NewWithOptions(Options{Client: server.Client(), DiskDir: dir, DiskLimit: 1 << 20})
	key := cache.Register(server.URL + "/cover.png")

	data, contentType, err := cache.Get(context.Background(), key, 64)
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "image/jpeg" || len(data) == 0 {
		t.Fatalf("variant without a disk dir = %d bytes %q", len(data), contentType)
	}
}

func TestServeHTTPVariantQuery(t *testing.T) {
	original := encodedTestPNG(t, 120, 120)
	server := imageServer(t, original, "image/png", nil)
	cache := New(server.Client())
	key := cache.Register(server.URL + "/cover.png")

	request := httptest.NewRequest(http.MethodGet, "/art/"+key+"?size=64", nil)
	response := httptest.NewRecorder()
	cache.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "image/jpeg" {
		t.Fatalf("content type = %q, want image/jpeg", contentType)
	}
	decoded, err := jpeg.Decode(bytes.NewReader(response.Body.Bytes()))
	if err != nil {
		t.Fatalf("decode served variant: %v", err)
	}
	if got := decoded.Bounds(); got != image.Rect(0, 0, 64, 64) {
		t.Fatalf("served bounds = %v, want 64x64", got)
	}
}

func TestNewUsesExtendedMemoryBounds(t *testing.T) {
	cache := New(nil)
	if cache.maxItems != 256 {
		t.Fatalf("max entries = %d, want 256", cache.maxItems)
	}
	if cache.maxBytes != 64<<20 {
		t.Fatalf("max bytes = %d, want 64 MiB", cache.maxBytes)
	}
	if cache.maxImage != 3<<20 {
		t.Fatalf("max image = %d, want 3 MiB", cache.maxImage)
	}
}
