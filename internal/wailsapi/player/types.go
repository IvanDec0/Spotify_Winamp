package playerapi

import "cadence/internal/playback"

// Type aliases keep the Wails service signatures readable while the domain
// types stay owned by playback.
type Provider = playback.Provider
type PlayerState = playback.PlayerState
type Device = playback.Device
type QueueItem = playback.QueueItem
type RepeatMode = playback.RepeatMode
