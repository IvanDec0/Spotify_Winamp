// Package artwork proxies album art through Go. The frontend never receives a
// remote image URL, only an opaque local key. Originals and resized variants
// live in a bounded in-memory LRU; resized variants may also be persisted in a
// bounded on-disk LRU. Untouched originals and failed decodes are never written
// to disk.
package artwork

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"cadence/internal/diagnostics"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	defaultMaxEntries = 256
	defaultMaxBytes   = 64 << 20
	defaultMaxImage   = 3 << 20
	defaultDiskLimit  = 256 << 20

	variantContentType = "image/jpeg"
)

// Options configures a cache. An empty DiskDir keeps the cache memory-only.
type Options struct {
	Client *http.Client
	// DiskDir enables a bounded on-disk LRU of resized variants.
	DiskDir string
	// DiskLimit caps the disk cache in bytes. Zero uses 256 MiB.
	DiskLimit int64
}

type entry struct {
	data        []byte
	contentType string
}

type diskFile struct {
	size int64
}

// flight is one in-progress cache fill. Waiters share its result instead of
// starting a second fetch or decode.
type flight struct {
	done        chan struct{}
	data        []byte
	contentType string
	err         error
	waiters     int
}

// Cache stores a bounded set of remote image URLs and their fetched or derived
// bytes.
type Cache struct {
	mu          sync.Mutex
	client      *http.Client
	sources     map[string]string
	sourceOrder []string
	entries     map[string]*entry
	order       []string
	bytes       int64
	maxItems    int
	maxBytes    int64
	maxImage    int64
	recorder    atomic.Pointer[diagnostics.Recorder]

	diskDir   string
	diskLimit int64
	diskBytes int64
	diskFiles map[string]diskFile
	diskOrder []string

	flights map[string]*flight
}

// New returns a memory-only cache. Use NewWithOptions to enable the disk LRU.
func New(client *http.Client) *Cache {
	return NewWithOptions(Options{Client: client})
}

func NewWithOptions(options Options) *Cache {
	client := options.Client
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second}
	}
	cache := &Cache{
		client:    client,
		sources:   make(map[string]string),
		entries:   make(map[string]*entry),
		maxItems:  defaultMaxEntries,
		maxBytes:  defaultMaxBytes,
		maxImage:  defaultMaxImage,
		diskFiles: make(map[string]diskFile),
		flights:   make(map[string]*flight),
	}
	cache.configureDisk(options.DiskDir, options.DiskLimit)
	return cache
}

func (c *Cache) configureDisk(dir string, limit int64) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return
	}
	if limit <= 0 {
		limit = defaultDiskLimit
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	c.diskDir = dir
	c.diskLimit = limit
	c.scanDisk()
}

// scanDisk indexes existing variants and evicts the oldest files when the
// directory already exceeds the configured limit.
func (c *Cache) scanDisk() {
	dirEntries, err := os.ReadDir(c.diskDir)
	if err != nil {
		c.diskDir = ""
		return
	}
	type candidate struct {
		key      string
		size     int64
		modified time.Time
	}
	candidates := make([]candidate, 0, len(dirEntries))
	for _, dirEntry := range dirEntries {
		if dirEntry.IsDir() {
			continue
		}
		name := dirEntry.Name()
		if !strings.HasSuffix(name, ".jpg") {
			continue
		}
		key := strings.TrimSuffix(name, ".jpg")
		if !validVariantKey(key) {
			continue
		}
		info, err := dirEntry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		candidates = append(candidates, candidate{key: key, size: info.Size(), modified: info.ModTime()})
	}
	sort.Slice(candidates, func(left, right int) bool {
		if candidates[left].modified.Equal(candidates[right].modified) {
			return candidates[left].key < candidates[right].key
		}
		return candidates[left].modified.Before(candidates[right].modified)
	})
	for _, file := range candidates {
		c.diskFiles[file.key] = diskFile{size: file.size}
		c.diskOrder = append(c.diskOrder, file.key)
		c.diskBytes += file.size
	}
	evicted := c.evictDiskLocked()
	for _, key := range evicted {
		_ = os.Remove(filepath.Join(c.diskDir, key+".jpg"))
	}
}

// SetRecorder attaches an optional diagnostics recorder. A nil recorder
// disables instrumentation without changing cache behavior.
func (c *Cache) SetRecorder(recorder *diagnostics.Recorder) {
	c.recorder.Store(recorder)
}

// observe is the hot-path accounting entry point: one atomic load, and an
// immediate return when diagnostics are disabled.
func (c *Cache) observe(kind string, bytes int64) {
	recorder := c.recorder.Load()
	if recorder == nil {
		return
	}
	recorder.ObserveArtwork(kind, bytes)
}

// Register maps a remote image URL to an opaque key used by the frontend.
// Only http and https URLs are accepted by the provider, but Register itself
// never exposes the URL to a caller.
func (c *Cache) Register(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(rawURL))
	key := hex.EncodeToString(sum[:])
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sources[key] = rawURL
	c.touchSourceLocked(key)
	for len(c.sourceOrder) > c.maxItems {
		delete(c.sources, c.sourceOrder[0])
		c.sourceOrder = c.sourceOrder[1:]
	}
	return key
}

// Get returns image bytes for a registered key. A supported positive size
// returns a JPEG variant scaled to at most that size; zero and unknown sizes
// return the untouched original. Variants are served from memory, then disk,
// and concurrent misses for the same key and size share one fetch and decode.
func (c *Cache) Get(ctx context.Context, key string, size int) ([]byte, string, error) {
	if !validKey(key) {
		c.observe("missing", 0)
		return nil, "", errors.New("unknown album art key")
	}
	size = normalizeSize(size)
	cacheKey := key
	if size > 0 {
		cacheKey = variantKey(key, size)
	}

	if data, contentType, ok := c.memoryGet(cacheKey); ok {
		c.observe("hit", 0)
		return data, contentType, nil
	}

	if size > 0 {
		if data, ok := c.diskGet(cacheKey); ok {
			c.storeMemory(cacheKey, data, variantContentType)
			c.observe("disk_hit", 0)
			return data, variantContentType, nil
		}
	}

	c.mu.Lock()
	if data, contentType, ok := c.entryLocked(cacheKey); ok {
		c.mu.Unlock()
		c.observe("hit", 0)
		return data, contentType, nil
	}
	if inFlight, ok := c.flights[cacheKey]; ok {
		inFlight.waiters++
		c.mu.Unlock()
		<-inFlight.done
		return inFlight.data, inFlight.contentType, inFlight.err
	}
	if _, ok := c.sources[key]; !ok {
		c.mu.Unlock()
		c.observe("missing", 0)
		return nil, "", errors.New("unknown album art key")
	}
	inFlight := &flight{done: make(chan struct{}), waiters: 1}
	c.flights[cacheKey] = inFlight
	c.mu.Unlock()

	data, contentType, err := c.load(ctx, key, cacheKey, size)

	c.mu.Lock()
	delete(c.flights, cacheKey)
	inFlight.data = data
	inFlight.contentType = contentType
	inFlight.err = err
	close(inFlight.done)
	c.mu.Unlock()
	return data, contentType, err
}

func (c *Cache) load(ctx context.Context, key, cacheKey string, size int) ([]byte, string, error) {
	c.observe("miss", 0)
	original, contentType, err := c.original(ctx, key)
	if err != nil {
		c.observe("error", 0)
		return nil, "", err
	}
	if size == 0 {
		return original, contentType, nil
	}
	resized, ok := resizeJPEG(original, size)
	if !ok {
		// A non-upscaled or undecodable image keeps the original bytes and type.
		return original, contentType, nil
	}
	c.storeMemory(cacheKey, resized, variantContentType)
	c.writeDisk(cacheKey, resized)
	return resized, variantContentType, nil
}

func (c *Cache) original(ctx context.Context, key string) ([]byte, string, error) {
	if data, contentType, ok := c.memoryGet(key); ok {
		return data, contentType, nil
	}
	c.mu.Lock()
	rawURL, ok := c.sources[key]
	c.mu.Unlock()
	if !ok {
		return nil, "", errors.New("unknown album art key")
	}
	data, contentType, err := c.fetch(ctx, rawURL)
	if err != nil {
		return nil, "", err
	}
	c.storeMemory(key, data, contentType)
	return data, contentType, nil
}

func (c *Cache) fetch(ctx context.Context, rawURL string) ([]byte, string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	response, err := c.client.Do(request)
	if err != nil {
		return nil, "", fmt.Errorf("fetch album art: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("fetch album art: Spotify image server returned %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, c.maxImage+1))
	if err != nil {
		return nil, "", fmt.Errorf("read album art: %w", err)
	}
	if int64(len(data)) > c.maxImage {
		return nil, "", errors.New("album art exceeds the image size limit")
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0]))
	switch contentType {
	case "image/jpeg", "image/jpg", "image/png", "image/webp":
	default:
		return nil, "", fmt.Errorf("unsupported album art content type %q", contentType)
	}
	return data, contentType, nil
}

func (c *Cache) memoryGet(cacheKey string) ([]byte, string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.entryLocked(cacheKey)
}

func (c *Cache) entryLocked(cacheKey string) ([]byte, string, bool) {
	item, ok := c.entries[cacheKey]
	if !ok {
		return nil, "", false
	}
	data := append([]byte(nil), item.data...)
	contentType := item.contentType
	c.touchEntryLocked(cacheKey)
	return data, contentType, true
}

func (c *Cache) storeMemory(cacheKey string, data []byte, contentType string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if existing, ok := c.entries[cacheKey]; ok {
		c.bytes += int64(len(data)) - int64(len(existing.data))
		existing.data = append([]byte(nil), data...)
		existing.contentType = contentType
		c.touchEntryLocked(cacheKey)
		c.evictMemoryLocked()
		return
	}
	c.entries[cacheKey] = &entry{data: append([]byte(nil), data...), contentType: contentType}
	c.order = append(c.order, cacheKey)
	c.bytes += int64(len(data))
	c.evictMemoryLocked()
}

func (c *Cache) touchEntryLocked(cacheKey string) {
	for index, existing := range c.order {
		if existing == cacheKey {
			c.order = append(c.order[:index], c.order[index+1:]...)
			break
		}
	}
	c.order = append(c.order, cacheKey)
}

func (c *Cache) touchSourceLocked(key string) {
	for index, existing := range c.sourceOrder {
		if existing == key {
			c.sourceOrder = append(c.sourceOrder[:index], c.sourceOrder[index+1:]...)
			break
		}
	}
	c.sourceOrder = append(c.sourceOrder, key)
}

func (c *Cache) evictMemoryLocked() {
	for len(c.order) > c.maxItems || c.bytes > c.maxBytes {
		if len(c.order) == 0 {
			return
		}
		oldest := c.order[0]
		c.order = c.order[1:]
		if item, ok := c.entries[oldest]; ok {
			c.bytes -= int64(len(item.data))
			delete(c.entries, oldest)
		}
	}
	if c.bytes < 0 {
		c.bytes = 0
	}
}

func (c *Cache) diskGet(cacheKey string) ([]byte, bool) {
	c.mu.Lock()
	dir := c.diskDir
	c.mu.Unlock()
	if dir == "" {
		return nil, false
	}
	path := filepath.Join(dir, cacheKey+".jpg")
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		c.forgetDisk(cacheKey)
		return nil, false
	}
	now := time.Now()
	_ = os.Chtimes(path, now, now)
	c.trackDisk(cacheKey, int64(len(data)))
	return data, true
}

func (c *Cache) trackDisk(cacheKey string, size int64) {
	c.mu.Lock()
	if c.diskDir == "" {
		c.mu.Unlock()
		return
	}
	if existing, ok := c.diskFiles[cacheKey]; ok {
		c.diskBytes += size - existing.size
	} else {
		c.diskBytes += size
	}
	c.diskFiles[cacheKey] = diskFile{size: size}
	c.touchDiskOrderLocked(cacheKey)
	evicted := c.evictDiskLocked()
	dir := c.diskDir
	c.mu.Unlock()
	for _, key := range evicted {
		_ = os.Remove(filepath.Join(dir, key+".jpg"))
	}
}

func (c *Cache) forgetDisk(cacheKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if existing, ok := c.diskFiles[cacheKey]; ok {
		c.diskBytes -= existing.size
		delete(c.diskFiles, cacheKey)
	}
	for index, existing := range c.diskOrder {
		if existing == cacheKey {
			c.diskOrder = append(c.diskOrder[:index], c.diskOrder[index+1:]...)
			break
		}
	}
	if c.diskBytes < 0 {
		c.diskBytes = 0
	}
}

func (c *Cache) touchDiskOrderLocked(cacheKey string) {
	for index, existing := range c.diskOrder {
		if existing == cacheKey {
			c.diskOrder = append(c.diskOrder[:index], c.diskOrder[index+1:]...)
			break
		}
	}
	c.diskOrder = append(c.diskOrder, cacheKey)
}

// evictDiskLocked removes the oldest indexed variants until the byte budget is
// satisfied and returns their keys so the caller can delete the files outside
// the cache lock.
func (c *Cache) evictDiskLocked() []string {
	var evicted []string
	for c.diskBytes > c.diskLimit && len(c.diskOrder) > 0 {
		oldest := c.diskOrder[0]
		c.diskOrder = c.diskOrder[1:]
		file, ok := c.diskFiles[oldest]
		if !ok {
			continue
		}
		delete(c.diskFiles, oldest)
		c.diskBytes -= file.size
		evicted = append(evicted, oldest)
	}
	if c.diskBytes < 0 {
		c.diskBytes = 0
	}
	return evicted
}

func (c *Cache) writeDisk(cacheKey string, data []byte) {
	c.mu.Lock()
	dir := c.diskDir
	c.mu.Unlock()
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	temp, err := os.CreateTemp(dir, ".art-*.tmp")
	if err != nil {
		return
	}
	tempName := temp.Name()
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		_ = os.Remove(tempName)
		return
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempName)
		return
	}
	if err := os.Rename(tempName, filepath.Join(dir, cacheKey+".jpg")); err != nil {
		_ = os.Remove(tempName)
		return
	}
	c.trackDisk(cacheKey, int64(len(data)))
}

// resizeJPEG scales an image to fit within size while preserving its aspect
// ratio. It never upscales: images already within the bounds return ok=false so
// the caller serves the original untouched.
func resizeJPEG(original []byte, size int) ([]byte, bool) {
	source, _, err := image.Decode(bytes.NewReader(original))
	if err != nil {
		return nil, false
	}
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, false
	}
	targetWidth, targetHeight := fitSize(width, height, size)
	if targetWidth == width && targetHeight == height {
		return nil, false
	}
	destination := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	draw.CatmullRom.Scale(destination, destination.Bounds(), source, bounds, draw.Src, nil)
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, destination, &jpeg.Options{Quality: 85}); err != nil {
		return nil, false
	}
	return buffer.Bytes(), true
}

func fitSize(width, height, limit int) (int, int) {
	if width <= limit && height <= limit {
		return width, height
	}
	if width >= height {
		scaled := height * limit / width
		if scaled < 1 {
			scaled = 1
		}
		return limit, scaled
	}
	scaled := width * limit / height
	if scaled < 1 {
		scaled = 1
	}
	return scaled, limit
}

func normalizeSize(size int) int {
	if supportedSize(size) {
		return size
	}
	return 0
}

func supportedSize(size int) bool {
	switch size {
	case 64, 128, 240, 640:
		return true
	default:
		return false
	}
}

func variantKey(key string, size int) string {
	return fmt.Sprintf("%s@%d", key, size)
}

func validKey(key string) bool {
	if len(key) != 64 {
		return false
	}
	for _, char := range key {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

func validVariantKey(cacheKey string) bool {
	key, sizeText, ok := strings.Cut(cacheKey, "@")
	if !ok || !validKey(key) {
		return false
	}
	size, err := strconv.Atoi(sizeText)
	return err == nil && supportedSize(size)
}

// ServeHTTP serves /art/<opaque-key>?size=<variant>. It is wired into the
// application asset handler; requests never contain the remote URL. A "served"
// observation records only a GET body write that completed without an error, so
// aborted or partial writes do not inflate delivered-image bytes.
func (c *Cache) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	const prefix = "/art/"
	if !strings.HasPrefix(request.URL.Path, prefix) {
		http.NotFound(writer, request)
		return
	}
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		writer.Header().Set("Allow", "GET, HEAD")
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	key := strings.TrimPrefix(request.URL.Path, prefix)
	if len(key) != 64 || strings.Contains(key, "/") {
		http.NotFound(writer, request)
		return
	}
	size := 0
	if raw := request.URL.Query().Get("size"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			size = parsed
		}
	}
	data, contentType, err := c.Get(request.Context(), key, size)
	if err != nil {
		http.Error(writer, "album art unavailable", http.StatusNotFound)
		return
	}
	writer.Header().Set("Content-Type", contentType)
	writer.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	writer.Header().Set("Cache-Control", "public, max-age=86400, immutable")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.WriteHeader(http.StatusOK)
	if request.Method == http.MethodGet {
		if written, err := writer.Write(data); err == nil {
			c.observe("served", int64(written))
		}
	}
}
