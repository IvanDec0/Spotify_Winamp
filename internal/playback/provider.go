package playback

import "context"

// Provider is the contract shared by the demo player and the future Spotify
// Connect provider. Provider implementations must return copies of their state
// so callers cannot mutate internal slices.
type Provider interface {
	State(ctx context.Context) (PlayerState, error)
	Play(ctx context.Context) error
	Pause(ctx context.Context) error
	Toggle(ctx context.Context) error
	Next(ctx context.Context) error
	Previous(ctx context.Context) error
	Seek(ctx context.Context, positionMS int) error
	SetVolume(ctx context.Context, volume int) error
	SetShuffle(ctx context.Context, enabled bool) error
	SetRepeat(ctx context.Context, mode RepeatMode) error
	Transfer(ctx context.Context, deviceID string) error
}
