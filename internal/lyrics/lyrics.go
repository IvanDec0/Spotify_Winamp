// Package lyrics fetches time-synced lyrics from LRCLIB and caches the result
// locally. It never touches playback: the frontend derives the active line from
// the player position.
package lyrics

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ErrNotFound means the provider has no lyrics for the requested track.
var ErrNotFound = errors.New("lyrics not found")

const (
	defaultBaseURL = "https://lrclib.net/api"
	defaultTTL     = 7 * 24 * time.Hour
	notFoundTTL    = 6 * time.Hour
	userAgent      = "Cadence/0.2.0 (https://github.com/ivandec0/cadence)"
)

// Cache bounds. Lyrics files are small; these caps keep the directory from
// growing forever across months of listening.
var (
	maxCacheFiles = 2000
	maxCacheBytes = int64(16 << 20)
)

// Track is the metadata sent to the lyrics provider.
type Track struct {
	Title      string
	Artists    []string
	Album      string
	DurationMS int
	URI        string
}

// Line is one lyric line. TimeMS is -1 when the provider only has plain text.
type Line struct {
	TimeMS int    `json:"timeMs"`
	Text   string `json:"text"`
}

// Result is the normalized lyrics payload returned to the frontend.
type Result struct {
	Available    bool   `json:"available"`
	Instrumental bool   `json:"instrumental"`
	Synced       bool   `json:"synced"`
	Source       string `json:"source"`
	Reason       string `json:"reason,omitempty"`
	Lines        []Line `json:"lines"`
}

// Options configures a Client.
type Options struct {
	HTTPClient *http.Client
	BaseURL    string
	CacheDir   string
	TTL        time.Duration
	Now        func() time.Time
}

// Client fetches and caches lyrics.
type Client struct {
	http     *http.Client
	baseURL  string
	cacheDir string
	ttl      time.Duration
	now      func() time.Time
}

// NewClient creates a lyrics client. A nil HTTP client uses an 8 second timeout
// and an empty cache directory disables disk caching.
func NewClient(options Options) *Client {
	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 8 * time.Second}
	}
	baseURL := strings.TrimRight(options.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	ttl := options.TTL
	if ttl <= 0 {
		ttl = defaultTTL
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Client{
		http:     httpClient,
		baseURL:  baseURL,
		cacheDir: options.CacheDir,
		ttl:      ttl,
		now:      now,
	}
}

// Fetch returns lyrics for a track. Positive and negative results are cached;
// a provider miss is reported as ErrNotFound.
func (c *Client) Fetch(ctx context.Context, track Track) (Result, error) {
	track.Title = strings.TrimSpace(track.Title)
	if track.Title == "" || len(track.Artists) == 0 {
		return Result{Available: false, Reason: "no_track", Source: "LRCLIB"}, nil
	}

	key := cacheKey(track)
	if cached, ok := c.readCache(key); ok {
		if cached.NotFound {
			return Result{}, ErrNotFound
		}
		return cached.Result, nil
	}

	result, err := c.fetchRemote(ctx, track)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.writeCache(key, cacheEntry{
				FetchedAt: c.now(),
				NotFound:  true,
			})
		}
		return Result{}, err
	}
	c.writeCache(key, cacheEntry{
		FetchedAt: c.now(),
		Result:    result,
	})
	return result, nil
}

func (c *Client) fetchRemote(ctx context.Context, track Track) (Result, error) {
	candidates, err := c.get(ctx, track)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return Result{}, err
		}
		candidates, err = c.search(ctx, track)
		if err != nil {
			return Result{}, err
		}
	}
	if len(candidates) == 0 {
		return Result{}, ErrNotFound
	}
	best := chooseBest(candidates, track)
	if best == nil {
		return Result{}, ErrNotFound
	}
	return normalize(*best), nil
}

func (c *Client) get(ctx context.Context, track Track) ([]providerTrack, error) {
	query := trackQuery(track)
	var item providerTrack
	status, err := c.doJSON(ctx, "/get?"+query.Encode(), &item)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("lyrics provider returned %d", status)
	}
	return []providerTrack{item}, nil
}

func (c *Client) search(ctx context.Context, track Track) ([]providerTrack, error) {
	query := trackQuery(track)
	var items []providerTrack
	status, err := c.doJSON(ctx, "/search?"+query.Encode(), &items)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("lyrics provider returned %d", status)
	}
	return items, nil
}

func (c *Client) doJSON(ctx context.Context, path string, target any) (int, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return 0, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", userAgent)
	response, err := c.http.Do(request)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode != http.StatusOK {
		return response.StatusCode, nil
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return response.StatusCode, fmt.Errorf("decode lyrics response: %w", err)
	}
	return response.StatusCode, nil
}

func trackQuery(track Track) url.Values {
	values := url.Values{}
	values.Set("track_name", track.Title)
	values.Set("artist_name", strings.Join(track.Artists, ", "))
	if track.Album != "" {
		values.Set("album_name", track.Album)
	}
	if track.DurationMS > 0 {
		values.Set("duration", fmt.Sprintf("%d", (track.DurationMS+500)/1000))
	}
	return values
}

func normalize(item providerTrack) Result {
	result := Result{
		Available:    true,
		Instrumental: item.Instrumental,
		Source:       "LRCLIB",
	}
	if item.Instrumental {
		return result
	}
	if lines := ParseLRC(item.SyncedLyrics); len(lines) > 0 {
		result.Synced = true
		result.Lines = lines
		return result
	}
	if lines := splitPlain(item.PlainLyrics); len(lines) > 0 {
		result.Lines = lines
		return result
	}
	return Result{Available: false, Reason: "not_found", Source: "LRCLIB"}
}

func splitPlain(raw string) []Line {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	lines := make([]Line, 0)
	for _, text := range strings.Split(raw, "\n") {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		lines = append(lines, Line{TimeMS: -1, Text: text})
	}
	return lines
}

type providerTrack struct {
	ID           int     `json:"id"`
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	PlainLyrics  string  `json:"plainLyrics"`
	SyncedLyrics string  `json:"syncedLyrics"`
}

func chooseBest(items []providerTrack, track Track) *providerTrack {
	var best *providerTrack
	bestScore := -1.0
	for index := range items {
		item := &items[index]
		score := matchScore(*item, track)
		if score > bestScore {
			best = item
			bestScore = score
		}
	}
	return best
}

func matchScore(item providerTrack, track Track) float64 {
	score := 0.0
	if sameText(item.TrackName, track.Title) {
		score += 4
	}
	if sameText(item.ArtistName, strings.Join(track.Artists, ", ")) ||
		containsArtist(item.ArtistName, track.Artists) {
		score += 3
	}
	if track.Album != "" && sameText(item.AlbumName, track.Album) {
		score += 1
	}
	if track.DurationMS > 0 && item.Duration > 0 {
		diff := item.Duration - float64(track.DurationMS)/1000
		if diff < 0 {
			diff = -diff
		}
		switch {
		case diff <= 2:
			score += 3
		case diff <= 5:
			score += 2
		case diff <= 10:
			score += 1
		}
	}
	return score
}

func sameText(left, right string) bool {
	return normalizeText(left) == normalizeText(right)
}

func containsArtist(value string, artists []string) bool {
	normalized := normalizeText(value)
	for _, artist := range artists {
		if candidate := normalizeText(artist); candidate != "" && strings.Contains(normalized, candidate) {
			return true
		}
	}
	return false
}

func normalizeText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(
		"’", "'",
		"‘", "'",
		"“", "\"",
		"”", "\"",
		"–", "-",
		"—", "-",
	)
	return replacer.Replace(value)
}

type cacheEntry struct {
	FetchedAt time.Time `json:"fetchedAt"`
	NotFound  bool      `json:"notFound"`
	Result    Result    `json:"result"`
}

func (c *Client) readCache(key string) (cacheEntry, bool) {
	if c.cacheDir == "" {
		return cacheEntry{}, false
	}
	path := filepath.Join(c.cacheDir, key+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return cacheEntry{}, false
	}
	var entry cacheEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return cacheEntry{}, false
	}
	ttl := c.ttl
	if entry.NotFound {
		ttl = notFoundTTL
	}
	if c.now().Sub(entry.FetchedAt) > ttl {
		return cacheEntry{}, false
	}
	return entry, true
}

func (c *Client) writeCache(key string, entry cacheEntry) {
	if c.cacheDir == "" {
		return
	}
	if err := os.MkdirAll(c.cacheDir, 0o755); err != nil {
		return
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		return
	}
	path := filepath.Join(c.cacheDir, key+".json")
	temp := path + ".tmp"
	if err := os.WriteFile(temp, raw, 0o644); err != nil {
		return
	}
	_ = os.Rename(temp, path)
	c.pruneCache()
}

// pruneCache removes the oldest lyrics files until the directory is under the
// file-count and byte limits. It is best effort.
func (c *Client) pruneCache() {
	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		return
	}
	type cacheFile struct {
		name    string
		size    int64
		modTime time.Time
	}
	files := make([]cacheFile, 0, len(entries))
	var total int64
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, cacheFile{name: entry.Name(), size: info.Size(), modTime: info.ModTime()})
		total += info.Size()
	}
	if len(files) <= maxCacheFiles && total <= maxCacheBytes {
		return
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})
	for index, file := range files {
		if len(files)-index <= maxCacheFiles && total <= maxCacheBytes {
			break
		}
		if err := os.Remove(filepath.Join(c.cacheDir, file.name)); err == nil {
			total -= file.size
		}
	}
}

func cacheKey(track Track) string {
	seed := strings.Join([]string{
		track.URI,
		strings.ToLower(track.Title),
		strings.ToLower(strings.Join(track.Artists, ",")),
		strings.ToLower(track.Album),
		fmt.Sprintf("%d", track.DurationMS/1000),
	}, "|")
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:16])
}
