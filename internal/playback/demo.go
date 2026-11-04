package playback

import (
	"context"
	"sync"
	"time"
)

const demoProviderName = "demo"

// DemoProvider is a deterministic, dependency-free provider used by the demo
// player and the frontend test fixtures. It simulates elapsed playback time so
// the UI can be developed without a Spotify account.
type DemoProvider struct {
	mu             sync.Mutex
	now            func() time.Time
	tracks         []Track
	devices        []Device
	current        int
	status         Status
	basePositionMS int
	lastTick       time.Time
	volume         int
	shuffle        bool
	repeat         RepeatMode
	activeDevice   int
	revision       uint64
}

// NewDemoProvider returns a demo provider that starts mid-way through the first
// track with a small fictional queue.
func NewDemoProvider() *DemoProvider {
	return newDemoProvider(time.Now)
}

func newDemoProvider(now func() time.Time) *DemoProvider {
	devices := []Device{
		{ID: "studio", Name: "Studio monitors", Type: "speaker", VolumePercent: 64},
		{ID: "living-room", Name: "Living room", Type: "speaker", VolumePercent: 38},
	}
	devices[0].Active = true

	return &DemoProvider{
		now:            now,
		tracks:         demoTracks(),
		devices:        devices,
		status:         StatusPlaying,
		basePositionMS: 38_000,
		lastTick:       now(),
		volume:         64,
		repeat:         RepeatOff,
	}
}

func (p *DemoProvider) State(ctx context.Context) (PlayerState, error) {
	if err := ctx.Err(); err != nil {
		return PlayerState{}, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.advanceLocked()

	return p.stateLocked(), nil
}

// Queue returns a copy of the demo queue.
func (p *DemoProvider) Queue(ctx context.Context) ([]QueueItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	return p.queueLocked(), nil
}

func (p *DemoProvider) Play(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.status == StatusPlaying {
		return nil
	}
	p.status = StatusPlaying
	p.lastTick = p.now()
	p.revision++
	return nil
}

func (p *DemoProvider) Pause(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.status == StatusPaused {
		return nil
	}
	p.basePositionMS = p.positionLocked()
	p.status = StatusPaused
	p.revision++
	return nil
}

func (p *DemoProvider) Toggle(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.status == StatusPlaying {
		p.basePositionMS = p.positionLocked()
		p.status = StatusPaused
	} else {
		p.status = StatusPlaying
		p.lastTick = p.now()
	}
	p.revision++
	return nil
}

func (p *DemoProvider) Next(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.current = (p.current + 1) % len(p.tracks)
	p.basePositionMS = 0
	p.lastTick = p.now()
	if p.status == StatusStopped {
		p.status = StatusPaused
	}
	p.revision++
	return nil
}

func (p *DemoProvider) Previous(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.positionLocked() > 3_000 {
		p.basePositionMS = 0
	} else {
		p.current = (p.current - 1 + len(p.tracks)) % len(p.tracks)
		p.basePositionMS = 0
	}
	p.lastTick = p.now()
	if p.status == StatusStopped {
		p.status = StatusPaused
	}
	p.revision++
	return nil
}

func (p *DemoProvider) Seek(ctx context.Context, positionMS int) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	duration := p.tracks[p.current].DurationMS
	p.basePositionMS = clamp(positionMS, 0, duration)
	p.lastTick = p.now()
	p.revision++
	return nil
}

func (p *DemoProvider) SetVolume(ctx context.Context, volume int) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.volume = clamp(volume, 0, 100)
	p.devices[p.activeDevice].VolumePercent = p.volume
	p.revision++
	return nil
}

func (p *DemoProvider) SetShuffle(ctx context.Context, enabled bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.shuffle = enabled
	p.revision++
	return nil
}

func (p *DemoProvider) SetRepeat(ctx context.Context, mode RepeatMode) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	switch mode {
	case RepeatOff, RepeatContext, RepeatTrack:
	default:
		mode = RepeatOff
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.repeat = mode
	p.revision++
	return nil
}

func (p *DemoProvider) Transfer(ctx context.Context, deviceID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.devices {
		if p.devices[i].ID == deviceID {
			p.activeDevice = i
			p.volume = p.devices[i].VolumePercent
			p.revision++
			break
		}
	}
	return nil
}

// advanceLocked moves the simulated playhead across track boundaries. It must
// be called with p.mu held.
func (p *DemoProvider) advanceLocked() {
	if p.status != StatusPlaying {
		return
	}

	now := p.now()
	elapsed := now.Sub(p.lastTick)
	if elapsed < 0 {
		elapsed = 0
	}
	position := p.basePositionMS + int(elapsed/time.Millisecond)

	for {
		duration := p.tracks[p.current].DurationMS
		if duration <= 0 || position < duration {
			break
		}
		position -= duration

		if p.repeat == RepeatTrack {
			break
		}

		next := p.current + 1
		if next >= len(p.tracks) {
			if p.repeat == RepeatContext {
				next = 0
			} else {
				p.status = StatusPaused
				p.basePositionMS = duration
				p.lastTick = now
				p.revision++
				return
			}
		}

		p.current = next
		p.revision++
	}

	p.basePositionMS = position
	p.lastTick = now
}

func (p *DemoProvider) positionLocked() int {
	if p.status != StatusPlaying {
		return p.basePositionMS
	}

	elapsed := p.now().Sub(p.lastTick)
	if elapsed < 0 {
		elapsed = 0
	}
	position := p.basePositionMS + int(elapsed/time.Millisecond)
	duration := p.tracks[p.current].DurationMS
	if position > duration {
		position = duration
	}
	return position
}

func (p *DemoProvider) stateLocked() PlayerState {
	track := p.tracks[p.current]
	devices := p.devicesLocked()
	device := devices[p.activeDevice]

	return PlayerState{
		Provider:   demoProviderName,
		Status:     p.status,
		Track:      cloneTrack(track),
		ProgressMS: p.positionLocked(),
		DurationMS: track.DurationMS,
		Volume:     p.volume,
		Shuffle:    p.shuffle,
		Repeat:     p.repeat,
		Device:     device,
		Devices:    devices,
		Queue:      p.queueLocked(),
		CanSeek:    true,
		CanControl: true,
		Revision:   p.revision,
	}
}

func (p *DemoProvider) devicesLocked() []Device {
	devices := make([]Device, len(p.devices))
	copy(devices, p.devices)
	for i := range devices {
		devices[i].Active = i == p.activeDevice
		if devices[i].Active {
			devices[i].VolumePercent = p.volume
		}
	}
	return devices
}

func (p *DemoProvider) queueLocked() []QueueItem {
	queue := make([]QueueItem, 0, len(p.tracks))
	for i := range p.tracks {
		index := (p.current + i) % len(p.tracks)
		queue = append(queue, QueueItem{
			Track:     cloneTrack(p.tracks[index]),
			Index:     i,
			IsCurrent: i == 0,
			IsNext:    i == 1,
		})
	}
	return queue
}

func cloneTrack(track Track) Track {
	clone := track
	clone.Artists = append([]string(nil), track.Artists...)
	return clone
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

func demoTracks() []Track {
	return []Track{
		{
			ID:         "neon-meridian",
			Title:      "Neon Meridian",
			Artists:    []string{"Aster Vale"},
			Album:      "Night Cartography",
			Cover:      "meridian",
			DurationMS: 252_000,
		},
		{
			ID:         "glass-tides",
			Title:      "Glass Tides",
			Artists:    []string{"Lumen Field"},
			Album:      "Soft Machines",
			Cover:      "tides",
			DurationMS: 228_000,
		},
		{
			ID:         "slow-orbit",
			Title:      "Slow Orbit",
			Artists:    []string{"Kite Assembly"},
			Album:      "Perihelion",
			Cover:      "orbit",
			DurationMS: 306_000,
		},
		{
			ID:         "afterimage",
			Title:      "Afterimage",
			Artists:    []string{"Sable Coast"},
			Album:      "Chromatic Drift",
			Cover:      "afterimage",
			DurationMS: 215_000,
		},
		{
			ID:         "signal-bloom",
			Title:      "Signal Bloom",
			Artists:    []string{"Vera Static"},
			Album:      "Quiet Voltage",
			Cover:      "bloom",
			DurationMS: 266_000,
		},
		{
			ID:         "night-bus",
			Title:      "Night Bus",
			Artists:    []string{"The Long Return"},
			Album:      "City in Amber",
			Cover:      "nightbus",
			DurationMS: 238_000,
		},
	}
}
