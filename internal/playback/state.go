package playback

// Status is the playback state reported by a provider.
type Status string

const (
	StatusPlaying Status = "playing"
	StatusPaused  Status = "paused"
	StatusStopped Status = "stopped"
)

// RepeatMode controls what happens at the end of a track or context.
type RepeatMode string

const (
	RepeatOff     RepeatMode = "off"
	RepeatContext RepeatMode = "context"
	RepeatTrack   RepeatMode = "track"
)

// PlayerError is a token-free, user-facing playback error. The frontend uses
// the code for targeted copy and the message as a safe fallback.
type PlayerError struct {
	Code              string `json:"code"`
	Message           string `json:"message"`
	Retryable         bool   `json:"retryable"`
	RetryAfterSeconds int    `json:"retryAfterSeconds,omitempty"`
}

// Track is the provider-neutral metadata needed by the player UI.
type Track struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Artists    []string `json:"artists"`
	Album      string   `json:"album"`
	Cover      string   `json:"cover"`
	URI        string   `json:"uri"`
	DurationMS int      `json:"durationMs"`
}

// Device is a playback target. The demo provider uses fictional devices; the
// local engine reports Cadence itself as the active device.
type Device struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	Active        bool   `json:"active"`
	VolumePercent int    `json:"volumePercent"`
	Restricted    bool   `json:"restricted"`
}

// QueueItem describes one row in the queue.
type QueueItem struct {
	Track     Track `json:"track"`
	Index     int   `json:"index"`
	IsCurrent bool  `json:"isCurrent"`
	IsNext    bool  `json:"isNext"`
}

// PlayerState is the complete, token-free state sent to the frontend.
type PlayerState struct {
	Provider   string       `json:"provider"`
	Status     Status       `json:"status"`
	Track      Track        `json:"track"`
	ProgressMS int          `json:"progressMs"`
	DurationMS int          `json:"durationMs"`
	Volume     int          `json:"volume"`
	Shuffle    bool         `json:"shuffle"`
	Repeat     RepeatMode   `json:"repeat"`
	Device     Device       `json:"device"`
	Devices    []Device     `json:"devices"`
	Queue      []QueueItem  `json:"queue"`
	CanSeek    bool         `json:"canSeek"`
	CanControl bool         `json:"canControl"`
	Error      *PlayerError `json:"error,omitempty"`
	Revision   uint64       `json:"revision"`
}
