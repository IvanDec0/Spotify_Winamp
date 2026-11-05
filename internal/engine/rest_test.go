package engine

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestClientStatusAuthAndPlayback(t *testing.T) {
	var lastPlay map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/status":
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"username":"user","device_id":"dev","device_name":"Cadence","device_type":"COMPUTER","stopped":false,"paused":true,"volume":20,"volume_steps":100,"track":{"uri":"spotify:track:1","name":"Track","duration":1000,"artist_names":["Artist"],"album_name":"Album","position":500}}`))
		case "/auth/code":
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"url":"https://spotify.com/pair?code=ABC","code":"ABC","expires_at":"2026-11-03T22:00:00Z"}`))
		case "/player/play":
			body, _ := io.ReadAll(request.Body)
			_ = json.Unmarshal(body, &lastPlay)
			writer.WriteHeader(http.StatusOK)
		case "/player/volume":
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"value":20,"max":100}`))
		default:
			writer.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()
	client := NewClient(server.URL, server.Client())

	status, err := client.Status(context.Background())
	if err != nil || status == nil {
		t.Fatalf("Status() = %#v, %v", status, err)
	}
	if status.DeviceName != "Cadence" || status.Track == nil || status.Track.Name != "Track" {
		t.Fatalf("status = %#v", status)
	}
	code, err := client.AuthCode(context.Background())
	if err != nil || code == nil || code.Code != "ABC" {
		t.Fatalf("AuthCode() = %#v, %v", code, err)
	}
	if err := client.Play(context.Background(), "spotify:playlist:1", "spotify:track:2", false, 0); err != nil {
		t.Fatal(err)
	}
	if lastPlay["uri"] != "spotify:playlist:1" || lastPlay["skip_to_uri"] != "spotify:track:2" {
		t.Fatalf("play payload = %#v", lastPlay)
	}
	value, max, err := client.Volume(context.Background())
	if err != nil || value != 20 || max != 100 {
		t.Fatalf("Volume() = %d/%d, %v", value, max, err)
	}
}

func TestClientNoSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	client := NewClient(server.URL, server.Client())
	status, err := client.Status(context.Background())
	if err != nil || status != nil {
		t.Fatalf("Status() = %#v, %v", status, err)
	}
	code, err := client.AuthCode(context.Background())
	if err != nil || code != nil {
		t.Fatalf("AuthCode() = %#v, %v", code, err)
	}
}

func TestClientLibraryAndContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/library/playlists":
			_, _ = writer.Write([]byte(`{"total":1,"offset":0,"limit":20,"items":[{"uri":"spotify:playlist:1","name":"Mix","owner_username":"owner","length":4,"image_url":"https://images.example/p.jpg"}]}`))
		case "/library/albums":
			_, _ = writer.Write([]byte(`{"total":1,"offset":0,"limit":20,"items":[{"uri":"spotify:album:1","name":"Album","artist_names":["Artist"],"year":2026,"image_url":null}]}`))
		case "/library/artists":
			_, _ = writer.Write([]byte(`{"total":1,"offset":0,"limit":20,"items":[{"uri":"spotify:artist:1","name":"Artist","image_url":null}]}`))
		case "/context/tracks":
			_, _ = writer.Write([]byte(`{"uri":"spotify:playlist:1","ready":true,"length":1,"cached":1,"tracks":[{"uri":"spotify:track:1","track":{"uri":"spotify:track:1","name":"Song","duration":1000,"artist_names":["Artist"],"album_name":"Album"}}]}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := NewClient(server.URL, server.Client())

	playlists, err := client.Playlists(context.Background(), 0, 20)
	if err != nil || len(playlists.Items) != 1 || playlists.Items[0].Name != "Mix" {
		t.Fatalf("Playlists() = %#v, %v", playlists, err)
	}
	albums, err := client.Albums(context.Background(), 0, 20)
	if err != nil || len(albums.Items) != 1 || albums.Items[0].Year != 2026 {
		t.Fatalf("Albums() = %#v, %v", albums, err)
	}
	artists, err := client.Artists(context.Background(), 0, 20)
	if err != nil || len(artists.Items) != 1 || artists.Items[0].Name != "Artist" {
		t.Fatalf("Artists() = %#v, %v", artists, err)
	}
	contextTracks, err := client.ContextTracks(context.Background(), "spotify:playlist:1")
	if err != nil || !contextTracks.Ready || len(contextTracks.Items) != 1 || contextTracks.Items[0].Track.Name != "Song" {
		t.Fatalf("ContextTracks() = %#v, %v", contextTracks, err)
	}
}

func TestClientSearchAndRadio(t *testing.T) {
	var lastRadio map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/search":
			if request.URL.Query().Get("q") != "daft punk" || request.URL.Query().Get("limit") != "7" {
				t.Errorf("search query = %q", request.URL.RawQuery)
			}
			_, _ = writer.Write([]byte(`{
				"tracks":{"hits":[{"name":"One More Time","uri":"spotify:track:1","image":"https://images.example/1.jpg","artists":[{"name":"Daft Punk","uri":"spotify:artist:1"}],"album":{"name":"Discovery","uri":"spotify:album:1"},"duration":320000}],"total":1},
				"albums":{"hits":[{"name":"Discovery","uri":"spotify:album:1","artists":[{"name":"Daft Punk","uri":"spotify:artist:1"}]}],"total":1},
				"artists":{"hits":[{"name":"Daft Punk","uri":"spotify:artist:1"}],"total":1},
				"playlists":{"hits":[{"name":"This Is Daft Punk","uri":"spotify:playlist:1","author":"Spotify","followersCount":42}],"total":1}
			}`))
		case "/player/radio":
			body, _ := io.ReadAll(request.Body)
			_ = json.Unmarshal(body, &lastRadio)
			_, _ = writer.Write([]byte(`null`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := NewClient(server.URL, server.Client())

	results, err := client.Search(context.Background(), "daft punk", 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(results.Tracks.Hits) != 1 || results.Tracks.Hits[0].Name != "One More Time" {
		t.Fatalf("tracks = %#v", results.Tracks)
	}
	if len(results.Albums.Hits) != 1 || results.Albums.Hits[0].Name != "Discovery" {
		t.Fatalf("albums = %#v", results.Albums)
	}
	if len(results.Artists.Hits) != 1 || len(results.Playlists.Hits) != 1 {
		t.Fatalf("artists/playlists = %#v / %#v", results.Artists, results.Playlists)
	}

	if err := client.StartRadio(context.Background(), "spotify:artist:1"); err != nil {
		t.Fatal(err)
	}
	if lastRadio["uri"] != "spotify:artist:1" {
		t.Fatalf("radio payload = %#v", lastRadio)
	}
}

type collectionCall struct {
	path string
	body map[string]any
}

func TestClientCollectionWrites(t *testing.T) {
	var mu sync.Mutex
	var calls []collectionCall
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		raw, _ := io.ReadAll(request.Body)
		body := map[string]any{}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &body)
		}
		mu.Lock()
		calls = append(calls, collectionCall{path: request.URL.Path, body: body})
		mu.Unlock()
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/library/collection/contains" {
			_, _ = writer.Write([]byte(`{"contains":[true,false]}`))
			return
		}
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	client := NewClient(server.URL, server.Client())

	lastCall := func() collectionCall {
		t.Helper()
		mu.Lock()
		defer mu.Unlock()
		if len(calls) == 0 {
			t.Fatal("no request captured")
		}
		return calls[len(calls)-1]
	}
	assertSetAndUris := func(wantSet string, wantUris ...string) {
		t.Helper()
		call := lastCall()
		if call.body["set"] != wantSet {
			t.Fatalf("set = %#v, want %q", call.body["set"], wantSet)
		}
		uris, ok := call.body["uris"].([]any)
		if !ok || len(uris) != len(wantUris) {
			t.Fatalf("uris = %#v, want %#v", call.body["uris"], wantUris)
		}
		for index, uri := range wantUris {
			if uris[index] != uri {
				t.Fatalf("uris = %#v, want %#v", uris, wantUris)
			}
		}
	}

	if err := client.CollectionWrite(context.Background(), "collection", []string{"spotify:album:1"}, false); err != nil {
		t.Fatalf("CollectionWrite() error = %v", err)
	}
	call := lastCall()
	if call.path != "/library/collection/write" || call.body["remove"] != false {
		t.Fatalf("collection write request = %#v", call)
	}
	assertSetAndUris("collection", "spotify:album:1")

	if err := client.CollectionWrite(context.Background(), "artist", []string{"spotify:artist:1"}, true); err != nil {
		t.Fatalf("CollectionWrite(remove) error = %v", err)
	}
	call = lastCall()
	if call.path != "/library/collection/write" || call.body["remove"] != true {
		t.Fatalf("collection remove request = %#v", call)
	}
	assertSetAndUris("artist", "spotify:artist:1")

	contains, err := client.CollectionContains(context.Background(), "collection", []string{"spotify:album:1", "spotify:show:1"})
	if err != nil {
		t.Fatalf("CollectionContains() error = %v", err)
	}
	if len(contains) != 2 || !contains[0] || contains[1] {
		t.Fatalf("contains = %#v, want [true false]", contains)
	}
	call = lastCall()
	if call.path != "/library/collection/contains" {
		t.Fatalf("contains path = %q", call.path)
	}
	assertSetAndUris("collection", "spotify:album:1", "spotify:show:1")

	if err := client.AddToPlaylist(context.Background(), "spotify:playlist:1", "spotify:episode:1"); err != nil {
		t.Fatalf("AddToPlaylist() error = %v", err)
	}
	call = lastCall()
	if call.path != "/library/playlists/add_tracks" || call.body["playlist_uri"] != "spotify:playlist:1" {
		t.Fatalf("playlist add request = %#v", call)
	}
	uris, ok := call.body["uris"].([]any)
	if !ok || len(uris) != 1 || uris[0] != "spotify:episode:1" {
		t.Fatalf("playlist add uris = %#v", call.body["uris"])
	}

	if err := client.RemoveFromPlaylist(context.Background(), "spotify:playlist:1", "spotify:track:1"); err != nil {
		t.Fatalf("RemoveFromPlaylist() error = %v", err)
	}
	call = lastCall()
	if call.path != "/library/playlists/remove_track" || call.body["playlist_uri"] != "spotify:playlist:1" || call.body["uri"] != "spotify:track:1" {
		t.Fatalf("playlist remove request = %#v", call)
	}
}

func TestClientCollectionErrorsPropagate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusForbidden)
		_, _ = writer.Write([]byte("forbidden"))
	}))
	defer server.Close()
	client := NewClient(server.URL, server.Client())

	wantError := func(label string, err error) {
		t.Helper()
		if err == nil || (!strings.Contains(err.Error(), "forbidden") && !strings.Contains(err.Error(), "403")) {
			t.Fatalf("%s error = %v, want 403 forbidden", label, err)
		}
	}

	wantError("CollectionWrite()", client.CollectionWrite(context.Background(), "collection", []string{"spotify:album:1"}, false))
	_, err := client.CollectionContains(context.Background(), "collection", []string{"spotify:album:1"})
	wantError("CollectionContains()", err)
	wantError("AddToPlaylist()", client.AddToPlaylist(context.Background(), "spotify:playlist:1", "spotify:track:1"))
	wantError("RemoveFromPlaylist()", client.RemoveFromPlaylist(context.Background(), "spotify:playlist:1", "spotify:track:1"))
}

func TestClientAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusBadGateway)
		_, _ = writer.Write([]byte("engine down"))
	}))
	defer server.Close()
	client := NewClient(server.URL, server.Client())
	_, err := client.Status(context.Background())
	if err == nil {
		t.Fatal("Status() error = nil")
	}
	if !strings.Contains(err.Error(), "502") && !strings.Contains(err.Error(), "engine down") {
		t.Fatalf("error = %v", err)
	}
}

func TestClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		time.Sleep(100 * time.Millisecond)
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{}`))
	}))
	defer server.Close()
	client := NewClient(server.URL, &http.Client{Timeout: 10 * time.Millisecond})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := client.Status(ctx); err == nil {
		t.Fatal("Status() error = nil")
	}
}
