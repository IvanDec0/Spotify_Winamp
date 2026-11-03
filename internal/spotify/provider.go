package spotify

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"cadence/internal/artcache"
	"cadence/internal/player"
)

const providerName = "spotify"

const (
	playbackPlayingInterval = 1 * time.Second
	playbackIdleInterval    = 5 * time.Second
	devicesInterval         = 10 * time.Second
	queueInterval           = 5 * time.Second
	maxBackoff              = 30 * time.Second
)

// Provider maps the Spotify Web API onto player.Provider. It caches state so
// the UI stays responsive and API polling follows the playback cadence.
type Provider struct {
	client        *Client
	art           *artcache.Cache
	onAuthProblem func(error)
	now           func() time.Time

	mu           sync.Mutex
	refreshMu    sync.Mutex
	state        player.PlayerState
	nextPlayback time.Time
	nextDevices  time.Time
	nextQueue    time.Time
	backoff      time.Duration
	authNotified bool
}

func NewProvider(client *Client, art *artcache.Cache, onAuthProblem func(error)) *Provider {
	return &Provider{
		client:        client,
		art:           art,
		onAuthProblem: onAuthProblem,
		now:           time.Now,
		state: player.PlayerState{
			Provider: providerName,
			Devices:  []player.Device{},
			Queue:    []player.QueueItem{},
		},
	}
}

func (p *Provider) SetNow(now func() time.Time) {
	if now != nil {
		p.now = now
	}
}

func (p *Provider) State(ctx context.Context) (player.PlayerState, error) {
	if err := ctx.Err(); err != nil {
		return player.PlayerState{}, err
	}

	p.refreshMu.Lock()
	defer p.refreshMu.Unlock()

	now := p.now()
	p.mu.Lock()
	due := !now.Before(p.nextPlayback)
	p.mu.Unlock()
	if due {
		p.refresh(ctx, now)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	return cloneState(p.state), nil
}

func (p *Provider) Play(ctx context.Context) error {
	if err := p.client.Play(ctx); err != nil {
		p.actionError(err)
		return err
	}
	p.mutate(func(state *player.PlayerState) {
		state.Status = player.StatusPlaying
		state.Error = nil
	})
	return nil
}

func (p *Provider) Pause(ctx context.Context) error {
	if err := p.client.Pause(ctx); err != nil {
		p.actionError(err)
		return err
	}
	p.mutate(func(state *player.PlayerState) {
		state.Status = player.StatusPaused
		state.Error = nil
	})
	return nil
}

func (p *Provider) Toggle(ctx context.Context) error {
	p.mu.Lock()
	playing := p.state.Status == player.StatusPlaying
	p.mu.Unlock()
	if playing {
		return p.Pause(ctx)
	}
	return p.Play(ctx)
}

func (p *Provider) Next(ctx context.Context) error {
	if err := p.client.Next(ctx); err != nil {
		p.actionError(err)
		return err
	}
	p.mutate(func(state *player.PlayerState) {
		state.ProgressMS = 0
		state.Error = nil
	})
	return nil
}

func (p *Provider) Previous(ctx context.Context) error {
	if err := p.client.Previous(ctx); err != nil {
		p.actionError(err)
		return err
	}
	p.mutate(func(state *player.PlayerState) {
		state.ProgressMS = 0
		state.Error = nil
	})
	return nil
}

func (p *Provider) Seek(ctx context.Context, positionMS int) error {
	if positionMS < 0 {
		positionMS = 0
	}
	if err := p.client.Seek(ctx, positionMS); err != nil {
		p.actionError(err)
		return err
	}
	p.mutate(func(state *player.PlayerState) {
		if positionMS <= state.DurationMS {
			state.ProgressMS = positionMS
		}
		state.Error = nil
	})
	return nil
}

func (p *Provider) SetVolume(ctx context.Context, volume int) error {
	volume = clampInt(volume, 0, 100)
	if err := p.client.SetVolume(ctx, volume); err != nil {
		p.actionError(err)
		return err
	}
	p.mutate(func(state *player.PlayerState) {
		state.Volume = volume
		state.Device.VolumePercent = volume
		state.Error = nil
	})
	return nil
}

func (p *Provider) SetShuffle(ctx context.Context, enabled bool) error {
	if err := p.client.SetShuffle(ctx, enabled); err != nil {
		p.actionError(err)
		return err
	}
	p.mutate(func(state *player.PlayerState) {
		state.Shuffle = enabled
		state.Error = nil
	})
	return nil
}

func (p *Provider) SetRepeat(ctx context.Context, mode player.RepeatMode) error {
	value := string(mode)
	switch mode {
	case player.RepeatOff, player.RepeatContext, player.RepeatTrack:
	default:
		return fmt.Errorf("unsupported repeat mode %q", mode)
	}
	if err := p.client.SetRepeat(ctx, value); err != nil {
		p.actionError(err)
		return err
	}
	p.mutate(func(state *player.PlayerState) {
		state.Repeat = mode
		state.Error = nil
	})
	return nil
}

func (p *Provider) Transfer(ctx context.Context, deviceID string) error {
	if strings.TrimSpace(deviceID) == "" {
		return errors.New("a playback device is required")
	}
	if err := p.client.Transfer(ctx, deviceID); err != nil {
		p.actionError(err)
		return err
	}
	p.mutate(func(state *player.PlayerState) {
		state.Device.ID = deviceID
		state.Device.Active = true
		for index := range state.Devices {
			state.Devices[index].Active = state.Devices[index].ID == deviceID
			if state.Devices[index].ID == deviceID {
				state.Device = state.Devices[index]
				state.Volume = state.Device.VolumePercent
			}
		}
		if state.Device.ID == deviceID {
			state.CanControl = !state.Device.Restricted
		}
		state.Error = nil
	})
	return nil
}

func (p *Provider) refresh(ctx context.Context, now time.Time) {
	playback, found, playbackErr := p.client.Playback(ctx)

	p.mu.Lock()
	devicesDue := !now.Before(p.nextDevices)
	queueDue := !now.Before(p.nextQueue)
	cachedTrackID := p.state.Track.ID
	noDevices := len(p.state.Devices) == 0
	p.mu.Unlock()

	var devices []DeviceDTO
	devicesFetched := false
	var queue QueueResponse
	queueFetched := false
	forceQueue := false
	if playbackErr == nil {
		if devicesDue || (found && noDevices) {
			if fetched, err := p.client.Devices(ctx); err == nil {
				devices = fetched
				devicesFetched = true
			}
		}
		forceQueue = found && playback.Item != nil && playback.Item.ID != cachedTrackID
		if queueDue || forceQueue {
			if fetched, err := p.client.Queue(ctx); err == nil {
				queue = fetched
				queueFetched = true
			}
		}
	}

	var authProblem error
	p.mu.Lock()
	if playbackErr != nil {
		authProblem = p.applyPlaybackErrorLocked(playbackErr, now)
	} else if !found {
		p.applyNoActiveDeviceLocked(now)
	} else {
		p.applyPlaybackLocked(playback, now)
		p.backoff = 0
	}
	if devicesFetched {
		p.state.Devices = mapDevices(devices)
		p.reconcileActiveDeviceLocked()
		p.nextDevices = now.Add(devicesInterval)
	} else if devicesDue {
		p.nextDevices = now.Add(devicesInterval)
	}
	if queueFetched {
		p.state.Queue = p.mapQueue(queue)
		p.nextQueue = now.Add(queueInterval)
	} else if queueDue || forceQueue {
		p.nextQueue = now.Add(queueInterval)
	}
	notify := authProblem != nil && !p.authNotified
	if notify {
		p.authNotified = true
	}
	p.state.Revision++
	p.mu.Unlock()

	if notify && p.onAuthProblem != nil {
		p.onAuthProblem(authProblem)
	}
}

func (p *Provider) applyPlaybackLocked(playback PlaybackDTO, now time.Time) {
	state := &p.state
	state.Provider = providerName
	state.Status = player.StatusPaused
	if playback.IsPlaying {
		state.Status = player.StatusPlaying
	}
	state.Track = p.mapTrack(playback.Item)
	state.DurationMS = 0
	if playback.Item != nil {
		state.DurationMS = playback.Item.DurationMS
	}
	state.ProgressMS = clampInt(playback.ProgressMS, 0, maxInt(state.DurationMS, 0))
	state.Shuffle = playback.ShuffleState
	state.Repeat = mapRepeat(playback.RepeatState)
	state.Error = nil
	if playback.Device != nil {
		state.Device = mapDevice(playback.Device)
		state.Device.Active = true
		state.Volume = state.Device.VolumePercent
	}
	state.CanControl = state.Device.ID != "" && !state.Device.Restricted
	state.CanSeek = state.CanControl && state.DurationMS > 0
	if playback.Actions.Disallows["pausing"] && state.Status == player.StatusPlaying {
		state.CanControl = false
	}
	if playback.Actions.Disallows["resuming"] && state.Status == player.StatusPaused {
		state.CanControl = false
	}
	if playback.Actions.Disallows["seeking"] {
		state.CanSeek = false
	}
	if state.Status == player.StatusPlaying {
		p.nextPlayback = now.Add(playbackPlayingInterval)
	} else {
		p.nextPlayback = now.Add(playbackIdleInterval)
	}
}

func (p *Provider) applyNoActiveDeviceLocked(now time.Time) {
	state := &p.state
	state.Provider = providerName
	state.Status = player.StatusStopped
	state.Track = player.Track{Artists: []string{}}
	state.ProgressMS = 0
	state.DurationMS = 0
	state.Volume = 0
	state.Device = player.Device{}
	state.Queue = []player.QueueItem{}
	state.CanControl = false
	state.CanSeek = false
	state.Error = &player.PlayerError{
		Code:    "no_active_device",
		Message: "No active Spotify device. Start playback in Spotify or choose a device.",
	}
	p.backoff = 0
	p.nextPlayback = now.Add(playbackIdleInterval)
}

func (p *Provider) applyPlaybackErrorLocked(err error, now time.Time) error {
	state := &p.state
	state.Provider = providerName
	state.CanControl = false
	state.CanSeek = false

	var apiErr *APIError
	switch {
	case errors.Is(err, ErrAuthRevoked), errors.As(err, &apiErr) && apiErr.StatusCode == 401:
		state.Error = &player.PlayerError{
			Code:      "auth_expired",
			Message:   "Spotify session expired. Open the account panel to reconnect.",
			Retryable: true,
		}
		p.nextPlayback = now.Add(maxBackoff)
		return err
	case errors.Is(err, ErrNotAuthenticated):
		state.Error = &player.PlayerError{
			Code:    "not_authenticated",
			Message: "Spotify is not connected. Open the account panel to connect.",
		}
		p.nextPlayback = now.Add(maxBackoff)
		return err
	case errors.As(err, &apiErr) && apiErr.StatusCode == 429:
		seconds := int(apiErr.RetryAfter.Seconds())
		if seconds < 1 {
			seconds = 2
		}
		p.nextPlayback = now.Add(max(apiErr.RetryAfter, 2*time.Second))
		state.Error = &player.PlayerError{
			Code:              "rate_limited",
			Message:           fmt.Sprintf("Spotify rate limit. Retrying in %d seconds.", seconds),
			Retryable:         true,
			RetryAfterSeconds: seconds,
		}
	case errors.As(err, &apiErr) && apiErr.StatusCode == 404:
		p.applyNoActiveDeviceLocked(now)
		state.Error.Message = "No active Spotify device. Start playback in Spotify or choose a device."
		return nil
	case errors.As(err, &apiErr) && apiErr.StatusCode == 403:
		message := "Spotify rejected this playback action."
		if strings.Contains(strings.ToLower(apiErr.Message), "premium") {
			message = "Spotify Premium is required for playback control."
		}
		state.Error = &player.PlayerError{
			Code:    "playback_restricted",
			Message: message,
		}
		p.nextPlayback = now.Add(maxBackoff)
	case errors.As(err, &apiErr) && apiErr.StatusCode >= 500:
		p.backoff = nextBackoff(p.backoff)
		state.Error = &player.PlayerError{
			Code:      "spotify_unavailable",
			Message:   "Spotify is unavailable. Cadence will retry automatically.",
			Retryable: true,
		}
		p.nextPlayback = now.Add(jitter(p.backoff))
	default:
		if apiErr != nil && apiErr.Code == "invalid_response" {
			state.Error = &player.PlayerError{
				Code:      "invalid_response",
				Message:   "Spotify returned an invalid response. Cadence will retry.",
				Retryable: true,
			}
		} else {
			p.backoff = nextBackoff(p.backoff)
			state.Error = &player.PlayerError{
				Code:      "network",
				Message:   "Cannot reach Spotify. Check your connection; Cadence will retry.",
				Retryable: true,
			}
		}
		p.nextPlayback = now.Add(jitter(p.backoff))
	}
	if p.backoff == 0 {
		p.backoff = nextBackoff(0)
	}
	return nil
}

func (p *Provider) actionError(err error) {
	now := p.now()
	p.mu.Lock()
	authProblem := p.applyPlaybackErrorLocked(err, now)
	notify := authProblem != nil && !p.authNotified
	if notify {
		p.authNotified = true
	}
	p.state.Revision++
	p.mu.Unlock()
	if notify && p.onAuthProblem != nil {
		p.onAuthProblem(authProblem)
	}
}

func (p *Provider) mutate(update func(*player.PlayerState)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	update(&p.state)
	p.state.Revision++
	p.nextPlayback = p.now().Add(500 * time.Millisecond)
}

func (p *Provider) reconcileActiveDeviceLocked() {
	active := p.state.Device
	if active.ID == "" {
		for _, device := range p.state.Devices {
			if device.Active {
				p.state.Device = device
				p.state.Volume = device.VolumePercent
				p.state.CanControl = !device.Restricted
				break
			}
		}
		return
	}
	found := false
	for index := range p.state.Devices {
		if p.state.Devices[index].ID == active.ID {
			p.state.Devices[index].Active = true
			found = true
		} else {
			p.state.Devices[index].Active = false
		}
	}
	if !found {
		active.Active = true
		p.state.Devices = append(p.state.Devices, active)
	}
}

func (p *Provider) mapQueue(response QueueResponse) []player.QueueItem {
	items := make([]player.QueueItem, 0, len(response.Queue)+1)
	index := 0
	if response.CurrentlyPlaying != nil {
		items = append(items, player.QueueItem{
			Track:     p.mapTrack(response.CurrentlyPlaying),
			Index:     0,
			IsCurrent: true,
		})
		index = 1
	}
	nextMarked := false
	for _, track := range response.Queue {
		trackCopy := track
		items = append(items, player.QueueItem{
			Track:  p.mapTrack(&trackCopy),
			Index:  index,
			IsNext: !nextMarked,
		})
		nextMarked = true
		index++
	}
	return items
}

func (p *Provider) mapTrack(track *TrackDTO) player.Track {
	if track == nil {
		return player.Track{Artists: []string{}}
	}
	artists := make([]string, 0, len(track.Artists))
	for _, artist := range track.Artists {
		if artist.Name != "" {
			artists = append(artists, artist.Name)
		}
	}
	album := track.Album.Name
	images := track.Album.Images
	if track.Show.Name != "" {
		album = track.Show.Name
		if len(images) == 0 {
			images = track.Show.Images
		}
		if len(artists) == 0 && track.Show.Publisher != "" {
			artists = []string{track.Show.Publisher}
		}
	}
	cover := ""
	if len(images) > 0 && p.art != nil {
		if key := p.art.Register(images[0].URL); key != "" {
			cover = "/art/" + key
		}
	}
	id := track.ID
	if id == "" {
		id = track.URI
	}
	return player.Track{
		ID:         id,
		Title:      track.Name,
		Artists:    artists,
		Album:      album,
		Cover:      cover,
		URI:        track.URI,
		DurationMS: track.DurationMS,
	}
}

func mapDevices(devices []DeviceDTO) []player.Device {
	result := make([]player.Device, 0, len(devices))
	for _, device := range devices {
		result = append(result, player.Device{
			ID:            device.ID,
			Name:          device.Name,
			Type:          device.Type,
			Active:        device.IsActive,
			VolumePercent: device.VolumePercent,
			Restricted:    device.IsRestricted,
		})
	}
	return result
}

func mapDevice(device *DeviceDTO) player.Device {
	if device == nil {
		return player.Device{}
	}
	return player.Device{
		ID:            device.ID,
		Name:          device.Name,
		Type:          device.Type,
		Active:        device.IsActive,
		VolumePercent: device.VolumePercent,
		Restricted:    device.IsRestricted,
	}
}

func mapRepeat(value string) player.RepeatMode {
	switch value {
	case "context":
		return player.RepeatContext
	case "track":
		return player.RepeatTrack
	default:
		return player.RepeatOff
	}
}

func cloneState(state player.PlayerState) player.PlayerState {
	cloned := state
	cloned.Track.Artists = append([]string(nil), state.Track.Artists...)
	cloned.Devices = append([]player.Device(nil), state.Devices...)
	cloned.Queue = make([]player.QueueItem, 0, len(state.Queue))
	for _, item := range state.Queue {
		item.Track.Artists = append([]string(nil), item.Track.Artists...)
		cloned.Queue = append(cloned.Queue, item)
	}
	if state.Error != nil {
		errorCopy := *state.Error
		cloned.Error = &errorCopy
	}
	return cloned
}

func nextBackoff(current time.Duration) time.Duration {
	if current <= 0 {
		return 2 * time.Second
	}
	current *= 2
	if current > maxBackoff {
		return maxBackoff
	}
	return current
}

func jitter(duration time.Duration) time.Duration {
	if duration <= 0 {
		return time.Second
	}
	factor := 0.8 + rand.Float64()*0.4
	return time.Duration(float64(duration) * factor)
}

func clampInt(value, minimum, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
