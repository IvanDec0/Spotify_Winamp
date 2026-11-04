package player

import "cadence/internal/playback"

// Temporary aliases while the Wails player service moves to
// internal/wailsapi/player. The domain types live in internal/playback.
type Provider = playback.Provider
type Status = playback.Status
type RepeatMode = playback.RepeatMode
type PlayerError = playback.PlayerError
type Track = playback.Track
type Device = playback.Device
type QueueItem = playback.QueueItem
type PlayerState = playback.PlayerState
type DemoProvider = playback.DemoProvider

const (
	StatusPlaying = playback.StatusPlaying
	StatusPaused  = playback.StatusPaused
	StatusStopped = playback.StatusStopped

	RepeatOff     = playback.RepeatOff
	RepeatContext = playback.RepeatContext
	RepeatTrack   = playback.RepeatTrack
)

func NewDemoProvider() *DemoProvider {
	return playback.NewDemoProvider()
}
