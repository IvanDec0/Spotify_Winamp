package soloist

import (
	"encoding/json"
	"testing"
)

func TestCommandJSON(t *testing.T) {
	position := int64(42000)
	volume := 0
	disabled := false
	limit := 25

	tests := []struct {
		name    string
		command Command
		want    string
	}{
		{name: "play", command: Play("spotify:track:1"), want: `{"type":"command","command":"play","uri":"spotify:track:1"}`},
		{name: "play empty", command: Play("  "), want: `{"type":"command","command":"play"}`},
		{name: "pause", command: Pause(), want: `{"type":"command","command":"pause"}`},
		{name: "skip next", command: SkipNext(), want: `{"type":"command","command":"skip_next"}`},
		{name: "skip prev", command: SkipPrev(), want: `{"type":"command","command":"skip_prev"}`},
		{name: "seek zero", command: Seek(0), want: `{"type":"command","command":"seek","position_ms":0}`},
		{name: "seek", command: Seek(position), want: `{"type":"command","command":"seek","position_ms":42000}`},
		{name: "volume zero", command: SetVolume(volume), want: `{"type":"command","command":"set_volume","volume":0}`},
		{name: "shuffle false", command: SetShuffle(disabled), want: `{"type":"command","command":"set_shuffle","enabled":false}`},
		{name: "repeat context", command: SetRepeatContext(true), want: `{"type":"command","command":"set_repeat_context","enabled":true}`},
		{name: "repeat track", command: SetRepeatTrack(false), want: `{"type":"command","command":"set_repeat_track","enabled":false}`},
		{name: "add to queue", command: AddToQueue("spotify:track:2"), want: `{"type":"command","command":"add_to_queue","uri":"spotify:track:2"}`},
		{name: "activate", command: Activate(), want: `{"type":"command","command":"activate"}`},
		{name: "deactivate", command: Deactivate(), want: `{"type":"command","command":"deactivate"}`},
		{name: "auth state", command: GetAuthState(), want: `{"type":"command","command":"get_auth_state"}`},
		{name: "state", command: GetState(), want: `{"type":"command","command":"get_state"}`},
		{name: "queue no limit", command: GetQueue(0), want: `{"type":"command","command":"get_queue"}`},
		{name: "queue limit", command: GetQueue(limit), want: `{"type":"command","command":"get_queue","limit":25}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := json.Marshal(test.command)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(data) != test.want {
				t.Fatalf("command JSON = %s, want %s", data, test.want)
			}
		})
	}
}

func TestRepeatCommands(t *testing.T) {
	tests := []struct {
		mode    string
		want    []string
		enabled []bool
	}{
		{mode: "off", want: []string{"set_repeat_track", "set_repeat_context"}, enabled: []bool{false, false}},
		{mode: "context", want: []string{"set_repeat_track", "set_repeat_context"}, enabled: []bool{false, true}},
		{mode: "track", want: []string{"set_repeat_context", "set_repeat_track"}, enabled: []bool{false, true}},
		{mode: " CONTEXT ", want: []string{"set_repeat_track", "set_repeat_context"}, enabled: []bool{false, true}},
	}
	for _, test := range tests {
		t.Run(test.mode, func(t *testing.T) {
			commands := RepeatCommands(test.mode)
			if len(commands) != len(test.want) {
				t.Fatalf("got %d commands, want %d", len(commands), len(test.want))
			}
			for index, command := range commands {
				if command.Command != test.want[index] {
					t.Fatalf("command %d = %q, want %q", index, command.Command, test.want[index])
				}
				if command.Enabled == nil || *command.Enabled != test.enabled[index] {
					t.Fatalf("command %d enabled = %v, want %v", index, command.Enabled, test.enabled[index])
				}
			}
		})
	}
	if commands := RepeatCommands("unknown"); commands != nil {
		t.Fatalf("unknown mode returned %v, want nil", commands)
	}
}

func TestDecodeAuthState(t *testing.T) {
	event, err := decodeEvent([]byte(`{"type":"auth_state","logged_in":true,"is_active":true,"device_name":"Cadence"}`))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if event.Type != "auth_state" {
		t.Fatalf("type = %q", event.Type)
	}
	if event.Auth == nil || !event.Auth.LoggedIn || !event.Auth.IsActive || event.Auth.DeviceName != "Cadence" {
		t.Fatalf("auth = %+v", event.Auth)
	}
	if event.IsActive == nil || !*event.IsActive {
		t.Fatalf("is_active = %v", event.IsActive)
	}
}

func TestDecodePlaybackState(t *testing.T) {
	data := `{
		"type":"playback_state",
		"status":"playing",
		"is_active":true,
		"volume":42,
		"options":{"shuffle":true,"repeat":"context","playback_speed":1},
		"position":{"position_ms":1000,"timestamp_ms":2000,"speed":1},
		"item":{"uri":"spotify:track:x","entity_type":"track","decorations":{
			"identity":{"name":"Song"},
			"visual_identity":{"cover":[{"url":"https://i.scdn.co/x","size":"large"}]},
			"playback":{"duration_ms":123456}
		}},
		"context":{"uri":"spotify:album:y","entity_type":"album","decorations":{"identity":{"name":"Album"}}},
		"available_actions":{"pause":{}}
	}`
	event, err := decodeEvent([]byte(data))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if event.Playback == nil {
		t.Fatal("playback is nil")
	}
	if event.Playback.Status != "playing" || !event.Playback.IsActive || event.Playback.Volume != 42 {
		t.Fatalf("playback = %+v", event.Playback)
	}
	if !event.Playback.Options.Shuffle || event.Playback.Options.Repeat != "context" {
		t.Fatalf("options = %+v", event.Playback.Options)
	}
	if event.Playback.Position == nil || event.Playback.Position.PositionMS != 1000 {
		t.Fatalf("position = %+v", event.Playback.Position)
	}
	if event.Playback.Item == nil || event.Playback.Item.Decorations.Identity.Name != "Song" {
		t.Fatalf("item = %+v", event.Playback.Item)
	}
	if event.Playback.Item.Decorations.Playback == nil || event.Playback.Item.Decorations.Playback.DurationMS != 123456 {
		t.Fatalf("item playback = %+v", event.Playback.Item.Decorations.Playback)
	}
	if _, ok := event.Playback.AvailableActions["pause"]; !ok {
		t.Fatalf("available actions = %v", event.Playback.AvailableActions)
	}
}

func TestDecodeQueueAndSimpleEvents(t *testing.T) {
	event, err := decodeEvent([]byte(`{"type":"queue_changed","previous":[{"uid":"1","source":"queue","item":{"uri":"spotify:track:a"}}],"upcoming":[{"uid":"2","source":"context"}]}`))
	if err != nil {
		t.Fatalf("decode queue: %v", err)
	}
	if event.Queue == nil || len(event.Queue.Previous) != 1 || len(event.Queue.Upcoming) != 1 {
		t.Fatalf("queue = %+v", event.Queue)
	}
	if event.Queue.Previous[0].Item == nil || event.Queue.Previous[0].Item.URI != "spotify:track:a" {
		t.Fatalf("previous item = %+v", event.Queue.Previous[0].Item)
	}

	event, err = decodeEvent([]byte(`{"type":"error","message":"bad key"}`))
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if event.Type != "error" || event.Message != "bad key" {
		t.Fatalf("event = %+v", event)
	}

	event, err = decodeEvent([]byte(`{"type":"command_result","command":"play"}`))
	if err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if event.Command != "play" {
		t.Fatalf("command = %q", event.Command)
	}
}
