package soloist

import (
	"encoding/json"
	"strings"
)

// Command is one message of Soloist's local WebSocket API. Construct values
// with Play, Pause and the other helpers so Type is set.
type Command struct {
	Type       string `json:"type"`
	Command    string `json:"command"`
	URI        string `json:"uri,omitempty"`
	PositionMS *int64 `json:"position_ms,omitempty"`
	Volume     *int   `json:"volume,omitempty"`
	Enabled    *bool  `json:"enabled,omitempty"`
	Limit      *int   `json:"limit,omitempty"`
}

func command(name string) Command {
	return Command{Type: "command", Command: name}
}

// Play starts a track, episode, album, playlist, artist or collection. An
// empty URI resumes the current item.
func Play(uri string) Command {
	cmd := command("play")
	cmd.URI = strings.TrimSpace(uri)
	return cmd
}

// Pause pauses playback.
func Pause() Command {
	return command("pause")
}

// SkipNext moves to the next item.
func SkipNext() Command {
	return command("skip_next")
}

// SkipPrev moves to the previous item.
func SkipPrev() Command {
	return command("skip_prev")
}

// Seek jumps to positionMS. Soloist may drop a seek sent while the item is
// still loading; callers retry while loading.
func Seek(positionMS int64) Command {
	cmd := command("seek")
	cmd.PositionMS = &positionMS
	return cmd
}

// SetVolume sets Soloist's volume percentage.
func SetVolume(volume int) Command {
	cmd := command("set_volume")
	cmd.Volume = &volume
	return cmd
}

// SetShuffle enables or disables shuffle.
func SetShuffle(enabled bool) Command {
	cmd := command("set_shuffle")
	cmd.Enabled = &enabled
	return cmd
}

// SetRepeatContext enables or disables repeat of the playing context.
func SetRepeatContext(enabled bool) Command {
	cmd := command("set_repeat_context")
	cmd.Enabled = &enabled
	return cmd
}

// SetRepeatTrack enables or disables repeat of the playing item.
func SetRepeatTrack(enabled bool) Command {
	cmd := command("set_repeat_track")
	cmd.Enabled = &enabled
	return cmd
}

// AddToQueue appends a track URI to the queue. Soloist accepts track URIs
// only.
func AddToQueue(uri string) Command {
	cmd := command("add_to_queue")
	cmd.URI = strings.TrimSpace(uri)
	return cmd
}

// Activate makes Soloist the active Spotify Connect device.
func Activate() Command {
	return command("activate")
}

// Deactivate releases the active Spotify Connect device.
func Deactivate() Command {
	return command("deactivate")
}

// GetAuthState asks for the current authentication state.
func GetAuthState() Command {
	return command("get_auth_state")
}

// GetState asks for the current playback state.
func GetState() Command {
	return command("get_state")
}

// GetQueue asks for the current queue.
func GetQueue(limit int) Command {
	cmd := command("get_queue")
	if limit > 0 {
		cmd.Limit = &limit
	}
	return cmd
}

// RepeatCommands returns the command sequence that applies mode ("off",
// "context" or "track"). Soloist applies the two flags independently, so the
// order disables the mode that no longer applies before enabling the new one.
func RepeatCommands(mode string) []Command {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "off":
		return []Command{SetRepeatTrack(false), SetRepeatContext(false)}
	case "context":
		return []Command{SetRepeatTrack(false), SetRepeatContext(true)}
	case "track":
		return []Command{SetRepeatContext(false), SetRepeatTrack(true)}
	default:
		return nil
	}
}

// Entity is Soloist's envelope for a track, episode, album, playlist or
// artist.
type Entity struct {
	URI         string            `json:"uri"`
	EntityType  string            `json:"entity_type"`
	Decorations EntityDecorations `json:"decorations"`
}

// EntityDecorations holds the metadata Soloist decorates an entity with.
type EntityDecorations struct {
	Identity       Identity        `json:"identity"`
	VisualIdentity VisualIdentity  `json:"visual_identity"`
	Parent         *ParentEntity   `json:"parent,omitempty"`
	Creators       []Creator       `json:"creators,omitempty"`
	Playback       *EntityPlayback `json:"playback,omitempty"`
}

// Identity is an entity's display name.
type Identity struct {
	Name string `json:"name"`
}

// VisualIdentity holds the covers of an entity.
type VisualIdentity struct {
	Cover []Cover `json:"cover"`
}

// Cover is one cover image at a named size.
type Cover struct {
	URL  string `json:"url"`
	Size string `json:"size"`
}

// ParentEntity wraps the album or show an item belongs to.
type ParentEntity struct {
	Entity *Entity `json:"entity"`
}

// Creator wraps an artist credit.
type Creator struct {
	Entity *Entity `json:"entity"`
}

// EntityPlayback is the playback metadata of an entity.
type EntityPlayback struct {
	DurationMS     int64    `json:"duration_ms"`
	ContentRatings []string `json:"content_ratings"`
}

// Position anchors playback at a wall-clock timestamp.
type Position struct {
	PositionMS  int64   `json:"position_ms"`
	TimestampMS int64   `json:"timestamp_ms"`
	Speed       float64 `json:"speed"`
}

// PlaybackOptions are the playback modes Soloist reports.
type PlaybackOptions struct {
	Shuffle       bool              `json:"shuffle"`
	Repeat        string            `json:"repeat"`
	PlaybackSpeed float64           `json:"playback_speed"`
	Modes         map[string]string `json:"modes,omitempty"`
}

// QueueEntry is one item in Soloist's queue.
type QueueEntry struct {
	UID    string  `json:"uid"`
	Source string  `json:"source"`
	Item   *Entity `json:"item"`
}

// QueueState is the previous and upcoming part of Soloist's queue.
type QueueState struct {
	Previous []QueueEntry `json:"previous,omitempty"`
	Upcoming []QueueEntry `json:"upcoming,omitempty"`
}

// AuthState is the payload of an auth_state event.
type AuthState struct {
	LoggedIn   bool   `json:"logged_in"`
	IsActive   bool   `json:"is_active"`
	DeviceName string `json:"device_name"`
}

// PlaybackState is the payload of a playback_state event.
type PlaybackState struct {
	Status           string                     `json:"status"`
	Item             *Entity                    `json:"item"`
	Context          *Entity                    `json:"context"`
	Position         *Position                  `json:"position"`
	Volume           int                        `json:"volume"`
	IsActive         bool                       `json:"is_active"`
	Options          PlaybackOptions            `json:"options"`
	AvailableActions map[string]json.RawMessage `json:"available_actions"`
}

// Event is one decoded Soloist WebSocket event. Fields that do not apply to
// the event's Type are zero.
type Event struct {
	Type string

	Auth     *AuthState
	Playback *PlaybackState
	Queue    *QueueState

	Options    *PlaybackOptions
	Position   *Position
	Item       *Entity
	Context    *Entity
	Status     string
	Volume     *int
	DeviceName string
	IsActive   *bool
	Command    string
	Message    string
}

// wireEvent is the flat JSON shape Soloist sends. decodeEvent folds it into
// Event.
type wireEvent struct {
	Type             string                     `json:"type"`
	LoggedIn         bool                       `json:"logged_in"`
	IsActive         *bool                      `json:"is_active"`
	DeviceName       string                     `json:"device_name"`
	Status           string                     `json:"status"`
	Item             *Entity                    `json:"item"`
	Context          *Entity                    `json:"context"`
	Position         *Position                  `json:"position"`
	Volume           *int                       `json:"volume"`
	Options          *PlaybackOptions           `json:"options"`
	AvailableActions map[string]json.RawMessage `json:"available_actions"`
	Previous         []QueueEntry               `json:"previous"`
	Upcoming         []QueueEntry               `json:"upcoming"`
	Queue            *QueueState                `json:"queue"`
	Command          string                     `json:"command"`
	Message          string                     `json:"message"`
}

func decodeEvent(data []byte) (Event, error) {
	var wire wireEvent
	if err := json.Unmarshal(data, &wire); err != nil {
		return Event{}, err
	}
	event := Event{
		Type:       wire.Type,
		Options:    wire.Options,
		Position:   wire.Position,
		Item:       wire.Item,
		Context:    wire.Context,
		Status:     wire.Status,
		Volume:     wire.Volume,
		DeviceName: wire.DeviceName,
		IsActive:   wire.IsActive,
		Command:    wire.Command,
		Message:    wire.Message,
	}
	switch wire.Type {
	case "auth_state":
		auth := &AuthState{LoggedIn: wire.LoggedIn, DeviceName: wire.DeviceName}
		if wire.IsActive != nil {
			auth.IsActive = *wire.IsActive
		}
		event.Auth = auth
	case "playback_state":
		playback := &PlaybackState{
			Status:           wire.Status,
			Item:             wire.Item,
			Context:          wire.Context,
			Position:         wire.Position,
			IsActive:         wire.IsActive != nil && *wire.IsActive,
			AvailableActions: wire.AvailableActions,
		}
		if wire.Volume != nil {
			playback.Volume = *wire.Volume
		}
		if wire.Options != nil {
			playback.Options = *wire.Options
		}
		event.Playback = playback
	}
	switch {
	case wire.Queue != nil:
		event.Queue = wire.Queue
	case wire.Type == "queue_changed" || len(wire.Previous) > 0 || len(wire.Upcoming) > 0:
		event.Queue = &QueueState{Previous: wire.Previous, Upcoming: wire.Upcoming}
	}
	return event, nil
}
