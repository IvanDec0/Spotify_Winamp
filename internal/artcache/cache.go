// Package artcache proxies album art through Go. The frontend never receives a
// remote image URL, only an opaque local key. The cache is in-memory and
// bounded so Cadence does not persist a listening history.
package artcache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	defaultMaxEntries = 96
	defaultMaxBytes   = 48 << 20
	defaultMaxImage   = 3 << 20
)

type entry struct {
	url         string
	data        []byte
	contentType string
}

// Cache stores a bounded set of remote image URLs and their fetched bytes.
type Cache struct {
	mu       sync.Mutex
	client   *http.Client
	entries  map[string]*entry
	order    []string
	bytes    int64
	maxItems int
	maxBytes int64
	maxImage int64
}

func New(client *http.Client) *Cache {
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second}
	}
	return &Cache{
		client:   client,
		entries:  make(map[string]*entry),
		maxItems: defaultMaxEntries,
		maxBytes: defaultMaxBytes,
		maxImage: defaultMaxImage,
	}
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
	if existing, ok := c.entries[key]; ok {
		existing.url = rawURL
		c.touchLocked(key)
		return key
	}
	c.entries[key] = &entry{url: rawURL}
	c.order = append(c.order, key)
	c.evictLocked()
	return key
}

// Get returns cached image bytes, fetching them from the registered URL on a
// miss. Content types are restricted to formats browsers render safely.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, string, error) {
	c.mu.Lock()
	item, ok := c.entries[key]
	if !ok {
		c.mu.Unlock()
		return nil, "", errors.New("unknown album art key")
	}
	url := item.url
	if len(item.data) > 0 {
		data := append([]byte(nil), item.data...)
		contentType := item.contentType
		c.touchLocked(key)
		c.mu.Unlock()
		return data, contentType, nil
	}
	c.mu.Unlock()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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

	c.mu.Lock()
	if current, ok := c.entries[key]; ok && len(current.data) == 0 {
		current.data = append([]byte(nil), data...)
		current.contentType = contentType
		c.bytes += int64(len(data))
		c.touchLocked(key)
		c.evictLocked()
	}
	c.mu.Unlock()
	return data, contentType, nil
}

func (c *Cache) touchLocked(key string) {
	for index, existing := range c.order {
		if existing == key {
			c.order = append(c.order[:index], c.order[index+1:]...)
			break
		}
	}
	c.order = append(c.order, key)
}

func (c *Cache) evictLocked() {
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
}

// ServeHTTP serves /art/<opaque-key>. It is wired into the application asset
// handler; requests never contain the remote URL.
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
	data, contentType, err := c.Get(request.Context(), key)
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
		_, _ = writer.Write(data)
	}
}
