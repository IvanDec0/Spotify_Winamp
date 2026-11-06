package playback

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"cadence/internal/artwork"
	"cadence/internal/soloist"
)

const (
	soloistProviderName = "soloist"
	soloistDeviceID     = "soloist"
	soloistDefaultName  = "Cadence"

	soloistCommandTimeout = 5 * time.Second
	soloistQueryTimeout   = 5 * time.Second

	// soloistSeekTolerance is how close the reported position must be to the
	// requested one before a seek is considered applied.
	soloistSeekTolerance = 3000
	// soloistSeekAttempts bounds how many times a dropped seek is resent.
	soloistSeekAttempts = 3
)

// The seek retry window is tunable so tests stay fast without changing the
// production behavior: Soloist may drop a seek sent while an item is loading.
var (
	soloistSeekPollInterval = 500 * time.Millisecond
	soloistSeekPollTimeout  = 4 * time.Second
)

// soloistCoverSizes is the preferred order for picking a cover URL.
var soloistCoverSizes = []string{"xlarge", "large", "default", "small"}

// soloistBackendErrorCodes are the process-level failures that are still worth
// reporting once the WebSocket query succeeds.
var soloistBackendErrorCodes = map[string]bool{
	soloist.CodePairingRequired: true,
	soloist.CodeAPIKeyMissing:   true,
	soloist.CodeAPIKeyInvalid:   true,
	soloist.CodeAPIKeyRejected:  true,
	soloist.CodeNotLoggedIn:     true,
	soloist.CodeBuildExpired:    true,
	soloist.CodeStartFailed:     true,
	soloist.CodeNotInstalled:    true,
}

// SoloistBackend is the subset of soloist.Service the provider needs.
type SoloistBackend interface {
	Client() *soloist.Client
	State() soloist.State
}

// SoloistProvider maps Spotify Soloist's local WebSocket protocol to the
// provider contract. It implements Provider and Streamer.
type SoloistProvider struct {
	backend SoloistBackend
	art     *artwork.Cache

	streamMu   sync.Mutex
	revision   uint64
	lastState  PlayerState
	hasState   bool
	appliedSeq uint64
	fetchSeq   atomic.Uint64
}

var (
	_ Provider = (*SoloistProvider)(nil)
	_ Streamer = (*SoloistProvider)(nil)
)

// NewSoloistProvider returns a provider backed by Soloist. A nil art cache
// leaves cover keys empty.
func NewSoloistProvider(backend SoloistBackend, art *artwork.Cache) *SoloistProvider {
	return &SoloistProvider{backend: backend, art: art}
}

// State returns the current Soloist playback state.
func (p *SoloistProvider) State(ctx context.Context) (PlayerState, error) {
	state, seq, err := p.fetch(ctx)
	if err != nil {
		return PlayerState{}, err
	}
	return p.publish(state, seq, nil), nil
}

// Queue returns the current item followed by the upcoming items. It queries
// the state for the current item because the queue event carries only the
// upcoming entries.
func (p *SoloistProvider) Queue(ctx context.Context) ([]QueueItem, error) {
	client := p.backend.Client()
	if client == nil {
		return nil, errSoloistNotRunning()
	}
	stateEvent, err := client.Do(ctx, soloist.GetState(), "playback_state", soloistQueryTimeout)
	if err != nil {
		return nil, err
	}
	queueEvent, err := client.Do(ctx, soloist.GetQueue(50), "queue_changed", soloistQueryTimeout)
	if err != nil {
		return nil, err
	}

	var current *soloist.Entity
	if stateEvent.Playback != nil {
		current = stateEvent.Playback.Item
	}
	currentURI := ""
	if current != nil {
		currentURI = current.URI
	}

	items := make([]QueueItem, 0, 50)
	if current != nil {
		items = append(items, QueueItem{Track: p.mapEntity(current), Index: 0, IsCurrent: true})
	}
	var upcoming []soloist.QueueEntry
	if queueEvent.Queue != nil {
		upcoming = queueEvent.Queue.Upcoming
	}
	nextMarked := false
	for _, entry := range upcoming {
		if len(items) >= 50 {
			break
		}
		if entry.Item == nil {
			continue
		}
		if currentURI != "" && entry.Item.URI == currentURI {
			continue
		}
		item := QueueItem{Track: p.mapEntity(entry.Item), Index: len(items)}
		if !nextMarked {
			item.IsNext = true
			nextMarked = true
		}
		items = append(items, item)
	}
	return items, nil
}

// Play activates Soloist and resumes the current item.
func (p *SoloistProvider) Play(ctx context.Context) error {
	client := p.backend.Client()
	if client == nil {
		return errSoloistNotRunning()
	}
	if _, err := client.Do(ctx, soloist.Activate(), "command_result", soloistCommandTimeout); err != nil {
		return err
	}
	_, err := client.Do(ctx, soloist.Play(""), "command_result", soloistCommandTimeout)
	return err
}

// PlayContext activates Soloist and starts uri. Soloist's play has no offset,
// so a skipToURI plays that track standalone and loses its parent context.
func (p *SoloistProvider) PlayContext(ctx context.Context, uri, skipToURI string) error {
	uri = strings.TrimSpace(uri)
	skipToURI = strings.TrimSpace(skipToURI)
	if uri == "" {
		return &soloist.Error{
			Code:    soloist.CodeUnsupportedAction,
			Message: "the playback context is empty",
		}
	}
	client := p.backend.Client()
	if client == nil {
		return errSoloistNotRunning()
	}
	if _, err := client.Do(ctx, soloist.Activate(), "command_result", soloistCommandTimeout); err != nil {
		return err
	}
	target := uri
	if skipToURI != "" {
		// Soloist cannot start a context at an offset. Playing the track
		// standalone is the closest available behavior, at the cost of the
		// surrounding album, playlist or artist context.
		target = skipToURI
	}
	_, err := client.Do(ctx, soloist.Play(target), "command_result", soloistCommandTimeout)
	return err
}

// PlayTrack starts a single track URI.
func (p *SoloistProvider) PlayTrack(ctx context.Context, uri string) error {
	return p.PlayContext(ctx, uri, "")
}

// AddToQueue appends a track URI to Soloist's queue. Soloist accepts only
// tracks, so other entity types are rejected before any command is sent.
func (p *SoloistProvider) AddToQueue(ctx context.Context, uri string) error {
	uri = strings.TrimSpace(uri)
	if !strings.HasPrefix(strings.ToLower(uri), "spotify:track:") {
		return &soloist.Error{
			Code:    soloist.CodeUnsupportedAction,
			Message: "Only Spotify tracks can be added to the queue.",
		}
	}
	return p.command(ctx, soloist.AddToQueue(uri))
}

// StartRadio is not available with Soloist: go-librespot resolves radio through
// its private streaming session, which Soloist cannot reuse.
func (p *SoloistProvider) StartRadio(ctx context.Context, uri string) error {
	return &soloist.Error{
		Code:    soloist.CodeUnsupportedAction,
		Message: "Spotify radio is not available with the Soloist backend.",
	}
}

// Pause pauses playback.
func (p *SoloistProvider) Pause(ctx context.Context) error {
	return p.command(ctx, soloist.Pause())
}

// Toggle pauses a playing item and resumes a paused or stopped one.
func (p *SoloistProvider) Toggle(ctx context.Context) error {
	client := p.backend.Client()
	if client == nil {
		return errSoloistNotRunning()
	}
	event, err := client.Do(ctx, soloist.GetState(), "playback_state", soloistQueryTimeout)
	if err != nil {
		return err
	}
	if event.Playback != nil {
		switch strings.ToLower(strings.TrimSpace(event.Playback.Status)) {
		case "playing", "buffering":
			return p.command(ctx, soloist.Pause())
		}
	}
	if _, err := client.Do(ctx, soloist.Activate(), "command_result", soloistCommandTimeout); err != nil {
		return err
	}
	_, err = client.Do(ctx, soloist.Play(""), "command_result", soloistCommandTimeout)
	return err
}

// Next skips to the next item.
func (p *SoloistProvider) Next(ctx context.Context) error {
	return p.command(ctx, soloist.SkipNext())
}

// Previous skips to the previous item.
func (p *SoloistProvider) Previous(ctx context.Context) error {
	return p.command(ctx, soloist.SkipPrev())
}

// Seek moves playback to positionMS. Soloist may drop a seek sent while the
// item is still loading, so the command is resent a bounded number of times
// until the reported position converges.
func (p *SoloistProvider) Seek(ctx context.Context, positionMS int) error {
	client := p.backend.Client()
	if client == nil {
		return errSoloistNotRunning()
	}
	target := positionMS
	if target < 0 {
		target = 0
	}
	p.streamMu.Lock()
	if p.hasState && p.lastState.DurationMS > 0 && target > p.lastState.DurationMS {
		target = p.lastState.DurationMS
	}
	p.streamMu.Unlock()

	for attempt := 0; attempt < soloistSeekAttempts; attempt++ {
		if _, err := client.Do(ctx, soloist.Seek(int64(target)), "command_result", soloistCommandTimeout); err != nil {
			return err
		}
		converged, err := p.seekConverged(ctx, client, target)
		if err != nil {
			return err
		}
		if converged {
			return nil
		}
	}
	// The command was accepted but the position never converged; the stream
	// reconciles from the next state event instead of failing the action.
	return nil
}

// seekConverged polls the state for a short window and reports whether the
// seek was applied. An unverifiable poll stops the retry loop without failing
// the action.
func (p *SoloistProvider) seekConverged(ctx context.Context, client *soloist.Client, target int) (bool, error) {
	deadline := time.Now().Add(soloistSeekPollTimeout)
	for {
		if !time.Now().Before(deadline) {
			return false, nil
		}
		event, err := client.Do(ctx, soloist.GetState(), "playback_state", soloistQueryTimeout)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return false, ctxErr
			}
			return true, nil
		}
		if event.Playback != nil && event.Playback.Position != nil {
			position := int(event.Playback.Position.PositionMS)
			if absInt(position-target) <= soloistSeekTolerance {
				return true, nil
			}
			status := strings.ToLower(strings.TrimSpace(event.Playback.Status))
			if status == "playing" && position >= target-1000 {
				return true, nil
			}
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(soloistSeekPollInterval):
		}
	}
}

// SetVolume sets Soloist's volume, clamped to the protocol range.
func (p *SoloistProvider) SetVolume(ctx context.Context, volume int) error {
	return p.command(ctx, soloist.SetVolume(clamp(volume, 0, 100)))
}

// SetShuffle enables or disables shuffle.
func (p *SoloistProvider) SetShuffle(ctx context.Context, enabled bool) error {
	return p.command(ctx, soloist.SetShuffle(enabled))
}

// SetRepeat applies mode with Soloist's two independent repeat flags.
func (p *SoloistProvider) SetRepeat(ctx context.Context, mode RepeatMode) error {
	commands := soloist.RepeatCommands(string(mode))
	if len(commands) == 0 {
		return &soloist.Error{
			Code:    soloist.CodeUnsupportedAction,
			Message: fmt.Sprintf("unsupported repeat mode %q", mode),
		}
	}
	for _, cmd := range commands {
		if err := p.command(ctx, cmd); err != nil {
			return err
		}
	}
	return nil
}

// Transfer activates the local Soloist device. Other Connect devices are not
// supported while Soloist owns audio playback.
func (p *SoloistProvider) Transfer(ctx context.Context, deviceID string) error {
	id := strings.TrimSpace(deviceID)
	if id == "" || id == soloistDeviceID {
		return p.command(ctx, soloist.Activate())
	}
	return &soloist.Error{
		Code:    soloist.CodeUnsupportedAction,
		Message: "Cadence plays through the local Spotify Soloist device",
	}
}

// Stream publishes the initial state, then refreshes only when Soloist sends
// an event and reconciles every 15 seconds. It returns when ctx is cancelled
// or the connection ends; callers reconnect with backoff.
func (p *SoloistProvider) Stream(ctx context.Context, emit func(PlayerState)) error {
	client := p.backend.Client()
	if client == nil {
		p.publish(soloistUnavailableState("soloist_not_running", "Spotify Soloist is not running."), p.fetchSeq.Add(1), emit)
		return errSoloistNotRunning()
	}

	// Watch before the initial fetch: events that land while it is in flight
	// set the dirty flag and are refreshed on the next coalescing tick.
	watchCtx, stopWatch := context.WithCancel(ctx)
	defer stopWatch()
	events, unsubscribe := client.Subscribe()
	defer unsubscribe()
	var dirty atomic.Bool
	go func() {
		for {
			select {
			case <-watchCtx.Done():
				return
			case <-client.Done():
				return
			case <-events:
				dirty.Store(true)
			}
		}
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
		case <-client.Done():
			if err := client.Err(); err != nil {
				return err
			}
			return errors.New("the Spotify Soloist connection closed")
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

func (p *SoloistProvider) refresh(ctx context.Context, emit func(PlayerState)) (PlayerState, error) {
	state, seq, err := p.fetch(ctx)
	if err != nil {
		return PlayerState{}, err
	}
	return p.publish(state, seq, emit), nil
}

// publish stamps a new revision when the state changed and calls emit only
// then. A slow fetch that arrives after a newer capture was applied is
// discarded instead of reviving stale state. Unchanged states keep the
// previous revision and capture timestamp.
func (p *SoloistProvider) publish(state PlayerState, seq uint64, emit func(PlayerState)) PlayerState {
	p.streamMu.Lock()
	if seq < p.appliedSeq {
		latest := clonePlayerState(p.lastState)
		p.streamMu.Unlock()
		return latest
	}
	p.appliedSeq = seq
	if p.hasState && sameStreamedState(p.lastState, state) {
		state.Revision = p.revision
		state.CapturedAt = p.lastState.CapturedAt
		unchanged := clonePlayerState(state)
		p.streamMu.Unlock()
		return unchanged
	}
	p.revision++
	state.Revision = p.revision
	state.CapturedAt = time.Now().UnixMilli()
	p.lastState = clonePlayerState(state)
	p.hasState = true
	published := clonePlayerState(state)
	p.streamMu.Unlock()
	if emit != nil {
		emit(published)
	}
	return published
}

func (p *SoloistProvider) fetch(ctx context.Context) (PlayerState, uint64, error) {
	seq := p.fetchSeq.Add(1)
	backendState := p.backend.State()
	client := p.backend.Client()
	if client == nil {
		return soloistUnavailableState("soloist_not_running", "Spotify Soloist is not running."), seq, nil
	}
	if backendState.NeedsPairing || !backendState.LoggedIn {
		return soloistPairingState(), seq, nil
	}
	event, err := client.Do(ctx, soloist.GetState(), "playback_state", soloistQueryTimeout)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return PlayerState{}, seq, ctxErr
		}
		code := soloist.CodeOf(err)
		if code == "" {
			code = soloist.CodeProtocol
		}
		return soloistUnavailableState(code, err.Error()), seq, nil
	}
	playback := &soloist.PlaybackState{}
	if event.Playback != nil {
		playback = event.Playback
	}
	state := p.mapPlayback(playback, backendState)
	if failure := soloistBackendFailure(backendState); failure != nil {
		state.Error = failure
	}
	return state, seq, nil
}

// command sends one control command and waits for Soloist to accept it.
func (p *SoloistProvider) command(ctx context.Context, cmd soloist.Command) error {
	client := p.backend.Client()
	if client == nil {
		return errSoloistNotRunning()
	}
	_, err := client.Do(ctx, cmd, "command_result", soloistCommandTimeout)
	return err
}

func (p *SoloistProvider) mapPlayback(playback *soloist.PlaybackState, backendState soloist.State) PlayerState {
	state := PlayerState{
		Provider: soloistProviderName,
		Status:   StatusStopped,
		Track:    Track{Artists: []string{}},
		Devices:  []Device{},
		Queue:    []QueueItem{},
		Repeat:   RepeatOff,
	}
	switch strings.ToLower(strings.TrimSpace(playback.Status)) {
	case "playing", "buffering":
		state.Status = StatusPlaying
	case "paused":
		state.Status = StatusPaused
	}
	if playback.Item != nil {
		state.Track = p.mapEntity(playback.Item)
		state.DurationMS = state.Track.DurationMS
	}
	if playback.Position != nil {
		progress := int(playback.Position.PositionMS)
		if progress < 0 {
			progress = 0
		}
		if state.DurationMS > 0 && progress > state.DurationMS {
			progress = state.DurationMS
		}
		state.ProgressMS = progress
	}
	state.Volume = clamp(playback.Volume, 0, 100)
	state.Shuffle = playback.Options.Shuffle
	switch strings.ToLower(strings.TrimSpace(playback.Options.Repeat)) {
	case "track":
		state.Repeat = RepeatTrack
	case "context":
		state.Repeat = RepeatContext
	}
	name := strings.TrimSpace(backendState.DeviceName)
	if name == "" {
		name = soloistDefaultName
	}
	device := Device{
		ID:            soloistDeviceID,
		Name:          name,
		Type:          "speaker",
		Active:        playback.IsActive,
		VolumePercent: state.Volume,
	}
	state.Device = device
	state.Devices = []Device{device}
	state.CanControl = backendState.LoggedIn && playback.Item != nil
	state.CanSeek = playback.Item != nil && state.DurationMS > 0
	return state
}

// mapEntity converts a Soloist entity envelope into the provider-neutral track.
func (p *SoloistProvider) mapEntity(entity *soloist.Entity) Track {
	track := Track{Artists: []string{}}
	if entity == nil {
		return track
	}
	track.ID = entity.URI
	track.URI = entity.URI
	track.Title = entity.Decorations.Identity.Name
	for _, creator := range entity.Decorations.Creators {
		if creator.Entity == nil {
			continue
		}
		name := creator.Entity.Decorations.Identity.Name
		if name == "" {
			continue
		}
		track.Artists = append(track.Artists, name)
	}
	if parent := entity.Decorations.Parent; parent != nil && parent.Entity != nil {
		switch strings.ToLower(parent.Entity.EntityType) {
		case "album", "show":
			track.Album = parent.Entity.Decorations.Identity.Name
		}
	}
	if playback := entity.Decorations.Playback; playback != nil {
		track.DurationMS = int(playback.DurationMS)
	}
	track.Cover = p.cover(entity.Decorations.VisualIdentity.Cover)
	return track
}

// cover registers the best available cover URL and returns its local key.
func (p *SoloistProvider) cover(covers []soloist.Cover) string {
	if p.art == nil || len(covers) == 0 {
		return ""
	}
	for _, size := range soloistCoverSizes {
		for _, cover := range covers {
			if !strings.EqualFold(cover.Size, size) || cover.URL == "" {
				continue
			}
			if key := p.art.Register(cover.URL); key != "" {
				return "/art/" + key
			}
		}
	}
	for _, cover := range covers {
		if cover.URL == "" {
			continue
		}
		if key := p.art.Register(cover.URL); key != "" {
			return "/art/" + key
		}
	}
	return ""
}

func soloistUnavailableState(code, message string) PlayerState {
	return PlayerState{
		Provider: soloistProviderName,
		Status:   StatusStopped,
		Track:    Track{Artists: []string{}},
		Devices:  []Device{},
		Queue:    []QueueItem{},
		Error: &PlayerError{
			Code:    code,
			Message: message,
		},
	}
}

func soloistPairingState() PlayerState {
	return PlayerState{
		Provider: soloistProviderName,
		Status:   StatusStopped,
		Track:    Track{Artists: []string{}},
		Devices:  []Device{},
		Queue:    []QueueItem{},
		Error: &PlayerError{
			Code:    soloist.CodePairingRequired,
			Message: "Open Spotify on your phone and select Cadence once.",
		},
	}
}

// soloistBackendFailure reports a process-level failure that coexists with a
// successful WebSocket query, without ever exposing a secret.
func soloistBackendFailure(state soloist.State) *PlayerError {
	if state.ErrorCode == "" || !soloistBackendErrorCodes[state.ErrorCode] {
		return nil
	}
	message := strings.TrimSpace(state.Error)
	if message == "" {
		message = "Spotify Soloist reported a problem."
	}
	return &PlayerError{Code: state.ErrorCode, Message: message}
}

func errSoloistNotRunning() error {
	return errors.New("Spotify Soloist is not running")
}

// clonePlayerState copies one state so callers cannot mutate the provider's
// stored snapshot.
func clonePlayerState(state PlayerState) PlayerState {
	clone := state
	clone.Track.Artists = append([]string{}, state.Track.Artists...)
	clone.Devices = append([]Device{}, state.Devices...)
	clone.Queue = append([]QueueItem{}, state.Queue...)
	for index := range clone.Queue {
		clone.Queue[index].Track.Artists = append([]string{}, state.Queue[index].Track.Artists...)
	}
	if state.Error != nil {
		failure := *state.Error
		clone.Error = &failure
	}
	return clone
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
