package playback

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"cadence/internal/artwork"
	"cadence/internal/soloist"

	"github.com/coder/websocket"
)

// fakeSoloistBackend is a controllable SoloistBackend for the provider tests.
type fakeSoloistBackend struct {
	mu     sync.Mutex
	client *soloist.Client
	state  soloist.State
}

func newFakeSoloistBackend(client *soloist.Client) *fakeSoloistBackend {
	return &fakeSoloistBackend{
		client: client,
		state:  soloist.State{Running: true, LoggedIn: true, DeviceName: "Cadence"},
	}
}

func (f *fakeSoloistBackend) Client() *soloist.Client {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.client
}

func (f *fakeSoloistBackend) State() soloist.State {
	f.mu.Lock()
	defer f.mu.Unlock()
	state := f.state
	state.Accounts = append([]string(nil), f.state.Accounts...)
	return state
}

func (f *fakeSoloistBackend) setState(state soloist.State) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state = state
}

// fakeSoloistServer answers protocol commands from an httptest WebSocket
// server. A respond hook can override the default behavior per command.
type fakeSoloistServer struct {
	mu       sync.Mutex
	commands []soloist.Command
	playback string
	queue    string
	respond  func(command soloist.Command) (payloads []string, handled bool)
}

func (s *fakeSoloistServer) setPlayback(payload string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.playback = payload
}

func (s *fakeSoloistServer) setQueue(payload string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queue = payload
}

func (s *fakeSoloistServer) setRespond(respond func(command soloist.Command) ([]string, bool)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.respond = respond
}

func (s *fakeSoloistServer) commandList() []soloist.Command {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]soloist.Command(nil), s.commands...)
}

func (s *fakeSoloistServer) commandNames() []string {
	commands := s.commandList()
	names := make([]string, 0, len(commands))
	for _, command := range commands {
		names = append(names, command.Command)
	}
	return names
}

func startFakeSoloist(t *testing.T, server *fakeSoloistServer) string {
	t.Helper()
	httpServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		conn, err := websocket.Accept(writer, request, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		for {
			_, data, err := conn.Read(context.Background())
			if err != nil {
				return
			}
			var command soloist.Command
			if err := json.Unmarshal(data, &command); err != nil {
				return
			}
			server.mu.Lock()
			server.commands = append(server.commands, command)
			respond := server.respond
			playback := server.playback
			queue := server.queue
			server.mu.Unlock()

			var payloads []string
			handled := false
			if respond != nil {
				payloads, handled = respond(command)
			}
			if !handled {
				switch command.Command {
				case "get_state":
					payloads = []string{playback}
				case "get_queue":
					payloads = []string{queue}
				default:
					payloads = []string{fmt.Sprintf(`{"type":"command_result","command":%q}`, command.Command)}
				}
			}
			for _, payload := range payloads {
				if payload == "" {
					continue
				}
				if err := conn.Write(context.Background(), websocket.MessageText, []byte(payload)); err != nil {
					return
				}
			}
		}
	}))
	t.Cleanup(httpServer.Close)
	return "ws" + strings.TrimPrefix(httpServer.URL, "http")
}

func dialSoloistClient(t *testing.T, url string) *soloist.Client {
	t.Helper()
	client, err := soloist.Dial(context.Background(), url)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Close()
	})
	return client
}

func soloistTrackEntity(uri, title string) string {
	return fmt.Sprintf(
		`{"uri":%q,"entity_type":"track","decorations":{"identity":{"name":%q},"playback":{"duration_ms":180000}}}`,
		uri,
		title,
	)
}

func TestSoloistProviderStateMapping(t *testing.T) {
	const xlargeCover = "https://images.example/xlarge.jpg"
	payload := `{
		"type":"playback_state",
		"status":"playing",
		"is_active":true,
		"volume":55,
		"options":{"shuffle":true,"repeat":"context"},
		"position":{"position_ms":60000,"timestamp_ms":1700000000000,"speed":1},
		"item":{
			"uri":"spotify:track:one",
			"entity_type":"track",
			"decorations":{
				"identity":{"name":"Song"},
				"visual_identity":{"cover":[
					{"url":"https://images.example/small.jpg","size":"small"},
					{"url":"` + xlargeCover + `","size":"xlarge"},
					{"url":"https://images.example/large.jpg","size":"large"}
				]},
				"parent":{"entity":{"uri":"spotify:album:one","entity_type":"album","decorations":{"identity":{"name":"Album"}}}},
				"creators":[
					{"entity":{"uri":"spotify:artist:a","entity_type":"artist","decorations":{"identity":{"name":"Artist A"}}}},
					{"entity":{"uri":"spotify:artist:b","entity_type":"artist","decorations":{"identity":{"name":""}}}},
					{"entity":{"uri":"spotify:artist:c","entity_type":"artist","decorations":{"identity":{"name":"Artist C"}}}}
				],
				"playback":{"duration_ms":240000}
			}
		}
	}`

	server := &fakeSoloistServer{}
	server.setPlayback(payload)
	url := startFakeSoloist(t, server)
	client := dialSoloistClient(t, url)
	art := artwork.New(nil)
	provider := NewSoloistProvider(newFakeSoloistBackend(client), art)

	state, err := provider.State(context.Background())
	if err != nil {
		t.Fatalf("State() error = %v", err)
	}
	if state.Provider != "soloist" {
		t.Fatalf("provider = %q", state.Provider)
	}
	if state.Status != StatusPlaying {
		t.Fatalf("status = %q", state.Status)
	}
	if state.Track.ID != "spotify:track:one" || state.Track.URI != "spotify:track:one" {
		t.Fatalf("track id/uri = %#v", state.Track)
	}
	if state.Track.Title != "Song" {
		t.Fatalf("title = %q", state.Track.Title)
	}
	if strings.Join(state.Track.Artists, ",") != "Artist A,Artist C" {
		t.Fatalf("artists = %#v", state.Track.Artists)
	}
	if state.Track.Album != "Album" {
		t.Fatalf("album = %q", state.Track.Album)
	}
	if state.Track.DurationMS != 240000 || state.DurationMS != 240000 {
		t.Fatalf("duration = %d/%d", state.Track.DurationMS, state.DurationMS)
	}
	wantCover := "/art/" + art.Register(xlargeCover)
	if state.Track.Cover != wantCover {
		t.Fatalf("cover = %q, want %q", state.Track.Cover, wantCover)
	}
	if state.ProgressMS != 60000 {
		t.Fatalf("progress = %d", state.ProgressMS)
	}
	if state.Volume != 55 || !state.Shuffle || state.Repeat != RepeatContext {
		t.Fatalf("controls = %#v", state)
	}
	if state.Device.ID != "soloist" || state.Device.Name != "Cadence" || state.Device.Type != "speaker" {
		t.Fatalf("device = %#v", state.Device)
	}
	if !state.Device.Active || state.Device.VolumePercent != 55 {
		t.Fatalf("device active/volume = %#v", state.Device)
	}
	if len(state.Devices) != 1 || state.Devices[0] != state.Device {
		t.Fatalf("devices = %#v", state.Devices)
	}
	if !state.CanControl || !state.CanSeek {
		t.Fatalf("canControl/canSeek = %v/%v", state.CanControl, state.CanSeek)
	}
	if state.Error != nil {
		t.Fatalf("error = %#v", state.Error)
	}
	if state.Revision != 1 || state.CapturedAt == 0 {
		t.Fatalf("revision/capturedAt = %d/%d", state.Revision, state.CapturedAt)
	}
}

func TestSoloistProviderPairingRequired(t *testing.T) {
	server := &fakeSoloistServer{}
	url := startFakeSoloist(t, server)
	client := dialSoloistClient(t, url)
	backend := newFakeSoloistBackend(client)
	backend.setState(soloist.State{NeedsPairing: true, DeviceName: "Cadence"})
	provider := NewSoloistProvider(backend, nil)

	state, err := provider.State(context.Background())
	if err != nil {
		t.Fatalf("State() error = %v", err)
	}
	if state.Status != StatusStopped {
		t.Fatalf("status = %q", state.Status)
	}
	if state.Error == nil || state.Error.Code != "soloist_pairing_required" {
		t.Fatalf("error = %#v", state.Error)
	}
	if state.Error.Message != "Open Spotify on your phone and select Cadence once." {
		t.Fatalf("message = %q", state.Error.Message)
	}
	if state.CanControl || state.CanSeek {
		t.Fatalf("canControl/canSeek = %v/%v", state.CanControl, state.CanSeek)
	}
	if len(state.Track.Artists) != 0 || state.Track.URI != "" || state.Track.Title != "" {
		t.Fatalf("track = %#v", state.Track)
	}
	if names := server.commandNames(); len(names) != 0 {
		t.Fatalf("commands = %#v, want no WebSocket query", names)
	}
}

func TestSoloistProviderNotRunning(t *testing.T) {
	backend := newFakeSoloistBackend(nil)
	provider := NewSoloistProvider(backend, nil)

	state, err := provider.State(context.Background())
	if err != nil {
		t.Fatalf("State() error = %v", err)
	}
	if state.Status != StatusStopped {
		t.Fatalf("status = %q", state.Status)
	}
	if state.Error == nil || state.Error.Code != "soloist_not_running" {
		t.Fatalf("error = %#v", state.Error)
	}
	if state.CanControl || state.CanSeek {
		t.Fatalf("canControl/canSeek = %v/%v", state.CanControl, state.CanSeek)
	}
}

func TestSoloistProviderKeepsPlaybackWithBackendError(t *testing.T) {
	server := &fakeSoloistServer{}
	server.setPlayback(`{"type":"playback_state","status":"paused","is_active":true,"volume":40,"item":` +
		soloistTrackEntity("spotify:track:one", "Song") + `}`)
	url := startFakeSoloist(t, server)
	backend := newFakeSoloistBackend(dialSoloistClient(t, url))
	backend.setState(soloist.State{
		Running:    true,
		LoggedIn:   true,
		DeviceName: "Cadence",
		ErrorCode:  soloist.CodeBuildExpired,
		Error:      "this Spotify Soloist build has expired",
	})
	provider := NewSoloistProvider(backend, nil)

	state, err := provider.State(context.Background())
	if err != nil {
		t.Fatalf("State() error = %v", err)
	}
	if state.Status != StatusPaused || state.Track.Title != "Song" {
		t.Fatalf("playback state was not preserved: %#v", state)
	}
	if state.Error == nil || state.Error.Code != soloist.CodeBuildExpired {
		t.Fatalf("error = %#v", state.Error)
	}
	if state.Error.Message != "this Spotify Soloist build has expired" {
		t.Fatalf("message = %q", state.Error.Message)
	}
}

func TestSoloistProviderPlayActivatesFirst(t *testing.T) {
	server := &fakeSoloistServer{}
	url := startFakeSoloist(t, server)
	provider := NewSoloistProvider(newFakeSoloistBackend(dialSoloistClient(t, url)), nil)

	if err := provider.Play(context.Background()); err != nil {
		t.Fatalf("Play() error = %v", err)
	}
	names := server.commandNames()
	if len(names) != 2 || names[0] != "activate" || names[1] != "play" {
		t.Fatalf("commands = %#v, want [activate play]", names)
	}
}

func TestSoloistProviderRepeatSequence(t *testing.T) {
	cases := []struct {
		name     string
		mode     RepeatMode
		commands []soloist.Command
	}{
		{
			name: "off",
			mode: RepeatOff,
			commands: []soloist.Command{
				soloist.SetRepeatTrack(false),
				soloist.SetRepeatContext(false),
			},
		},
		{
			name: "context",
			mode: RepeatContext,
			commands: []soloist.Command{
				soloist.SetRepeatTrack(false),
				soloist.SetRepeatContext(true),
			},
		},
		{
			name: "track",
			mode: RepeatTrack,
			commands: []soloist.Command{
				soloist.SetRepeatContext(false),
				soloist.SetRepeatTrack(true),
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			server := &fakeSoloistServer{}
			url := startFakeSoloist(t, server)
			provider := NewSoloistProvider(newFakeSoloistBackend(dialSoloistClient(t, url)), nil)

			if err := provider.SetRepeat(context.Background(), testCase.mode); err != nil {
				t.Fatalf("SetRepeat() error = %v", err)
			}
			commands := server.commandList()
			if len(commands) != len(testCase.commands) {
				t.Fatalf("commands = %#v, want %d", commands, len(testCase.commands))
			}
			for index, want := range testCase.commands {
				got := commands[index]
				if got.Command != want.Command {
					t.Fatalf("command %d = %q, want %q", index, got.Command, want.Command)
				}
				if got.Enabled == nil || want.Enabled == nil || *got.Enabled != *want.Enabled {
					t.Fatalf("command %d enabled = %v, want %v", index, got.Enabled, want.Enabled)
				}
			}
		})
	}

	t.Run("invalid", func(t *testing.T) {
		server := &fakeSoloistServer{}
		url := startFakeSoloist(t, server)
		provider := NewSoloistProvider(newFakeSoloistBackend(dialSoloistClient(t, url)), nil)

		err := provider.SetRepeat(context.Background(), RepeatMode("shuffle"))
		if soloist.CodeOf(err) != soloist.CodeUnsupportedAction {
			t.Fatalf("code = %q, want %q", soloist.CodeOf(err), soloist.CodeUnsupportedAction)
		}
		if names := server.commandNames(); len(names) != 0 {
			t.Fatalf("commands = %#v, want none", names)
		}
	})
}

func TestSoloistProviderSeekRetriesDroppedSeek(t *testing.T) {
	previousInterval, previousTimeout := soloistSeekPollInterval, soloistSeekPollTimeout
	soloistSeekPollInterval = 5 * time.Millisecond
	soloistSeekPollTimeout = 40 * time.Millisecond
	defer func() {
		soloistSeekPollInterval = previousInterval
		soloistSeekPollTimeout = previousTimeout
	}()

	const target = 30000
	var seeks atomic.Int64
	var position atomic.Int64
	server := &fakeSoloistServer{}
	server.setRespond(func(command soloist.Command) ([]string, bool) {
		switch command.Command {
		case "seek":
			if seeks.Add(1) > 1 {
				if command.PositionMS != nil {
					position.Store(*command.PositionMS)
				}
			}
			return []string{`{"type":"command_result","command":"seek"}`}, true
		case "get_state":
			payload := fmt.Sprintf(
				`{"type":"playback_state","status":"paused","volume":40,"position":{"position_ms":%d,"timestamp_ms":1,"speed":1}}`,
				position.Load(),
			)
			return []string{payload}, true
		}
		return nil, false
	})
	url := startFakeSoloist(t, server)
	provider := NewSoloistProvider(newFakeSoloistBackend(dialSoloistClient(t, url)), nil)

	if err := provider.Seek(context.Background(), target); err != nil {
		t.Fatalf("Seek() error = %v", err)
	}
	if seeks.Load() < 2 {
		t.Fatalf("seek commands = %d, want at least 2", seeks.Load())
	}
}

func TestSoloistProviderVolumeClamp(t *testing.T) {
	cases := []struct {
		input int
		want  int
	}{
		{input: -5, want: 0},
		{input: 150, want: 100},
	}
	for _, testCase := range cases {
		server := &fakeSoloistServer{}
		url := startFakeSoloist(t, server)
		provider := NewSoloistProvider(newFakeSoloistBackend(dialSoloistClient(t, url)), nil)

		if err := provider.SetVolume(context.Background(), testCase.input); err != nil {
			t.Fatalf("SetVolume(%d) error = %v", testCase.input, err)
		}
		commands := server.commandList()
		if len(commands) != 1 || commands[0].Command != "set_volume" {
			t.Fatalf("commands = %#v", commands)
		}
		if commands[0].Volume == nil || *commands[0].Volume != testCase.want {
			t.Fatalf("volume = %v, want %d", commands[0].Volume, testCase.want)
		}
	}
}

func TestSoloistProviderQueue(t *testing.T) {
	server := &fakeSoloistServer{}
	server.setPlayback(`{"type":"playback_state","status":"playing","is_active":true,"volume":40,"item":` +
		soloistTrackEntity("spotify:track:current", "Current") + `}`)

	entries := []string{`{"item":` + soloistTrackEntity("spotify:track:current", "Current") + `}`}
	for index := 1; index <= 60; index++ {
		entries = append(entries, `{"item":`+soloistTrackEntity(fmt.Sprintf("spotify:track:%d", index), fmt.Sprintf("Track %d", index))+`}`)
	}
	server.setQueue(`{"type":"queue_changed","previous":[],"upcoming":[` + strings.Join(entries, ",") + `]}`)

	url := startFakeSoloist(t, server)
	provider := NewSoloistProvider(newFakeSoloistBackend(dialSoloistClient(t, url)), nil)

	queue, err := provider.Queue(context.Background())
	if err != nil {
		t.Fatalf("Queue() error = %v", err)
	}
	if len(queue) != 50 {
		t.Fatalf("queue length = %d, want 50", len(queue))
	}
	if !queue[0].IsCurrent || queue[0].IsNext || queue[0].Index != 0 {
		t.Fatalf("queue[0] = %#v", queue[0])
	}
	if queue[0].Track.URI != "spotify:track:current" {
		t.Fatalf("queue[0] uri = %q", queue[0].Track.URI)
	}
	if queue[1].IsCurrent || !queue[1].IsNext || queue[1].Index != 1 {
		t.Fatalf("queue[1] = %#v", queue[1])
	}
	if queue[1].Track.URI != "spotify:track:1" {
		t.Fatalf("queue[1] uri = %q", queue[1].Track.URI)
	}
	if queue[49].Index != 49 || queue[49].Track.URI != "spotify:track:49" {
		t.Fatalf("queue[49] = %#v", queue[49])
	}
	currentCount, nextCount := 0, 0
	for index, item := range queue {
		if item.Index != index {
			t.Fatalf("queue[%d] index = %d", index, item.Index)
		}
		if item.IsCurrent {
			currentCount++
		}
		if item.IsNext {
			nextCount++
		}
	}
	if currentCount != 1 || nextCount != 1 {
		t.Fatalf("current/next flags = %d/%d, want 1/1", currentCount, nextCount)
	}
}

func TestSoloistProviderStream(t *testing.T) {
	var playback atomic.Value
	playback.Store(`{"type":"playback_state","status":"playing","is_active":true,"volume":50,"item":` +
		soloistTrackEntity("spotify:track:first", "First") + `}`)
	events := make(chan string, 4)
	serverConn := make(chan *websocket.Conn, 1)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		conn, err := websocket.Accept(writer, request, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		serverConn <- conn

		writerCtx, cancelWriter := context.WithCancel(request.Context())
		defer cancelWriter()
		var writeMu sync.Mutex
		write := func(payload string) error {
			writeMu.Lock()
			defer writeMu.Unlock()
			return conn.Write(context.Background(), websocket.MessageText, []byte(payload))
		}
		go func() {
			for {
				select {
				case <-writerCtx.Done():
					return
				case payload := <-events:
					if err := write(payload); err != nil {
						return
					}
				}
			}
		}()

		for {
			_, data, err := conn.Read(context.Background())
			if err != nil {
				return
			}
			var command soloist.Command
			if err := json.Unmarshal(data, &command); err != nil {
				return
			}
			if command.Command != "get_state" {
				continue
			}
			if err := write(playback.Load().(string)); err != nil {
				return
			}
		}
	}))
	defer server.Close()

	client := dialSoloistClient(t, "ws"+strings.TrimPrefix(server.URL, "http"))
	provider := NewSoloistProvider(newFakeSoloistBackend(client), nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	states := make(chan PlayerState, 8)
	errs := make(chan error, 1)
	go func() {
		errs <- provider.Stream(ctx, func(state PlayerState) {
			states <- state
		})
	}()

	initial := waitForState(t, states)
	if initial.Track.Title != "First" || initial.Revision != 1 {
		t.Fatalf("initial state = %#v", initial)
	}

	playback.Store(`{"type":"playback_state","status":"playing","is_active":true,"volume":50,"item":` +
		soloistTrackEntity("spotify:track:second", "Second") + `}`)
	events <- `{"type":"track_changed"}`

	changed := waitForState(t, states)
	if changed.Track.Title != "Second" || changed.Revision != 2 {
		t.Fatalf("changed state = %#v", changed)
	}

	serverSide := <-serverConn
	_ = serverSide.Close(websocket.StatusNormalClosure, "")
	select {
	case err := <-errs:
		if err == nil {
			t.Fatal("Stream() error = nil, want an error after the connection closed")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Stream() did not return after the connection closed")
	}
}

func TestSoloistProviderTransfer(t *testing.T) {
	server := &fakeSoloistServer{}
	url := startFakeSoloist(t, server)
	provider := NewSoloistProvider(newFakeSoloistBackend(dialSoloistClient(t, url)), nil)

	if err := provider.Transfer(context.Background(), ""); err != nil {
		t.Fatalf("Transfer(\"\") error = %v", err)
	}
	if err := provider.Transfer(context.Background(), "soloist"); err != nil {
		t.Fatalf("Transfer(soloist) error = %v", err)
	}
	activations := 0
	for _, name := range server.commandNames() {
		if name == "activate" {
			activations++
		}
	}
	if activations != 2 {
		t.Fatalf("activate commands = %d, want 2", activations)
	}

	err := provider.Transfer(context.Background(), "remote-device")
	if soloist.CodeOf(err) != soloist.CodeUnsupportedAction {
		t.Fatalf("Transfer(remote) code = %q, want %q", soloist.CodeOf(err), soloist.CodeUnsupportedAction)
	}
}

func TestSoloistProviderCommandError(t *testing.T) {
	server := &fakeSoloistServer{}
	server.setRespond(func(command soloist.Command) ([]string, bool) {
		if command.Command == "pause" {
			return []string{`{"type":"error","message":"command requires authentication"}`}, true
		}
		return nil, false
	})
	url := startFakeSoloist(t, server)
	provider := NewSoloistProvider(newFakeSoloistBackend(dialSoloistClient(t, url)), nil)

	err := provider.Pause(context.Background())
	if soloist.CodeOf(err) != soloist.CodeProtocol {
		t.Fatalf("code = %q, want %q", soloist.CodeOf(err), soloist.CodeProtocol)
	}
	if !strings.Contains(err.Error(), "command requires authentication") {
		t.Fatalf("error = %v", err)
	}
}

func TestSoloistProviderConcurrentAccess(t *testing.T) {
	server := &fakeSoloistServer{}
	server.setPlayback(`{"type":"playback_state","status":"playing","is_active":true,"volume":40,"item":` +
		soloistTrackEntity("spotify:track:current", "Current") + `}`)
	server.setQueue(`{"type":"queue_changed","upcoming":[{"item":` +
		soloistTrackEntity("spotify:track:next", "Next") + `}]}`)
	url := startFakeSoloist(t, server)
	provider := NewSoloistProvider(newFakeSoloistBackend(dialSoloistClient(t, url)), nil)

	var waitGroup sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		waitGroup.Add(1)
		go func(worker int) {
			defer waitGroup.Done()
			for iteration := 0; iteration < 5; iteration++ {
				switch worker % 4 {
				case 0:
					_, _ = provider.State(context.Background())
				case 1:
					_ = provider.SetVolume(context.Background(), 30)
				case 2:
					_ = provider.Pause(context.Background())
				case 3:
					_, _ = provider.Queue(context.Background())
				}
			}
		}(worker)
	}
	waitGroup.Wait()
}

func TestPlayContextActivatesThenPlaysURI(t *testing.T) {
	server := &fakeSoloistServer{}
	url := startFakeSoloist(t, server)
	provider := NewSoloistProvider(newFakeSoloistBackend(dialSoloistClient(t, url)), nil)

	if err := provider.PlayContext(context.Background(), " spotify:playlist:x ", ""); err != nil {
		t.Fatalf("PlayContext() error = %v", err)
	}
	commands := server.commandList()
	if len(commands) != 2 || commands[0].Command != "activate" || commands[1].Command != "play" {
		t.Fatalf("commands = %#v, want [activate play]", commands)
	}
	if commands[1].URI != "spotify:playlist:x" {
		t.Fatalf("play uri = %q, want the trimmed context uri", commands[1].URI)
	}
}

func TestPlayContextWithSkipPlaysStandaloneTrack(t *testing.T) {
	server := &fakeSoloistServer{}
	url := startFakeSoloist(t, server)
	provider := NewSoloistProvider(newFakeSoloistBackend(dialSoloistClient(t, url)), nil)

	if err := provider.PlayContext(context.Background(), "spotify:playlist:x", "spotify:track:y"); err != nil {
		t.Fatalf("PlayContext() error = %v", err)
	}
	commands := server.commandList()
	if len(commands) != 2 || commands[0].Command != "activate" || commands[1].Command != "play" {
		t.Fatalf("commands = %#v, want [activate play]", commands)
	}
	if commands[1].URI != "spotify:track:y" {
		t.Fatalf("play uri = %q, want the standalone track", commands[1].URI)
	}
	for _, command := range commands {
		if command.URI == "spotify:playlist:x" {
			t.Fatalf("context uri was sent despite a skip: %#v", commands)
		}
	}
}

func TestPlayContextEmptyURI(t *testing.T) {
	server := &fakeSoloistServer{}
	url := startFakeSoloist(t, server)
	provider := NewSoloistProvider(newFakeSoloistBackend(dialSoloistClient(t, url)), nil)

	err := provider.PlayContext(context.Background(), "   ", "")
	if soloist.CodeOf(err) != soloist.CodeUnsupportedAction {
		t.Fatalf("code = %q, want %q", soloist.CodeOf(err), soloist.CodeUnsupportedAction)
	}
	if names := server.commandNames(); len(names) != 0 {
		t.Fatalf("commands = %#v, want none", names)
	}
}

func TestPlayTrackDelegatesToContext(t *testing.T) {
	server := &fakeSoloistServer{}
	url := startFakeSoloist(t, server)
	provider := NewSoloistProvider(newFakeSoloistBackend(dialSoloistClient(t, url)), nil)

	if err := provider.PlayTrack(context.Background(), "spotify:track:z"); err != nil {
		t.Fatalf("PlayTrack() error = %v", err)
	}
	commands := server.commandList()
	if len(commands) != 2 || commands[0].Command != "activate" || commands[1].Command != "play" {
		t.Fatalf("commands = %#v, want [activate play]", commands)
	}
	if commands[1].URI != "spotify:track:z" {
		t.Fatalf("play uri = %q, want the track uri", commands[1].URI)
	}
}

func TestAddToQueueAcceptsTracksAndRejectsOtherURIs(t *testing.T) {
	cases := []struct {
		name     string
		uri      string
		wantURI  string
		accepted bool
	}{
		{name: "track", uri: "spotify:track:1", wantURI: "spotify:track:1", accepted: true},
		{name: "track with spaces", uri: " spotify:track:2 ", wantURI: "spotify:track:2", accepted: true},
		{name: "album", uri: "spotify:album:1"},
		{name: "playlist", uri: "spotify:playlist:1"},
		{name: "episode", uri: "spotify:episode:1"},
		{name: "empty", uri: "   "},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			server := &fakeSoloistServer{}
			url := startFakeSoloist(t, server)
			provider := NewSoloistProvider(newFakeSoloistBackend(dialSoloistClient(t, url)), nil)

			err := provider.AddToQueue(context.Background(), testCase.uri)
			if !testCase.accepted {
				if soloist.CodeOf(err) != soloist.CodeUnsupportedAction {
					t.Fatalf("code = %q, want %q", soloist.CodeOf(err), soloist.CodeUnsupportedAction)
				}
				if names := server.commandNames(); len(names) != 0 {
					t.Fatalf("commands = %#v, want none", names)
				}
				return
			}
			if err != nil {
				t.Fatalf("AddToQueue() error = %v", err)
			}
			commands := server.commandList()
			if len(commands) != 1 || commands[0].Command != "add_to_queue" {
				t.Fatalf("commands = %#v, want [add_to_queue]", commands)
			}
			if commands[0].URI != testCase.wantURI {
				t.Fatalf("queue uri = %q, want %q", commands[0].URI, testCase.wantURI)
			}
		})
	}
}

func TestStartRadioUnsupported(t *testing.T) {
	server := &fakeSoloistServer{}
	url := startFakeSoloist(t, server)
	provider := NewSoloistProvider(newFakeSoloistBackend(dialSoloistClient(t, url)), nil)

	err := provider.StartRadio(context.Background(), "spotify:artist:x")
	if soloist.CodeOf(err) != soloist.CodeUnsupportedAction {
		t.Fatalf("code = %q, want %q", soloist.CodeOf(err), soloist.CodeUnsupportedAction)
	}
	if !strings.Contains(err.Error(), "radio is not available") {
		t.Fatalf("error = %v, want the radio message", err)
	}
	if names := server.commandNames(); len(names) != 0 {
		t.Fatalf("commands = %#v, want none", names)
	}
}
