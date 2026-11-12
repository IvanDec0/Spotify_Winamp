package webplayer

import "time"

const (
	sdkTrackStringLimit  = 512
	sdkImageURLLimit     = 2048
	sdkArtistLimit       = 8
	sdkArtistStringLimit = 128
)

// SDKTrack is the subset of the Spotify SDK current track that the helper may
// send over the control socket. It never contains tokens.
type SDKTrack struct {
	URI        string   `json:"uri,omitempty"`
	Name       string   `json:"name,omitempty"`
	Artists    []string `json:"artists,omitempty"`
	Album      string   `json:"album,omitempty"`
	ImageURL   string   `json:"imageUrl,omitempty"`
	DurationMS int64    `json:"durationMs,omitempty"`
}

// SDKState is one player_state_changed snapshot reported by the SDK page.
// The SDK does not emit progress ticks, so the provider interpolates the
// position from PositionMS and the time the snapshot was received.
type SDKState struct {
	IsPlaying  bool     `json:"isPlaying"`
	Paused     bool     `json:"paused"`
	PositionMS int64    `json:"positionMs"`
	DurationMS int64    `json:"durationMs"`
	Track      SDKTrack `json:"track"`
}

// SDKStateProvider is implemented by backends that receive push snapshots from
// the SDK. Providers fall back to Web API polling when it is absent.
type SDKStateProvider interface {
	SDKState() (SDKState, time.Time, bool)
}

func sanitizeSDKState(state *SDKState) *SDKState {
	if state == nil {
		return nil
	}
	copied := *state
	copied.PositionMS = maxInt64(copied.PositionMS, 0)
	copied.DurationMS = maxInt64(copied.DurationMS, 0)
	copied.Track.URI = truncateRunes(state.Track.URI, sdkTrackStringLimit)
	copied.Track.Name = truncateRunes(state.Track.Name, sdkTrackStringLimit)
	copied.Track.Album = truncateRunes(state.Track.Album, sdkTrackStringLimit)
	copied.Track.ImageURL = truncateRunes(state.Track.ImageURL, sdkImageURLLimit)
	copied.Track.DurationMS = maxInt64(copied.Track.DurationMS, 0)
	copied.Track.Artists = append([]string(nil), state.Track.Artists...)
	if len(copied.Track.Artists) > sdkArtistLimit {
		copied.Track.Artists = copied.Track.Artists[:sdkArtistLimit]
	}
	for index := range copied.Track.Artists {
		copied.Track.Artists[index] = truncateRunes(copied.Track.Artists[index], sdkArtistStringLimit)
	}
	return &copied
}

func maxInt64(value, minimum int64) int64 {
	if value < minimum {
		return minimum
	}
	return value
}
