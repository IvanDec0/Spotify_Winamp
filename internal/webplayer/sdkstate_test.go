package webplayer

import (
	"strings"
	"testing"
)

func TestSanitizeSDKStateCapsFields(t *testing.T) {
	state := &SDKState{
		PositionMS: -10,
		DurationMS: -20,
		Track: SDKTrack{
			URI:        strings.Repeat("u", sdkTrackStringLimit+10),
			Name:       strings.Repeat("n", sdkTrackStringLimit+10),
			Album:      strings.Repeat("a", sdkTrackStringLimit+10),
			ImageURL:   strings.Repeat("i", sdkImageURLLimit+10),
			DurationMS: -30,
			Artists:    []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"},
		},
	}
	got := sanitizeSDKState(state)
	if got == nil {
		t.Fatal("sanitizeSDKState() = nil")
	}
	if got.PositionMS != 0 || got.DurationMS != 0 || got.Track.DurationMS != 0 {
		t.Fatalf("negative durations were not clamped: %#v", got)
	}
	if len([]rune(got.Track.URI)) != sdkTrackStringLimit ||
		len([]rune(got.Track.Name)) != sdkTrackStringLimit ||
		len([]rune(got.Track.Album)) != sdkTrackStringLimit ||
		len([]rune(got.Track.ImageURL)) != sdkImageURLLimit {
		t.Fatalf("string caps were not applied: %#v", got.Track)
	}
	if len(got.Track.Artists) != sdkArtistLimit {
		t.Fatalf("artist count = %d, want %d", len(got.Track.Artists), sdkArtistLimit)
	}
}

func TestSanitizeSDKStateNil(t *testing.T) {
	if got := sanitizeSDKState(nil); got != nil {
		t.Fatalf("sanitizeSDKState(nil) = %#v, want nil", got)
	}
}
