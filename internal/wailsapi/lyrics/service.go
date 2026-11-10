// Package lyricsapi exposes the lyrics client to the Wails frontend.
package lyricsapi

import (
	"context"
	"errors"

	"cadence/internal/lyrics"
)

// Track is the track metadata used for a lyrics lookup.
type Track struct {
	Title      string   `json:"title"`
	Artists    []string `json:"artists"`
	Album      string   `json:"album"`
	DurationMS int      `json:"durationMs"`
	URI        string   `json:"uri"`
}

// Line is one lyric line. TimeMS is -1 for unsynced plain text.
type Line struct {
	TimeMS int    `json:"timeMs"`
	Text   string `json:"text"`
}

// Result is returned even when lyrics are unavailable, so the UI can show a
// useful empty state instead of a generic error.
type Result struct {
	Available    bool   `json:"available"`
	Instrumental bool   `json:"instrumental"`
	Synced       bool   `json:"synced"`
	Source       string `json:"source"`
	Reason       string `json:"reason,omitempty"`
	Lines        []Line `json:"lines"`
}

// Service is the Wails-facing lyrics service.
type Service struct {
	client *lyrics.Client
}

// NewService creates a lyrics service.
func NewService(client *lyrics.Client) *Service {
	return &Service{client: client}
}

// Get fetches lyrics for the current track. Missing lyrics are a normal result;
// transport failures are returned as errors and surfaced as a retryable state.
func (s *Service) Get(track Track) (Result, error) {
	if s.client == nil {
		return Result{Available: false, Reason: "unavailable", Source: "LRCLIB"}, nil
	}
	result, err := s.client.Fetch(context.Background(), lyrics.Track{
		Title:      track.Title,
		Artists:    track.Artists,
		Album:      track.Album,
		DurationMS: track.DurationMS,
		URI:        track.URI,
	})
	if err != nil {
		if errors.Is(err, lyrics.ErrNotFound) {
			return Result{Available: false, Reason: "not_found", Source: "LRCLIB"}, nil
		}
		return Result{}, err
	}
	return Result{
		Available:    result.Available,
		Instrumental: result.Instrumental,
		Synced:       result.Synced,
		Source:       result.Source,
		Reason:       result.Reason,
		Lines:        toLines(result.Lines),
	}, nil
}

func toLines(lines []lyrics.Line) []Line {
	result := make([]Line, 0, len(lines))
	for _, line := range lines {
		result = append(result, Line{TimeMS: line.TimeMS, Text: line.Text})
	}
	return result
}
