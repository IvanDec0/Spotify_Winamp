package playback

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cadence/internal/artwork"
	"cadence/internal/engine"
)

func TestProviderMapsStatusAndContextQueue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/status":
			_, _ = writer.Write([]byte(`{
				"username":"user","device_id":"dev","device_name":"Cadence","device_type":"COMPUTER",
				"context_uri":"spotify:playlist:1","context_name":"Mix","stopped":false,"paused":false,
				"volume":20,"volume_steps":100,"shuffle_context":true,"repeat_context":true,
				"track":{"uri":"spotify:track:1","name":"Song","duration":1000,"position":500,"artist_names":["Artist"],"album_name":"Album","album_cover_url":"https://images.example/1.jpg"},
				"next_track":{"uri":"spotify:track:2","name":"Next","duration":2000,"artist_names":["Artist"],"album_name":"Album"}
			}`))
		case "/context/tracks":
			_, _ = writer.Write([]byte(`{"uri":"spotify:playlist:1","ready":true,"length":2,"cached":2,"tracks":[
				{"uri":"spotify:track:1","track":{"uri":"spotify:track:1","name":"Song","duration":1000,"artist_names":["Artist"],"album_name":"Album"}},
				{"uri":"spotify:track:2","track":{"uri":"spotify:track:2","name":"Next","duration":2000,"artist_names":["Artist"],"album_name":"Album"}}
			]}`))
		default:
			writer.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()
	runtime := engine.NewSupervisor(engine.Options{Client: engine.NewClient(server.URL, server.Client())})
	provider := NewEngineProvider(runtime, artwork.New(server.Client()))

	state, err := provider.State(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.Provider != "spotify" || state.Status != StatusPlaying || state.Track.Title != "Song" {
		t.Fatalf("state = %#v", state)
	}
	if !strings.HasPrefix(state.Track.Cover, "/art/") || state.Volume != 20 {
		t.Fatalf("cover/volume = %#v", state)
	}
	if state.Repeat != RepeatContext || !state.Shuffle {
		t.Fatalf("controls = %#v", state)
	}
	if len(state.Queue) != 2 || !state.Queue[0].IsCurrent || !state.Queue[1].IsNext {
		t.Fatalf("queue = %#v", state.Queue)
	}
	if state.Device.Name != "Cadence" || len(state.Devices) != 1 {
		t.Fatalf("device = %#v", state.Device)
	}
}

func TestProviderNoSessionShowsPairingError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	runtime := engine.NewSupervisor(engine.Options{Client: engine.NewClient(server.URL, server.Client())})
	provider := NewEngineProvider(runtime, nil)
	state, err := provider.State(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.Error == nil || state.Error.Code != "not_authenticated" {
		t.Fatalf("error = %#v", state.Error)
	}
}

func TestProviderActions(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		paths = append(paths, request.URL.Path+":"+string(body))
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	runtime := engine.NewSupervisor(engine.Options{Client: engine.NewClient(server.URL, server.Client())})
	provider := NewEngineProvider(runtime, nil)
	ctx := context.Background()
	if err := provider.Pause(ctx); err != nil {
		t.Fatal(err)
	}
	if err := provider.Seek(ctx, 1234); err != nil {
		t.Fatal(err)
	}
	if err := provider.SetVolume(ctx, 40); err != nil {
		t.Fatal(err)
	}
	if err := provider.SetRepeat(ctx, RepeatTrack); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(paths, "\n")
	for _, want := range []string{"/player/pause", `"position":1234`, `"volume":40`, `"repeat_track":true`} {
		if !strings.Contains(joined, want) {
			t.Fatalf("requests missing %q:\n%s", want, joined)
		}
	}
}

func TestProviderTransferLocalDevice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"device_id":"dev","device_name":"Cadence","stopped":true,"volume":100,"volume_steps":100}`))
	}))
	defer server.Close()
	runtime := engine.NewSupervisor(engine.Options{Client: engine.NewClient(server.URL, server.Client())})
	provider := NewEngineProvider(runtime, nil)
	if err := provider.Transfer(context.Background(), "dev"); err != nil {
		t.Fatal(err)
	}
	if err := provider.Transfer(context.Background(), "other"); err == nil {
		t.Fatal("remote transfer error = nil")
	}
}
