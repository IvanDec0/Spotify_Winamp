package playback

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"cadence/internal/artwork"
	"cadence/internal/engine"
)

const providerName = "spotify"

const contextRefreshInterval = 5 * time.Second

// streamRefreshInterval coalesces engine events into at most 10 refreshes per
// second; streamReconcileInterval repairs events missed during a reconnect.
var (
	streamRefreshInterval   = 100 * time.Millisecond
	streamReconcileInterval = 15 * time.Second
)

// EngineProvider maps the local go-librespot status to the player UI.
type EngineProvider struct {
	runtime *engine.Supervisor
	art     *artwork.Cache

	mu              sync.Mutex
	contextURI      string
	contextTracks   engine.ContextTracks
	contextFetched  time.Time
	contextFailedAt time.Time

	streamMu   sync.Mutex
	revision   uint64
	lastState  PlayerState
	hasState   bool
	appliedSeq uint64
	fetchSeq   atomic.Uint64
}

func NewEngineProvider(runtime *engine.Supervisor, art *artwork.Cache) *EngineProvider {
	return &EngineProvider{runtime: runtime, art: art}
}

func (p *EngineProvider) State(ctx context.Context) (PlayerState, error) {
	state, seq, err := p.fetch(ctx)
	if err != nil {
		return PlayerState{}, err
	}
	return p.publish(state, seq, nil), nil
}

// Queue fetches the playback queue on demand. It is separate from State so
// idle state traffic never pays for context enumeration.
func (p *EngineProvider) Queue(ctx context.Context) ([]QueueItem, error) {
	client := p.runtime.Client()
	if client == nil {
		return nil, fmt.Errorf("the local audio engine is not running")
	}
	status, err := client.Status(ctx)
	if err != nil {
		return nil, err
	}
	if status == nil {
		return []QueueItem{}, nil
	}
	return p.queueFor(ctx, client, status), nil
}

// Stream publishes the initial state, then refreshes only when the engine
// sends an event and reconciles every 15 seconds. It returns when ctx is
// cancelled or the event stream fails; callers reconnect with backoff.
func (p *EngineProvider) Stream(ctx context.Context, emit func(PlayerState)) error {
	client := p.runtime.Client()
	if client == nil {
		// Publish the stopped state before returning so a crashed engine does
		// not leave the frontend interpolating the last known position.
		p.publish(unavailableState("The local audio engine is not running."), p.fetchSeq.Add(1), emit)
		return errors.New("the local audio engine is not running")
	}

	// Watch before the initial fetch: events that land while it is in flight
	// set the dirty flag and are refreshed on the next coalescing tick.
	watchCtx, stopWatch := context.WithCancel(ctx)
	defer stopWatch()
	eventsDone := make(chan error, 1)
	var dirty atomic.Bool
	go func() {
		eventsDone <- client.WatchEvents(watchCtx, func(engine.Event) {
			dirty.Store(true)
		})
	}()

	if _, err := p.refresh(ctx, emit); err != nil {
		return err
	}

	refreshTicker := time.NewTicker(streamRefreshInterval)
	defer refreshTicker.Stop()
	reconcileTicker := time.NewTicker(streamReconcileInterval)
	defer reconcileTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-eventsDone:
			if err == nil {
				err = errors.New("the go-librespot event stream closed")
			}
			return err
		case <-refreshTicker.C:
			if !dirty.Swap(false) {
				continue
			}
			if _, err := p.refresh(ctx, emit); err != nil {
				return err
			}
		case <-reconcileTicker.C:
			if _, err := p.refresh(ctx, emit); err != nil {
				return err
			}
			// The reconcile snapshot already covers events that landed while
			// it was in flight, so do not refresh again on the next tick.
			dirty.Store(false)
		}
	}
}

func (p *EngineProvider) refresh(ctx context.Context, emit func(PlayerState)) (PlayerState, error) {
	state, seq, err := p.fetch(ctx)
	if err != nil {
		return PlayerState{}, err
	}
	return p.publish(state, seq, emit), nil
}

// publish stamps a new revision when the state changed and calls emit only
// then. Fetches carry a start sequence: a slow response that arrives after a
// newer capture was applied is discarded instead of reviving stale state.
// Unchanged states keep the previous revision and capture timestamp.
func (p *EngineProvider) publish(state PlayerState, seq uint64, emit func(PlayerState)) PlayerState {
	p.streamMu.Lock()
	if seq < p.appliedSeq {
		latest := p.lastState
		p.streamMu.Unlock()
		return latest
	}
	p.appliedSeq = seq
	if p.hasState && sameStreamedState(p.lastState, state) {
		state.Revision = p.revision
		state.CapturedAt = p.lastState.CapturedAt
		p.streamMu.Unlock()
		return state
	}
	p.revision++
	state.Revision = p.revision
	state.CapturedAt = time.Now().UnixMilli()
	p.lastState = state
	p.hasState = true
	p.streamMu.Unlock()
	if emit != nil {
		emit(state)
	}
	return state
}

// sameStreamedState compares two states without their transport metadata.
func sameStreamedState(left, right PlayerState) bool {
	left.Revision, right.Revision = 0, 0
	left.CapturedAt, right.CapturedAt = 0, 0
	return reflect.DeepEqual(left, right)
}

func (p *EngineProvider) fetch(ctx context.Context) (PlayerState, uint64, error) {
	seq := p.fetchSeq.Add(1)
	client := p.runtime.Client()
	if client == nil {
		return unavailableState("The local audio engine is starting."), seq, nil
	}
	status, err := client.Status(ctx)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return PlayerState{}, seq, ctxErr
		}
		return unavailableState("Cannot reach the local audio engine: " + err.Error()), seq, nil
	}
	if status == nil {
		state := unavailableState("Pair Cadence with Spotify to start playing music.")
		state.Error.Code = "not_authenticated"
		return state, seq, nil
	}

	state := PlayerState{
		Provider: providerName,
		Status:   StatusStopped,
		Devices:  []Device{},
		Queue:    []QueueItem{},
	}
	if !status.Stopped && !status.Paused {
		state.Status = StatusPlaying
	} else if status.Paused {
		state.Status = StatusPaused
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
		state.Repeat = RepeatTrack
	case status.RepeatContext:
		state.Repeat = RepeatContext
	default:
		state.Repeat = RepeatOff
	}
	volume := int(status.Volume)
	if status.VolumeSteps > 0 {
		volume = int(float64(status.Volume) * 100 / float64(status.VolumeSteps))
	}
	state.Volume = clamp(volume, 0, 100)
	device := Device{
		ID:            status.DeviceID,
		Name:          status.DeviceName,
		Type:          strings.ToLower(status.DeviceType),
		Active:        true,
		VolumePercent: state.Volume,
	}
	state.Device = device
	state.Devices = []Device{device}
	return state, seq, nil
}

func (p *EngineProvider) Play(ctx context.Context) error {
	client := p.runtime.Client()
	if client == nil {
		return fmt.Errorf("the local audio engine is not running")
	}
	return client.Resume(ctx)
}

func (p *EngineProvider) Pause(ctx context.Context) error {
	client := p.runtime.Client()
	if client == nil {
		return fmt.Errorf("the local audio engine is not running")
	}
	return client.Pause(ctx)
}

func (p *EngineProvider) Toggle(ctx context.Context) error {
	client := p.runtime.Client()
	if client == nil {
		return fmt.Errorf("the local audio engine is not running")
	}
	return client.PlayPause(ctx)
}

func (p *EngineProvider) Next(ctx context.Context) error {
	client := p.runtime.Client()
	if client == nil {
		return fmt.Errorf("the local audio engine is not running")
	}
	return client.Next(ctx)
}

func (p *EngineProvider) Previous(ctx context.Context) error {
	client := p.runtime.Client()
	if client == nil {
		return fmt.Errorf("the local audio engine is not running")
	}
	return client.Previous(ctx)
}

func (p *EngineProvider) Seek(ctx context.Context, positionMS int) error {
	client := p.runtime.Client()
	if client == nil {
		return fmt.Errorf("the local audio engine is not running")
	}
	return client.Seek(ctx, int64(positionMS))
}

func (p *EngineProvider) SetVolume(ctx context.Context, volume int) error {
	client := p.runtime.Client()
	if client == nil {
		return fmt.Errorf("the local audio engine is not running")
	}
	return client.SetVolume(ctx, clamp(volume, 0, 100))
}

func (p *EngineProvider) SetShuffle(ctx context.Context, enabled bool) error {
	client := p.runtime.Client()
	if client == nil {
		return fmt.Errorf("the local audio engine is not running")
	}
	return client.SetShuffle(ctx, enabled)
}

func (p *EngineProvider) SetRepeat(ctx context.Context, mode RepeatMode) error {
	client := p.runtime.Client()
	if client == nil {
		return fmt.Errorf("the local audio engine is not running")
	}
	switch mode {
	case RepeatTrack:
		if err := client.SetRepeatTrack(ctx, true); err != nil {
			return err
		}
		return client.SetRepeatContext(ctx, false)
	case RepeatContext:
		if err := client.SetRepeatTrack(ctx, false); err != nil {
			return err
		}
		return client.SetRepeatContext(ctx, true)
	case RepeatOff:
		if err := client.SetRepeatTrack(ctx, false); err != nil {
			return err
		}
		return client.SetRepeatContext(ctx, false)
	default:
		return fmt.Errorf("unsupported repeat mode %q", mode)
	}
}

func (p *EngineProvider) Transfer(ctx context.Context, deviceID string) error {
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

func (p *EngineProvider) queueFor(ctx context.Context, client *engine.Client, status *engine.Status) []QueueItem {
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
	queue := make([]QueueItem, 0, 2)
	if status.Track != nil {
		queue = append(queue, QueueItem{Track: p.mapTrack(status.Track), Index: 0, IsCurrent: true})
	}
	if status.NextTrack != nil {
		queue = append(queue, QueueItem{Track: p.mapTrack(status.NextTrack), Index: len(queue), IsNext: true})
	}
	return queue
}

func (p *EngineProvider) queueFromContext(tracks engine.ContextTracks, currentURI string) []QueueItem {
	items := make([]QueueItem, 0, len(tracks.Items))
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
		queueItem := QueueItem{
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

func (p *EngineProvider) context(ctx context.Context, client *engine.Client, uri string) engine.ContextTracks {
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

func (p *EngineProvider) mapTrack(track *engine.Track) Track {
	if track == nil {
		return Track{Artists: []string{}}
	}
	cover := ""
	if track.AlbumCoverURL != nil && p.art != nil {
		if key := p.art.Register(*track.AlbumCoverURL); key != "" {
			cover = "/art/" + key
		}
	}
	return Track{
		ID:         track.URI,
		Title:      track.Name,
		Artists:    append([]string(nil), track.ArtistNames...),
		Album:      track.AlbumName,
		Cover:      cover,
		URI:        track.URI,
		DurationMS: track.Duration,
	}
}

func unavailableState(message string) PlayerState {
	return PlayerState{
		Provider: providerName,
		Status:   StatusStopped,
		Track:    Track{Artists: []string{}},
		Devices:  []Device{},
		Queue:    []QueueItem{},
		Error: &PlayerError{
			Code:    "audio_unavailable",
			Message: message,
		},
	}
}
