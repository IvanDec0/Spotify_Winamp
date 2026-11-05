package catalog_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	"cadence/internal/artwork"
	"cadence/internal/catalog"
	"cadence/internal/engine"
)

var artKeyPattern = regexp.MustCompile(`^/art/[0-9a-f]{64}$`)

func newTestService(t *testing.T, handler http.Handler) *catalog.Service {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	supervisor := engine.NewSupervisor(engine.Options{
		Client: engine.NewClient(server.URL, server.Client()),
	})
	return catalog.NewService(catalog.Options{
		Supervisor: supervisor,
		Art:        artwork.New(nil),
		Enabled:    true,
	})
}

func assertArtKey(t *testing.T, value string) {
	t.Helper()
	if !artKeyPattern.MatchString(value) {
		t.Fatalf("art URL = %q, want /art/<64 lowercase hex>", value)
	}
	if strings.Contains(value, "http") {
		t.Fatalf("art URL %q leaks a remote URL", value)
	}
}

func TestGetStateDisabledReturnsDemoState(t *testing.T) {
	service := catalog.NewService(catalog.Options{Enabled: false})
	state := service.GetState()

	if state.Available || state.Running {
		t.Fatalf("disabled state available=%v running=%v, want false/false", state.Available, state.Running)
	}
	if state.Engine != "go-librespot" {
		t.Fatalf("engine = %q, want go-librespot", state.Engine)
	}
	if state.Error != "The local audio engine is disabled in demo mode." {
		t.Fatalf("error = %q, want the demo-mode error", state.Error)
	}
}

func TestGetStateEnabledWithoutEngineDoesNotPanic(t *testing.T) {
	service := catalog.NewService(catalog.Options{
		Supervisor: engine.NewSupervisor(engine.Options{}),
		Enabled:    true,
	})
	state := service.GetState()

	if state.Running {
		t.Fatal("running = true without a sidecar")
	}
	if state.Engine != "go-librespot" {
		t.Fatalf("engine = %q, want go-librespot", state.Engine)
	}
	if state.Pairing != nil {
		t.Fatalf("pairing = %#v, want nil", state.Pairing)
	}
}

func TestLibraryPagesRewriteImages(t *testing.T) {
	service := newTestService(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/library/playlists":
			_, _ = writer.Write([]byte(`{"total":2,"offset":5,"limit":10,"items":[
				{"uri":"spotify:playlist:1","name":"Mix","description":"Desc","owner_username":"owner","length":4,"image_url":"https://images.example/p.jpg","collaborative":true,"can_edit":true,"folder":["A","B"]}
			]}`))
		case "/library/albums":
			_, _ = writer.Write([]byte(`{"total":2,"offset":0,"limit":10,"items":[
				{"uri":"spotify:album:1","name":"Album","artist_names":["Artist"],"year":2026,"image_url":"https://images.example/a.jpg"},
				{"uri":"spotify:album:2","name":"Null Art","artist_names":[],"year":2020,"image_url":null}
			]}`))
		case "/library/artists":
			_, _ = writer.Write([]byte(`{"total":1,"offset":0,"limit":10,"items":[
				{"uri":"spotify:artist:1","name":"Artist","image_url":"https://images.example/artist.jpg"}
			]}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))

	playlists, err := service.Playlists(5, 10)
	if err != nil {
		t.Fatalf("Playlists() error = %v", err)
	}
	if playlists.Total != 2 || playlists.Offset != 5 || playlists.Limit != 10 || len(playlists.Items) != 1 {
		t.Fatalf("playlists page = %#v", playlists)
	}
	playlist := playlists.Items[0]
	if playlist.URI != "spotify:playlist:1" || playlist.Name != "Mix" || playlist.Description != "Desc" ||
		playlist.Owner != "owner" || playlist.Length != 4 || !playlist.Collaborative || !playlist.CanEdit ||
		len(playlist.Folder) != 2 {
		t.Fatalf("playlist item = %#v", playlist)
	}
	assertArtKey(t, playlist.ImageURL)

	albums, err := service.Albums(0, 10)
	if err != nil {
		t.Fatalf("Albums() error = %v", err)
	}
	if albums.Total != 2 || len(albums.Items) != 2 {
		t.Fatalf("albums page = %#v", albums)
	}
	if albums.Items[0].URI != "spotify:album:1" || albums.Items[0].Name != "Album" ||
		albums.Items[0].Year != 2026 || len(albums.Items[0].ArtistNames) != 1 {
		t.Fatalf("album item = %#v", albums.Items[0])
	}
	assertArtKey(t, albums.Items[0].ImageURL)
	if albums.Items[1].ImageURL != "" {
		t.Fatalf("null album image = %q, want empty", albums.Items[1].ImageURL)
	}

	artists, err := service.Artists(0, 10)
	if err != nil {
		t.Fatalf("Artists() error = %v", err)
	}
	if artists.Total != 1 || len(artists.Items) != 1 || artists.Items[0].URI != "spotify:artist:1" ||
		artists.Items[0].Name != "Artist" {
		t.Fatalf("artists page = %#v", artists)
	}
	assertArtKey(t, artists.Items[0].ImageURL)
}

func TestContextTracksMapsFieldsAndCover(t *testing.T) {
	service := newTestService(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/context/tracks" {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		if got := request.URL.Query().Get("uri"); got != "spotify:playlist:1" {
			t.Errorf("context uri = %q, want spotify:playlist:1", got)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"uri":"spotify:playlist:1","ready":true,"length":2,"cached":1,"tracks":[
			{"uri":"spotify:track:1","track":{"uri":"spotify:track:1","name":"Song","artist_names":["Artist","Other"],"album_name":"Album","duration":1000,"album_cover_url":"https://images.example/c.jpg"}},
			{"uri":"spotify:track:2","track":{"uri":"spotify:track:2","name":"No Cover","artist_names":[],"album_name":"Other","duration":2000}}
		]}`))
	}))

	result, err := service.ContextTracks("spotify:playlist:1")
	if err != nil {
		t.Fatalf("ContextTracks() error = %v", err)
	}
	if result.URI != "spotify:playlist:1" || !result.Ready || result.Length != 2 || result.Cached != 1 {
		t.Fatalf("context = %#v", result)
	}
	if len(result.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(result.Items))
	}
	item := result.Items[0]
	if item.URI != "spotify:track:1" || item.Title != "Song" || item.Album != "Album" || item.DurationMS != 1000 {
		t.Fatalf("item = %#v", item)
	}
	if len(item.Artists) != 2 || item.Artists[0] != "Artist" || item.Artists[1] != "Other" {
		t.Fatalf("artists = %#v", item.Artists)
	}
	assertArtKey(t, item.Cover)
	if result.Items[1].Cover != "" {
		t.Fatalf("missing cover = %q, want empty", result.Items[1].Cover)
	}
}

func TestSearchRewritesAllGroupsAndRejectsBlankQuery(t *testing.T) {
	service := newTestService(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/search" {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		if got := request.URL.Query().Get("q"); got != "daft punk" {
			t.Errorf("search query = %q, want daft punk", got)
		}
		if got := request.URL.Query().Get("limit"); got != "7" {
			t.Errorf("search limit = %q, want 7", got)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"tracks":{"hits":[{"name":"One More Time","uri":"spotify:track:1","image":"https://images.example/t.jpg","artists":[{"name":"Daft Punk","uri":"spotify:artist:1"}],"album":{"name":"Discovery","uri":"spotify:album:1"},"duration":320000}],"total":1},
			"albums":{"hits":[{"name":"Discovery","uri":"spotify:album:1","image":"https://images.example/al.jpg"}],"total":1},
			"artists":{"hits":[{"name":"Daft Punk","uri":"spotify:artist:1","image":"https://images.example/ar.jpg"}],"total":1},
			"playlists":{"hits":[{"name":"This Is Daft Punk","uri":"spotify:playlist:1","image":"https://images.example/p.jpg"}],"total":1},
			"shows":{"hits":[{"name":"Daft Punk Stories","uri":"spotify:show:1","image":"https://images.example/s.jpg","showType":"podcast","musicAndTalk":true}],"total":1},
			"episodes":{"hits":[{"name":"Episode One","uri":"spotify:episode:1","image":"https://images.example/e.jpg","explicit":true,"musicAndTalk":true}],"total":1}
		}`))
	}))

	results, err := service.Search("  daft punk  ", 7)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results.Tracks.Hits) != 1 || len(results.Albums.Hits) != 1 ||
		len(results.Artists.Hits) != 1 || len(results.Playlists.Hits) != 1 ||
		len(results.Shows.Hits) != 1 || len(results.Episodes.Hits) != 1 {
		t.Fatalf("search groups = %#v", results)
	}
	groups := []struct {
		name string
		hit  engine.SearchHit
	}{
		{name: "tracks", hit: results.Tracks.Hits[0]},
		{name: "albums", hit: results.Albums.Hits[0]},
		{name: "artists", hit: results.Artists.Hits[0]},
		{name: "playlists", hit: results.Playlists.Hits[0]},
		{name: "shows", hit: results.Shows.Hits[0]},
		{name: "episodes", hit: results.Episodes.Hits[0]},
	}
	for _, group := range groups {
		if group.hit.Name == "" || group.hit.URI == "" {
			t.Fatalf("%s hit lost fields: %#v", group.name, group.hit)
		}
		assertArtKey(t, group.hit.Image)
	}
	if results.Tracks.Hits[0].Name != "One More Time" || results.Albums.Hits[0].Name != "Discovery" ||
		results.Artists.Hits[0].Name != "Daft Punk" || results.Playlists.Hits[0].Name != "This Is Daft Punk" ||
		results.Shows.Hits[0].Name != "Daft Punk Stories" || results.Episodes.Hits[0].Name != "Episode One" {
		t.Fatalf("search hits = %#v", results)
	}
	show := results.Shows.Hits[0]
	if show.ShowType != "podcast" || !show.MusicAndTalk || show.Explicit {
		t.Fatalf("show hit = %#v", show)
	}
	episode := results.Episodes.Hits[0]
	if !episode.Explicit || !episode.MusicAndTalk || episode.ShowType != "" {
		t.Fatalf("episode hit = %#v", episode)
	}

	if _, err := service.Search("   ", 7); err == nil || err.Error() != "the search query is empty" {
		t.Fatalf("blank search error = %v, want the search query is empty", err)
	}
}

type capturedRequest struct {
	path string
	body map[string]any
}

func TestPlaybackActionsSendExpectedBodies(t *testing.T) {
	var mu sync.Mutex
	var requests []capturedRequest
	service := newTestService(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		decoded := map[string]any{}
		if len(body) > 0 {
			_ = json.Unmarshal(body, &decoded)
		}
		mu.Lock()
		requests = append(requests, capturedRequest{path: request.URL.Path, body: decoded})
		mu.Unlock()
		writer.WriteHeader(http.StatusOK)
	}))

	lastRequest := func() capturedRequest {
		t.Helper()
		mu.Lock()
		defer mu.Unlock()
		if len(requests) == 0 {
			t.Fatal("no request captured")
		}
		return requests[len(requests)-1]
	}

	if err := service.PlayContext("spotify:playlist:1", "spotify:track:2"); err != nil {
		t.Fatalf("PlayContext() error = %v", err)
	}
	request := lastRequest()
	if request.path != "/player/play" || request.body["uri"] != "spotify:playlist:1" ||
		request.body["skip_to_uri"] != "spotify:track:2" {
		t.Fatalf("play context request = %#v", request)
	}

	if err := service.PlayTrack("spotify:track:3"); err != nil {
		t.Fatalf("PlayTrack() error = %v", err)
	}
	request = lastRequest()
	if request.path != "/player/play" || request.body["uri"] != "spotify:track:3" {
		t.Fatalf("play track request = %#v", request)
	}
	if _, ok := request.body["skip_to_uri"]; ok {
		t.Fatalf("play track sent skip_to_uri: %#v", request.body)
	}

	if err := service.AddToQueue("spotify:track:4"); err != nil {
		t.Fatalf("AddToQueue() error = %v", err)
	}
	request = lastRequest()
	if request.path != "/player/add_to_queue" || request.body["uri"] != "spotify:track:4" {
		t.Fatalf("add to queue request = %#v", request)
	}

	if err := service.StartRadio("spotify:artist:1"); err != nil {
		t.Fatalf("StartRadio() error = %v", err)
	}
	request = lastRequest()
	if request.path != "/player/radio" || request.body["uri"] != "spotify:artist:1" {
		t.Fatalf("radio request = %#v", request)
	}

	mu.Lock()
	before := len(requests)
	mu.Unlock()
	if err := service.StartRadio("   "); err == nil || err.Error() != "the radio seed is empty" {
		t.Fatalf("blank radio error = %v, want the radio seed is empty", err)
	}
	mu.Lock()
	after := len(requests)
	mu.Unlock()
	if after != before {
		t.Fatalf("blank radio sent %d requests", after-before)
	}
}

func TestLibraryWritesAndYourEpisodes(t *testing.T) {
	var mu sync.Mutex
	var requests []capturedRequest
	service := newTestService(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		decoded := map[string]any{}
		if len(body) > 0 {
			_ = json.Unmarshal(body, &decoded)
		}
		mu.Lock()
		requests = append(requests, capturedRequest{path: request.URL.Path, body: decoded})
		mu.Unlock()
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/status":
			_, _ = writer.Write([]byte(`{"username":"listener"}`))
		case "/library/collection/contains":
			_, _ = writer.Write([]byte(`{"contains":[true]}`))
		case "/context/tracks":
			if got := request.URL.Query().Get("uri"); got != "spotify:user:listener:collection:your-episodes" {
				t.Errorf("your-episodes context uri = %q", got)
			}
			_, _ = writer.Write([]byte(`{"uri":"spotify:user:listener:collection:your-episodes","ready":true,"length":1,"cached":1,"tracks":[{"uri":"spotify:episode:1","track":{"uri":"spotify:episode:1","name":"Episode","artist_names":["Host"],"album_name":"Show","duration":1800000,"album_cover_url":"https://images.example/ep.jpg"}}]}`))
		default:
			writer.WriteHeader(http.StatusOK)
		}
	}))

	lastRequest := func() capturedRequest {
		t.Helper()
		mu.Lock()
		defer mu.Unlock()
		if len(requests) == 0 {
			t.Fatal("no request captured")
		}
		return requests[len(requests)-1]
	}
	uris := func(body map[string]any) []string {
		raw, _ := body["uris"].([]any)
		out := make([]string, len(raw))
		for index, value := range raw {
			out[index], _ = value.(string)
		}
		return out
	}
	assertWrite := func(wantSet string, wantRemove bool, wantURI string) {
		t.Helper()
		request := lastRequest()
		if request.path != "/library/collection/write" || request.body["set"] != wantSet || request.body["remove"] != wantRemove {
			t.Fatalf("collection write request = %#v", request)
		}
		if got := uris(request.body); len(got) != 1 || got[0] != wantURI {
			t.Fatalf("collection write uris = %#v, want %q", got, wantURI)
		}
	}
	assertContains := func(wantSet, wantURI string) {
		t.Helper()
		request := lastRequest()
		if request.path != "/library/collection/contains" || request.body["set"] != wantSet {
			t.Fatalf("collection contains request = %#v", request)
		}
		if got := uris(request.body); len(got) != 1 || got[0] != wantURI {
			t.Fatalf("collection contains uris = %#v, want %q", got, wantURI)
		}
	}

	if err := service.SetLiked("spotify:track:1", true); err != nil {
		t.Fatalf("SetLiked() error = %v", err)
	}
	assertWrite("collection", false, "spotify:track:1")

	if err := service.SetLiked("spotify:track:1", false); err != nil {
		t.Fatalf("SetLiked(remove) error = %v", err)
	}
	assertWrite("collection", true, "spotify:track:1")

	if err := service.SaveAlbum("spotify:album:1", true); err != nil {
		t.Fatalf("SaveAlbum() error = %v", err)
	}
	assertWrite("collection", false, "spotify:album:1")

	if err := service.SaveShow("spotify:show:1", true); err != nil {
		t.Fatalf("SaveShow() error = %v", err)
	}
	assertWrite("collection", false, "spotify:show:1")

	if err := service.FollowArtist("spotify:artist:1", true); err != nil {
		t.Fatalf("FollowArtist() error = %v", err)
	}
	assertWrite("artist", false, "spotify:artist:1")

	liked, err := service.IsLiked("spotify:track:1")
	if err != nil || !liked {
		t.Fatalf("IsLiked() = %v, %v", liked, err)
	}
	assertContains("collection", "spotify:track:1")
	savedAlbum, err := service.IsAlbumSaved("spotify:album:1")
	if err != nil || !savedAlbum {
		t.Fatalf("IsAlbumSaved() = %v, %v", savedAlbum, err)
	}
	assertContains("collection", "spotify:album:1")
	savedShow, err := service.IsShowSaved("spotify:show:1")
	if err != nil || !savedShow {
		t.Fatalf("IsShowSaved() = %v, %v", savedShow, err)
	}
	assertContains("collection", "spotify:show:1")
	followed, err := service.IsArtistFollowed("spotify:artist:1")
	if err != nil || !followed {
		t.Fatalf("IsArtistFollowed() = %v, %v", followed, err)
	}
	assertContains("artist", "spotify:artist:1")

	if err := service.AddToPlaylist("spotify:playlist:1", "spotify:episode:1"); err != nil {
		t.Fatalf("AddToPlaylist() error = %v", err)
	}
	request := lastRequest()
	if request.path != "/library/playlists/add_tracks" || request.body["playlist_uri"] != "spotify:playlist:1" {
		t.Fatalf("playlist add request = %#v", request)
	}
	if got := uris(request.body); len(got) != 1 || got[0] != "spotify:episode:1" {
		t.Fatalf("playlist add uris = %#v", got)
	}

	if err := service.RemoveFromPlaylist("spotify:playlist:1", "spotify:track:1"); err != nil {
		t.Fatalf("RemoveFromPlaylist() error = %v", err)
	}
	request = lastRequest()
	if request.path != "/library/playlists/remove_track" ||
		request.body["playlist_uri"] != "spotify:playlist:1" || request.body["uri"] != "spotify:track:1" {
		t.Fatalf("playlist remove request = %#v", request)
	}

	episodes, err := service.YourEpisodes()
	if err != nil {
		t.Fatalf("YourEpisodes() error = %v", err)
	}
	if episodes.URI != "spotify:user:listener:collection:your-episodes" || !episodes.Ready || episodes.Length != 1 {
		t.Fatalf("your episodes = %#v", episodes)
	}
	if len(episodes.Items) != 1 {
		t.Fatalf("your episodes items = %d, want 1", len(episodes.Items))
	}
	episode := episodes.Items[0]
	if episode.URI != "spotify:episode:1" || episode.Title != "Episode" || episode.Album != "Show" ||
		len(episode.Artists) != 1 || episode.Artists[0] != "Host" || episode.DurationMS != 1800000 {
		t.Fatalf("your episodes item = %#v", episode)
	}
	assertArtKey(t, episode.Cover)
}

func TestYourEpisodesRequiresUsername(t *testing.T) {
	service := newTestService(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Path != "/status" {
			t.Errorf("unexpected request %s", request.URL.Path)
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = writer.Write([]byte(`{"username":""}`))
	}))

	if _, err := service.YourEpisodes(); err == nil || err.Error() != "the engine did not report a username" {
		t.Fatalf("YourEpisodes() error = %v, want the engine did not report a username", err)
	}
}

func TestLibraryRouteErrorsPropagate(t *testing.T) {
	service := newTestService(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
		_, _ = writer.Write([]byte("boom"))
	}))

	if _, err := service.Albums(0, 10); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("Albums() error = %v, want the engine error", err)
	}
}

func TestMissingClientReturnsEngineNotRunning(t *testing.T) {
	service := catalog.NewService(catalog.Options{
		Supervisor: engine.NewSupervisor(engine.Options{}),
		Art:        artwork.New(nil),
		Enabled:    true,
	})

	if _, err := service.Playlists(0, 10); err == nil || err.Error() != "the local audio engine is not running" {
		t.Fatalf("Playlists() error = %v, want the local audio engine is not running", err)
	}
}
