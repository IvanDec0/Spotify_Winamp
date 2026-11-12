package webplayer

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
	"cadence/internal/catalog"
	"cadence/internal/playback"
)

const (
	providerName = "web"

	defaultWebDeviceName = "Cadence Web"
	webDeviceType        = "computer"
)

// Tunable polling intervals. They are variables so tests stay fast without
// changing the production behavior.
var (
	webStreamPlayerInterval  = 3 * time.Second
	webStreamPausedInterval  = 5 * time.Second
	webStreamIdleInterval    = 10 * time.Second
	webStreamSDKInterval     = time.Second
	webStreamDevicesInterval = 10 * time.Second
	webStreamLoginInterval   = 2 * time.Second
	webStreamErrorInterval   = 5 * time.Second
	webDeviceRetryInterval   = 500 * time.Millisecond
)

// ProviderBackend is the subset of the web player service the provider needs.
type ProviderBackend interface {
	API() *WebAPI
	DeviceID() string
	Running() bool
	LoggedIn() bool
	RuntimeInstalled() bool
	EnsureStarted(ctx context.Context) error
}

// Provider maps the Spotify Web API and the SDK device onto the playback
// provider contract. It implements Provider, Streamer and catalog.Playback.
type Provider struct {
	backend    ProviderBackend
	art        *artwork.Cache
	deviceName string

	streamMu   sync.Mutex
	revision   uint64
	lastState  playback.PlayerState
	hasState   bool
	appliedSeq uint64
	fetchSeq   atomic.Uint64
}

var (
	_ playback.Provider = (*Provider)(nil)
	_ playback.Streamer = (*Provider)(nil)
	_ catalog.Playback  = (*Provider)(nil)
)

// NewProvider returns a provider backed by the web player service. A nil art
// cache leaves cover keys empty.
func NewProvider(backend ProviderBackend, art *artwork.Cache) *Provider {
	name := defaultWebDeviceName
	if named, ok := backend.(interface{ DeviceName() string }); ok {
		if value := strings.TrimSpace(named.DeviceName()); value != "" {
			name = value
		}
	}
	return &Provider{backend: backend, art: art, deviceName: name}
}

// State returns the current Spotify playback state. Login and API failures are
// reported inside the state so the player UI can render them without treating
// a poll as a transport failure.
func (p *Provider) State(ctx context.Context) (playback.PlayerState, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if p == nil || p.backend == nil {
		return webErrorState(codedError(CodeAPI, "the Spotify Web API is not configured", nil)), nil
	}
	if !p.backend.LoggedIn() {
		state := loginRequiredState()
		return p.publish(state, p.fetchSeq.Add(1), nil), nil
	}
	api, err := p.webAPI()
	if err != nil {
		state := webErrorState(err)
		return p.publish(state, p.fetchSeq.Add(1), nil), nil
	}

	seq := p.fetchSeq.Add(1)
	player, err := api.Player(ctx)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return playback.PlayerState{}, ctxErr
		}
		return p.publish(webErrorState(err), seq, nil), nil
	}
	devices, err := api.Devices(ctx)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return playback.PlayerState{}, ctxErr
		}
		return p.publish(webErrorState(err), seq, nil), nil
	}
	return p.publish(p.mapState(player, devices), seq, nil), nil
}

// Stream polls the Web API until ctx is cancelled. It never starts the helper:
// only Play, PlayContext and Transfer(empty) do. While logged out it publishes
// the login state and re-checks instead of returning.
func (p *Provider) Stream(ctx context.Context, emit func(playback.PlayerState)) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if p == nil || p.backend == nil {
		return codedError(CodeAPI, "the Spotify Web API is not configured", nil)
	}
	var devices []apiDevice
	var devicesAt time.Time
	haveDevices := false

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !p.backend.LoggedIn() {
			haveDevices = false
			devices = nil
			p.publish(loginRequiredState(), p.fetchSeq.Add(1), emit)
			if err := sleepContext(ctx, webStreamLoginInterval); err != nil {
				return err
			}
			continue
		}
		api, err := p.webAPI()
		if err != nil {
			p.publish(webErrorState(err), p.fetchSeq.Add(1), emit)
			if err := sleepContext(ctx, webStreamErrorInterval); err != nil {
				return err
			}
			continue
		}

		if !haveDevices || time.Since(devicesAt) >= webStreamDevicesInterval {
			fetched, err := api.Devices(ctx)
			if err != nil {
				if ctxErr := ctx.Err(); ctxErr != nil {
					return ctxErr
				}
				p.publish(webErrorState(err), p.fetchSeq.Add(1), emit)
				if err := sleepContext(ctx, webStreamErrorInterval); err != nil {
					return err
				}
				continue
			}
			devices = fetched
			devicesAt = time.Now()
			haveDevices = true
		}

		// Prefer the SDK push while its device is the active Spotify Connect
		// target. The SDK emits only state changes, so the position is
		// interpolated from the snapshot timestamp.
		if snapshot, at, ok := p.sdkState(); ok &&
			sdkDeviceActive(devices, p.backend.DeviceID()) {
			state := p.mapSDKState(snapshot, at, devices)
			p.publish(state, p.fetchSeq.Add(1), emit)
			if err := sleepContext(ctx, webStreamSDKInterval); err != nil {
				return err
			}
			continue
		}

		seq := p.fetchSeq.Add(1)
		player, err := api.Player(ctx)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			p.publish(webErrorState(err), seq, emit)
			if err := sleepContext(ctx, webStreamErrorInterval); err != nil {
				return err
			}
			continue
		}
		state := p.mapState(player, devices)
		p.publish(state, seq, emit)
		interval := webStreamPlayerInterval
		switch state.Status {
		case playback.StatusPaused:
			interval = webStreamPausedInterval
		case playback.StatusStopped:
			interval = webStreamIdleInterval
		}
		if err := sleepContext(ctx, interval); err != nil {
			return err
		}
	}
}

// sdkState returns the last push snapshot when the backend supports it.
func (p *Provider) sdkState() (SDKState, time.Time, bool) {
	provider, ok := p.backend.(SDKStateProvider)
	if !ok {
		return SDKState{}, time.Time{}, false
	}
	return provider.SDKState()
}

func sdkDeviceActive(devices []apiDevice, deviceID string) bool {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return false
	}
	for _, device := range devices {
		if device.ID == deviceID {
			return device.IsActive
		}
	}
	return false
}

// Queue returns the current item followed by the upcoming items, capped at 50.
func (p *Provider) Queue(ctx context.Context) ([]playback.QueueItem, error) {
	api, err := p.webAPI()
	if err != nil {
		return nil, err
	}
	queue, err := api.Queue(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]playback.QueueItem, 0, 50)
	if queue == nil {
		return items, nil
	}
	if queue.CurrentlyPlaying != nil {
		items = append(items, playback.QueueItem{
			Track:     p.mapTrack(queue.CurrentlyPlaying),
			Index:     0,
			IsCurrent: true,
		})
	}
	nextMarked := false
	for index := range queue.Queue {
		if len(items) >= 50 {
			break
		}
		item := playback.QueueItem{
			Track: p.mapTrack(&queue.Queue[index]),
			Index: len(items),
		}
		if !nextMarked {
			item.IsNext = true
			nextMarked = true
		}
		items = append(items, item)
	}
	return items, nil
}

// Play ensures the SDK device is running and resumes playback.
func (p *Provider) Play(ctx context.Context) error {
	if p == nil || p.backend == nil {
		return codedError(CodeAPI, "the Spotify Web API is not configured", nil)
	}
	if !p.backend.RuntimeInstalled() {
		return ErrRuntimeNotInstalled
	}
	if err := p.backend.EnsureStarted(ctx); err != nil {
		return err
	}
	return p.playWithRetry(ctx, nil)
}

// PlayContext starts a context or a standalone track. Track URIs play as a
// one-item queue; everything else uses context_uri with an optional offset.
func (p *Provider) PlayContext(ctx context.Context, uri, skipToURI string) error {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return codedError(CodeInvalidArgument, "the playback context is empty", nil)
	}
	if p == nil || p.backend == nil {
		return codedError(CodeAPI, "the Spotify Web API is not configured", nil)
	}
	if err := p.backend.EnsureStarted(ctx); err != nil {
		return err
	}

	var body any
	if strings.HasPrefix(strings.ToLower(uri), "spotify:track:") {
		body = map[string]any{"uris": []string{uri}}
	} else {
		payload := map[string]any{"context_uri": uri}
		if skip := strings.TrimSpace(skipToURI); skip != "" {
			payload["offset"] = map[string]string{"uri": skip}
		}
		body = payload
	}
	return p.playWithRetry(ctx, body)
}

// PlayTrack starts one track URI.
func (p *Provider) PlayTrack(ctx context.Context, uri string) error {
	return p.PlayContext(ctx, uri, "")
}

// AddToQueue appends a track or episode to the queue without starting the
// helper.
func (p *Provider) AddToQueue(ctx context.Context, uri string) error {
	api, err := p.webAPI()
	if err != nil {
		return err
	}
	return api.AddToQueue(ctx, p.controlDeviceID(), uri)
}

// StartRadio is not available: Spotify exposes no public radio endpoint.
func (p *Provider) StartRadio(ctx context.Context, uri string) error {
	return codedError(CodeUnsupported, "Spotify does not expose public radio playback for the web player", nil)
}

// Pause pauses playback.
func (p *Provider) Pause(ctx context.Context) error {
	api, err := p.webAPI()
	if err != nil {
		return err
	}
	return api.Pause(ctx, p.controlDeviceID())
}

// Toggle pauses a playing item and resumes a paused or stopped one.
func (p *Provider) Toggle(ctx context.Context) error {
	api, err := p.webAPI()
	if err != nil {
		return err
	}
	player, err := api.Player(ctx)
	if err != nil {
		return err
	}
	if player != nil && player.IsPlaying {
		return p.Pause(ctx)
	}
	return p.Play(ctx)
}

// Next skips to the next item.
func (p *Provider) Next(ctx context.Context) error {
	api, err := p.webAPI()
	if err != nil {
		return err
	}
	return api.Next(ctx, p.controlDeviceID())
}

// Previous skips to the previous item.
func (p *Provider) Previous(ctx context.Context) error {
	api, err := p.webAPI()
	if err != nil {
		return err
	}
	return api.Previous(ctx, p.controlDeviceID())
}

// Seek moves playback to positionMS.
func (p *Provider) Seek(ctx context.Context, positionMS int) error {
	if positionMS < 0 {
		positionMS = 0
	}
	api, err := p.webAPI()
	if err != nil {
		return err
	}
	return api.Seek(ctx, p.controlDeviceID(), positionMS)
}

// SetVolume sets the active device volume, clamped to the API range.
func (p *Provider) SetVolume(ctx context.Context, volume int) error {
	api, err := p.webAPI()
	if err != nil {
		return err
	}
	return api.SetVolume(ctx, p.controlDeviceID(), clampInt(volume, 0, 100))
}

// SetShuffle enables or disables shuffle.
func (p *Provider) SetShuffle(ctx context.Context, enabled bool) error {
	api, err := p.webAPI()
	if err != nil {
		return err
	}
	if err := api.SetShuffle(ctx, p.controlDeviceID(), enabled); err != nil {
		return err
	}
	// The SDK push state does not carry shuffle/repeat. Keep the last known
	// value so the next pushed state does not flip the UI back to off.
	p.streamMu.Lock()
	p.lastState.Shuffle = enabled
	p.streamMu.Unlock()
	return nil
}

// SetRepeat applies off, context or track.
func (p *Provider) SetRepeat(ctx context.Context, mode playback.RepeatMode) error {
	switch mode {
	case playback.RepeatOff, playback.RepeatContext, playback.RepeatTrack:
	default:
		return codedError(CodeInvalidArgument, fmt.Sprintf("unsupported repeat mode %q", mode), nil)
	}
	api, err := p.webAPI()
	if err != nil {
		return err
	}
	if err := api.SetRepeat(ctx, p.controlDeviceID(), string(mode)); err != nil {
		return err
	}
	p.streamMu.Lock()
	p.lastState.Repeat = mode
	p.streamMu.Unlock()
	return nil
}

// Transfer activates a device. An empty id means the SDK device, which is
// started on demand; an explicit id is transferred without starting anything.
func (p *Provider) Transfer(ctx context.Context, deviceID string) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		if p == nil || p.backend == nil {
			return codedError(CodeAPI, "the Spotify Web API is not configured", nil)
		}
		if err := p.backend.EnsureStarted(ctx); err != nil {
			return err
		}
		deviceID = strings.TrimSpace(p.backend.DeviceID())
		if deviceID == "" {
			return codedError(CodeDeviceMissing, "the Cadence Web device is not ready", nil)
		}
	}
	api, err := p.webAPI()
	if err != nil {
		return err
	}
	return api.Transfer(ctx, deviceID)
}

func (p *Provider) playWithRetry(ctx context.Context, body any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	api, err := p.webAPI()
	if err != nil {
		return err
	}
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		last = api.Play(ctx, p.sdkDeviceID(), body)
		if last == nil {
			return nil
		}
		if !errors.Is(last, ErrNoActiveDevice) || attempt == 2 {
			return last
		}
		if err := sleepContext(ctx, webDeviceRetryInterval); err != nil {
			return err
		}
	}
	return last
}

func (p *Provider) webAPI() (*WebAPI, error) {
	if p == nil || p.backend == nil {
		return nil, codedError(CodeAPI, "the Spotify Web API is not configured", nil)
	}
	api := p.backend.API()
	if api == nil {
		return nil, codedError(CodeAPI, "the Spotify Web API is not configured", nil)
	}
	return api, nil
}

func (p *Provider) controlDeviceID() string {
	if p == nil || p.backend == nil || !p.backend.Running() {
		return ""
	}
	return p.backend.DeviceID()
}

func (p *Provider) sdkDeviceID() string {
	if p == nil || p.backend == nil {
		return ""
	}
	return strings.TrimSpace(p.backend.DeviceID())
}

// publish stamps a new revision when the state changed and calls emit only
// then. A slow fetch that arrives after a newer capture was applied is
// discarded instead of reviving stale state.
func (p *Provider) publish(state playback.PlayerState, seq uint64, emit func(playback.PlayerState)) playback.PlayerState {
	p.streamMu.Lock()
	if seq < p.appliedSeq {
		latest := cloneState(p.lastState)
		p.streamMu.Unlock()
		return latest
	}
	p.appliedSeq = seq
	if p.hasState && sameState(p.lastState, state) {
		state.Revision = p.revision
		state.CapturedAt = p.lastState.CapturedAt
		unchanged := cloneState(state)
		p.streamMu.Unlock()
		return unchanged
	}
	p.revision++
	state.Revision = p.revision
	state.CapturedAt = time.Now().UnixMilli()
	p.lastState = cloneState(state)
	p.hasState = true
	published := cloneState(state)
	p.streamMu.Unlock()
	if emit != nil {
		emit(published)
	}
	return published
}

// mapSDKState builds a player state from a pushed SDK snapshot. The SDK does
// not emit progress ticks, so position is snapshot.PositionMS plus the time
// elapsed since the snapshot was received.
func (p *Provider) mapSDKState(snapshot SDKState, at time.Time, devices []apiDevice) playback.PlayerState {
	state := webBaseState()
	state.Status = playback.StatusPaused
	if snapshot.IsPlaying {
		state.Status = playback.StatusPlaying
	}
	position := snapshot.PositionMS
	if snapshot.IsPlaying {
		position += time.Since(at).Milliseconds()
	}
	if position < 0 {
		position = 0
	}
	if snapshot.DurationMS > 0 && position > snapshot.DurationMS {
		position = snapshot.DurationMS
	}
	state.ProgressMS = int(position)
	state.DurationMS = int(snapshot.DurationMS)
	state.Track = playback.Track{
		URI:        snapshot.Track.URI,
		Title:      snapshot.Track.Name,
		Artists:    append([]string(nil), snapshot.Track.Artists...),
		Album:      snapshot.Track.Album,
		DurationMS: int(snapshot.Track.DurationMS),
	}
	if state.Track.DurationMS == 0 && state.DurationMS > 0 {
		state.Track.DurationMS = state.DurationMS
	}
	if p.art != nil && snapshot.Track.ImageURL != "" {
		if key := p.art.Register(snapshot.Track.ImageURL); key != "" {
			state.Track.Cover = "/art/" + key
		}
	}
	for _, device := range devices {
		state.Devices = append(state.Devices, mapDevice(device))
	}
	selected := playback.Device{
		ID:            p.backend.DeviceID(),
		Name:          p.deviceName,
		Type:          webDeviceType,
		Active:        true,
		VolumePercent: p.lastVolume(),
	}
	for _, device := range state.Devices {
		if device.ID == selected.ID {
			selected = device
			break
		}
	}
	found := false
	for _, device := range state.Devices {
		if device.ID == selected.ID {
			found = true
			break
		}
	}
	if !found {
		state.Devices = append(state.Devices, selected)
	}
	state.Device = selected
	state.Volume = selected.VolumePercent
	state.Shuffle = p.lastToggleState()
	state.Repeat = p.lastRepeatState()
	state.CanControl = p.backend.LoggedIn()
	state.CanSeek = state.Track.URI != "" && state.DurationMS > 0
	return state
}

func (p *Provider) lastVolume() int {
	p.streamMu.Lock()
	defer p.streamMu.Unlock()
	return p.lastState.Volume
}

func (p *Provider) lastToggleState() bool {
	p.streamMu.Lock()
	defer p.streamMu.Unlock()
	return p.lastState.Shuffle
}

func (p *Provider) lastRepeatState() playback.RepeatMode {
	p.streamMu.Lock()
	defer p.streamMu.Unlock()
	return p.lastState.Repeat
}

func (p *Provider) mapState(player *playerResponse, devices []apiDevice) playback.PlayerState {
	state := webBaseState()
	if player != nil {
		switch {
		case player.IsPlaying:
			state.Status = playback.StatusPlaying
		case player.Item != nil:
			state.Status = playback.StatusPaused
		}
		if player.Item != nil {
			state.Track = p.mapTrack(player.Item)
			state.DurationMS = state.Track.DurationMS
		}
		state.ProgressMS = player.ProgressMS
		if state.ProgressMS < 0 {
			state.ProgressMS = 0
		}
		if state.DurationMS > 0 && state.ProgressMS > state.DurationMS {
			state.ProgressMS = state.DurationMS
		}
		state.Shuffle = player.ShuffleState
		state.Repeat = mapRepeat(player.RepeatState)
		if player.Device != nil {
			state.Volume = clampInt(player.Device.VolumePercent, 0, 100)
		}
	}

	seen := make(map[string]bool, len(devices)+1)
	for _, device := range devices {
		state.Devices = append(state.Devices, mapDevice(device))
		seen[device.ID] = true
	}
	if player != nil && player.Device != nil && player.Device.ID != "" && !seen[player.Device.ID] {
		state.Devices = append(state.Devices, mapDevice(*player.Device))
		seen[player.Device.ID] = true
	}

	if player == nil || player.Device == nil {
		for _, device := range state.Devices {
			if device.Active {
				state.Volume = device.VolumePercent
				break
			}
		}
	}

	selected, found := p.sdkDevice(state.Devices)
	if !found {
		for _, device := range state.Devices {
			if device.Active {
				selected, found = device, true
				break
			}
		}
	}
	if !found {
		selected = playback.Device{
			Name:          p.deviceName,
			Type:          webDeviceType,
			Active:        p.backend != nil && p.backend.Running(),
			VolumePercent: state.Volume,
		}
		state.Devices = append(state.Devices, selected)
	}
	state.Device = selected
	state.CanControl = p.backend != nil && p.backend.LoggedIn()
	state.CanSeek = state.Track.URI != "" && state.DurationMS > 0
	return state
}

// sdkDevice returns the running helper's device when the API already lists it.
func (p *Provider) sdkDevice(devices []playback.Device) (playback.Device, bool) {
	if p == nil || p.backend == nil || !p.backend.Running() {
		return playback.Device{}, false
	}
	id := p.backend.DeviceID()
	if id == "" {
		return playback.Device{}, false
	}
	for _, device := range devices {
		if device.ID == id {
			return device, true
		}
	}
	return playback.Device{}, false
}

func (p *Provider) mapTrack(track *apiTrack) playback.Track {
	result := playback.Track{Artists: []string{}}
	if track == nil {
		return result
	}
	result.ID = track.ID
	result.Title = track.Name
	result.URI = track.URI
	result.DurationMS = track.DurationMS
	for _, artist := range track.Artists {
		if name := strings.TrimSpace(artist.Name); name != "" {
			result.Artists = append(result.Artists, name)
		}
	}
	if track.Album != nil {
		result.Album = track.Album.Name
		if p.art != nil {
			if raw := firstImageURL(track.Album.Images); raw != "" {
				if key := p.art.Register(raw); key != "" {
					result.Cover = "/art/" + key
				}
			}
		}
	}
	return result
}

func mapDevice(device apiDevice) playback.Device {
	return playback.Device{
		ID:            device.ID,
		Name:          device.Name,
		Type:          strings.ToLower(device.Type),
		Active:        device.IsActive,
		VolumePercent: clampInt(device.VolumePercent, 0, 100),
		Restricted:    device.IsRestricted,
	}
}

func mapRepeat(value string) playback.RepeatMode {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "track":
		return playback.RepeatTrack
	case "context":
		return playback.RepeatContext
	default:
		return playback.RepeatOff
	}
}

func webBaseState() playback.PlayerState {
	return playback.PlayerState{
		Provider: providerName,
		Status:   playback.StatusStopped,
		Track:    playback.Track{Artists: []string{}},
		Devices:  []playback.Device{},
		Queue:    []playback.QueueItem{},
		Repeat:   playback.RepeatOff,
	}
}

func loginRequiredState() playback.PlayerState {
	state := webBaseState()
	state.Error = &playback.PlayerError{
		Code:    CodeLoginRequired,
		Message: "Connect Spotify to play through the web player.",
	}
	return state
}

func webErrorState(err error) playback.PlayerState {
	state := webBaseState()
	state.Error = webPlayerError(err)
	return state
}

func webPlayerError(err error) *playback.PlayerError {
	switch {
	case errors.Is(err, ErrLoginRequired):
		return &playback.PlayerError{
			Code:    CodeLoginRequired,
			Message: "Connect Spotify to play through the web player.",
		}
	case errors.Is(err, ErrPremiumRequired):
		return &playback.PlayerError{
			Code:    CodePremium,
			Message: "Spotify Premium is required for web playback.",
		}
	case errors.Is(err, ErrNoActiveDevice):
		return &playback.PlayerError{
			Code:    CodeDeviceMissing,
			Message: "The Cadence Web device is not ready yet.",
		}
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.Status == 429:
			return &playback.PlayerError{
				Code:              CodeRateLimited,
				Message:           "Spotify is rate limiting requests. Try again shortly.",
				Retryable:         true,
				RetryAfterSeconds: apiErr.RetryAfter,
			}
		case apiErr.Status == 401:
			return &playback.PlayerError{
				Code:    CodeLoginRequired,
				Message: "Connect Spotify to play through the web player.",
			}
		case apiErr.Status >= 500:
			return &playback.PlayerError{
				Code:      CodeAPI,
				Message:   "The Spotify Web API is temporarily unavailable.",
				Retryable: true,
			}
		}
	}
	return &playback.PlayerError{
		Code:    CodeAPI,
		Message: "The Spotify Web API is unavailable.",
	}
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

func cloneState(state playback.PlayerState) playback.PlayerState {
	clone := state
	clone.Track.Artists = append([]string{}, state.Track.Artists...)
	clone.Devices = append([]playback.Device{}, state.Devices...)
	clone.Queue = append([]playback.QueueItem{}, state.Queue...)
	for index := range clone.Queue {
		clone.Queue[index].Track.Artists = append([]string{}, state.Queue[index].Track.Artists...)
	}
	if state.Error != nil {
		failure := *state.Error
		clone.Error = &failure
	}
	return clone
}

func sameState(left, right playback.PlayerState) bool {
	left.Revision, right.Revision = 0, 0
	left.CapturedAt, right.CapturedAt = 0, 0
	return reflect.DeepEqual(left, right)
}
