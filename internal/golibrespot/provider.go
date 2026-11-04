package golibrespot

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"cadence/internal/artwork"
	"cadence/internal/engine"
	"cadence/internal/player"
)

const providerName = "spotify"

const contextRefreshInterval = 5 * time.Second

// Provider maps the local go-librespot status to the player UI.
type Provider struct {
	runtime *engine.Supervisor
	art     *artwork.Cache

	mu              sync.Mutex
	contextURI      string
	contextTracks   engine.ContextTracks
	contextFetched  time.Time
	contextFailedAt time.Time
}

func NewProvider(runtime *engine.Supervisor, art *artwork.Cache) *Provider {
	return &Provider{runtime: runtime, art: art}
}

func (p *Provider) State(ctx context.Context) (player.PlayerState, error) {
	client := p.runtime.Client()
	if client == nil {
		return unavailableState("The local audio engine is starting."), nil
	}
	status, err := client.Status(ctx)
	if err != nil {
		return unavailableState("Cannot reach the local audio engine: " + err.Error()), nil
	}
	if status == nil {
		state := unavailableState("Pair Cadence with Spotify to start playing music.")
		state.Error.Code = "not_authenticated"
		return state, nil
	}

	state := player.PlayerState{
		Provider: providerName,
		Status:   player.StatusStopped,
		Devices:  []player.Device{},
		Queue:    []player.QueueItem{},
	}
	if !status.Stopped && !status.Paused {
		state.Status = player.StatusPlaying
	} else if status.Paused {
		state.Status = player.StatusPaused
	}
	if status.Track != nil {
		state.Track = p.mapTrack(status.Track)
		state.DurationMS = status.Track.Duration
		state.ProgressMS = int(status.Track.Position)
		if state.ProgressMS < 0 {
			state.ProgressMS = 0
		}
		if state.DurationMS > 0 && state.ProgressMS > state.DurationMS {
			state.ProgressMS = state.DurationMS
		}
		state.CanControl = true
		state.CanSeek = state.DurationMS > 0
	}
	state.Shuffle = status.ShuffleContext
	switch {
	case status.RepeatTrack:
		state.Repeat = player.RepeatTrack
	case status.RepeatContext:
		state.Repeat = player.RepeatContext
	default:
		state.Repeat = player.RepeatOff
	}
	volume := int(status.Volume)
	if status.VolumeSteps > 0 {
		volume = int(float64(status.Volume) * 100 / float64(status.VolumeSteps))
	}
	state.Volume = clamp(volume, 0, 100)
	device := player.Device{
		ID:            status.DeviceID,
		Name:          status.DeviceName,
		Type:          strings.ToLower(status.DeviceType),
		Active:        true,
		VolumePercent: state.Volume,
	}
	state.Device = device
	state.Devices = []player.Device{device}
	state.Queue = p.queueFor(ctx, client, status)
	state.Revision = 1
	return state, nil
}

func (p *Provider) Play(ctx context.Context) error {
	client := p.runtime.Client()
	if client == nil {
		return fmt.Errorf("the local audio engine is not running")
	}
	return client.Resume(ctx)
}

func (p *Provider) Pause(ctx context.Context) error {
	client := p.runtime.Client()
	if client == nil {
		return fmt.Errorf("the local audio engine is not running")
	}
	return client.Pause(ctx)
}

func (p *Provider) Toggle(ctx context.Context) error {
	client := p.runtime.Client()
	if client == nil {
		return fmt.Errorf("the local audio engine is not running")
	}
	return client.PlayPause(ctx)
}

func (p *Provider) Next(ctx context.Context) error {
	client := p.runtime.Client()
	if client == nil {
		return fmt.Errorf("the local audio engine is not running")
	}
	return client.Next(ctx)
}

func (p *Provider) Previous(ctx context.Context) error {
	client := p.runtime.Client()
	if client == nil {
		return fmt.Errorf("the local audio engine is not running")
	}
	return client.Previous(ctx)
}

func (p *Provider) Seek(ctx context.Context, positionMS int) error {
	client := p.runtime.Client()
	if client == nil {
		return fmt.Errorf("the local audio engine is not running")
	}
	return client.Seek(ctx, int64(positionMS))
}

func (p *Provider) SetVolume(ctx context.Context, volume int) error {
	client := p.runtime.Client()
	if client == nil {
		return fmt.Errorf("the local audio engine is not running")
	}
	return client.SetVolume(ctx, clamp(volume, 0, 100))
}

func (p *Provider) SetShuffle(ctx context.Context, enabled bool) error {
	client := p.runtime.Client()
	if client == nil {
		return fmt.Errorf("the local audio engine is not running")
	}
	return client.SetShuffle(ctx, enabled)
}

func (p *Provider) SetRepeat(ctx context.Context, mode player.RepeatMode) error {
	client := p.runtime.Client()
	if client == nil {
		return fmt.Errorf("the local audio engine is not running")
	}
	switch mode {
	case player.RepeatTrack:
		if err := client.SetRepeatTrack(ctx, true); err != nil {
			return err
		}
		return client.SetRepeatContext(ctx, false)
	case player.RepeatContext:
		if err := client.SetRepeatTrack(ctx, false); err != nil {
			return err
		}
		return client.SetRepeatContext(ctx, true)
	case player.RepeatOff:
		if err := client.SetRepeatTrack(ctx, false); err != nil {
			return err
		}
		return client.SetRepeatContext(ctx, false)
	default:
		return fmt.Errorf("unsupported repeat mode %q", mode)
	}
}

func (p *Provider) Transfer(ctx context.Context, deviceID string) error {
	client := p.runtime.Client()
	if client == nil {
		return fmt.Errorf("the local audio engine is not running")
	}
	status, err := client.Status(ctx)
	if err != nil {
		return err
	}
	if status == nil {
		return fmt.Errorf("the local audio engine is not authenticated")
	}
	if deviceID == "" || deviceID == status.DeviceID {
		return nil
	}
	return fmt.Errorf("Cadence plays through the local audio output")
}

func (p *Provider) queueFor(ctx context.Context, client *engine.Client, status *engine.Status) []player.QueueItem {
	currentURI := ""
	if status.Track != nil {
		currentURI = status.Track.URI
	}
	if status.ContextURI != "" {
		tracks := p.context(ctx, client, status.ContextURI)
		if tracks.Ready && len(tracks.Items) > 0 {
			if queue := p.queueFromContext(tracks, currentURI); len(queue) > 0 {
				return queue
			}
		}
	}
	queue := make([]player.QueueItem, 0, 2)
	if status.Track != nil {
		queue = append(queue, player.QueueItem{Track: p.mapTrack(status.Track), Index: 0, IsCurrent: true})
	}
	if status.NextTrack != nil {
		queue = append(queue, player.QueueItem{Track: p.mapTrack(status.NextTrack), Index: len(queue), IsNext: true})
	}
	return queue
}

func (p *Provider) queueFromContext(tracks engine.ContextTracks, currentURI string) []player.QueueItem {
	items := make([]player.QueueItem, 0, len(tracks.Items))
	start := -1
	for index, item := range tracks.Items {
		if item.URI == currentURI {
			start = index
			break
		}
	}
	if start < 0 {
		return nil
	}
	nextMarked := false
	for index := start; index < len(tracks.Items); index++ {
		item := tracks.Items[index]
		track := item.Track
		if track.Name == "" && item.URI != currentURI {
			continue
		}
		queueItem := player.QueueItem{
			Track: p.mapTrack(&track),
			Index: len(items),
		}
		if item.URI == currentURI {
			queueItem.IsCurrent = true
		} else if !nextMarked {
			queueItem.IsNext = true
			nextMarked = true
		}
		items = append(items, queueItem)
		if len(items) >= 50 {
			break
		}
	}
	return items
}

func (p *Provider) context(ctx context.Context, client *engine.Client, uri string) engine.ContextTracks {
	p.mu.Lock()
	if p.contextURI == uri && (p.contextTracks.Ready || time.Since(p.contextFetched) < contextRefreshInterval) {
		tracks := p.contextTracks
		p.mu.Unlock()
		return tracks
	}
	p.mu.Unlock()

	tracks, err := client.ContextTracks(ctx, uri)
	p.mu.Lock()
	defer p.mu.Unlock()
	if err != nil {
		if time.Since(p.contextFailedAt) > contextRefreshInterval {
			p.contextFailedAt = time.Now()
			p.contextTracks = engine.ContextTracks{URI: uri}
			p.contextURI = uri
			p.contextFetched = time.Now()
		}
		return p.contextTracks
	}
	p.contextURI = uri
	p.contextTracks = tracks
	p.contextFetched = time.Now()
	return tracks
}

func (p *Provider) mapTrack(track *engine.Track) player.Track {
	if track == nil {
		return player.Track{Artists: []string{}}
	}
	cover := ""
	if track.AlbumCoverURL != nil && p.art != nil {
		if key := p.art.Register(*track.AlbumCoverURL); key != "" {
			cover = "/art/" + key
		}
	}
	return player.Track{
		ID:         track.URI,
		Title:      track.Name,
		Artists:    append([]string(nil), track.ArtistNames...),
		Album:      track.AlbumName,
		Cover:      cover,
		URI:        track.URI,
		DurationMS: track.Duration,
	}
}

func unavailableState(message string) player.PlayerState {
	return player.PlayerState{
		Provider: providerName,
		Status:   player.StatusStopped,
		Track:    player.Track{Artists: []string{}},
		Devices:  []player.Device{},
		Queue:    []player.QueueItem{},
		Error: &player.PlayerError{
			Code:    "audio_unavailable",
			Message: message,
		},
		Revision: 1,
	}
}

func clamp(value, minimum, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}
