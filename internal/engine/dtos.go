package engine

import "time"

// Root describes API reachability and playback readiness.
type Root struct {
	PlaybackReady bool `json:"playback_ready"`
}

// Track is the subset of go-librespot track metadata used by Cadence.
type Track struct {
	URI           string   `json:"uri"`
	Name          string   `json:"name"`
	ArtistNames   []string `json:"artist_names"`
	ArtistURIs    []string `json:"artist_uris"`
	AlbumName     string   `json:"album_name"`
	AlbumURI      string   `json:"album_uri"`
	AlbumCoverURL *string  `json:"album_cover_url"`
	Position      int64    `json:"position"`
	Duration      int      `json:"duration"`
	TrackNumber   int      `json:"track_number"`
	DiscNumber    int      `json:"disc_number"`
	Format        string   `json:"format"`
	Codec         string   `json:"codec"`
	Bitrate       *int     `json:"bitrate"`
	SampleRate    *int     `json:"sample_rate"`
	BitDepth      *int     `json:"bit_depth"`
}

// Status is the player status returned by go-librespot.
type Status struct {
	Username           string `json:"username"`
	DeviceID           string `json:"device_id"`
	DeviceType         string `json:"device_type"`
	DeviceName         string `json:"device_name"`
	PlayOrigin         string `json:"play_origin"`
	PlayOriginDeviceID string `json:"play_origin_device_id"`
	ContextURI         string `json:"context_uri"`
	ContextName        string `json:"context_name"`
	Stopped            bool   `json:"stopped"`
	Paused             bool   `json:"paused"`
	Buffering          bool   `json:"buffering"`
	Volume             uint32 `json:"volume"`
	VolumeSteps        uint32 `json:"volume_steps"`
	RepeatContext      bool   `json:"repeat_context"`
	RepeatTrack        bool   `json:"repeat_track"`
	ShuffleContext     bool   `json:"shuffle_context"`
	Track              *Track `json:"track"`
	NextTrack          *Track `json:"next_track"`
}

type valueResponse struct {
	Value uint32 `json:"value"`
	Max   uint32 `json:"max"`
}

type authCode struct {
	URL       string    `json:"url"`
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Pairing asks the user to approve Cadence at a URL or by entering a code.
type Pairing struct {
	URL       string `json:"url"`
	Code      string `json:"code"`
	ExpiresAt string `json:"expiresAt"`
}

// AudioState is the token-free state exposed to the frontend.
type AudioState struct {
	Available       bool     `json:"available"`
	Running         bool     `json:"running"`
	Authenticated   bool     `json:"authenticated"`
	Engine          string   `json:"engine"`
	EngineVersion   string   `json:"engineVersion"`
	DeviceName      string   `json:"deviceName"`
	DeviceType      string   `json:"deviceType"`
	Pairing         *Pairing `json:"pairing"`
	Error           string   `json:"error"`
	BinaryPath      string   `json:"binaryPath"`
	ConfigDirectory string   `json:"configDirectory"`
	AudioBackend    string   `json:"audioBackend"`
}

// LibraryPlaylist is a playlist in the user's library.
type LibraryPlaylist struct {
	URI           string   `json:"uri"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Owner         string   `json:"owner"`
	Length        int      `json:"length"`
	ImageURL      string   `json:"imageUrl"`
	Collaborative bool     `json:"collaborative"`
	CanEdit       bool     `json:"canEdit"`
	Folder        []string `json:"folder"`
}

type PlaylistPage struct {
	Total  int               `json:"total"`
	Offset int               `json:"offset"`
	Limit  int               `json:"limit"`
	Items  []LibraryPlaylist `json:"items"`
}

// LibraryAlbum is an album saved in the user's library.
type LibraryAlbum struct {
	URI         string   `json:"uri"`
	Name        string   `json:"name"`
	ArtistNames []string `json:"artistNames"`
	Year        int      `json:"year"`
	ImageURL    string   `json:"imageUrl"`
}

type AlbumPage struct {
	Total  int            `json:"total"`
	Offset int            `json:"offset"`
	Limit  int            `json:"limit"`
	Items  []LibraryAlbum `json:"items"`
}

// LibraryArtist is an artist followed by the user.
type LibraryArtist struct {
	URI      string `json:"uri"`
	Name     string `json:"name"`
	ImageURL string `json:"imageUrl"`
}

type ArtistPage struct {
	Total  int             `json:"total"`
	Offset int             `json:"offset"`
	Limit  int             `json:"limit"`
	Items  []LibraryArtist `json:"items"`
}

// ContextTrack is one entry in a playlist, album, artist or Liked Songs list.
type ContextTrack struct {
	URI   string `json:"uri"`
	Track Track  `json:"track"`
}

// ContextTracks is a context listing. Ready is false while go-librespot
// enumerates the context in the background.
type ContextTracks struct {
	URI    string         `json:"uri"`
	Ready  bool           `json:"ready"`
	Length int            `json:"length"`
	Cached int            `json:"cached"`
	Items  []ContextTrack `json:"items"`
}

// SearchArtist is an artist reference inside a search hit.
type SearchArtist struct {
	Name string `json:"name"`
	URI  string `json:"uri"`
}

// SearchAlbum is an album reference inside a search hit.
type SearchAlbum struct {
	Name string `json:"name"`
	URI  string `json:"uri"`
}

// SearchHit is one result from the engine's internal Spotify search.
type SearchHit struct {
	Name           string         `json:"name"`
	URI            string         `json:"uri"`
	Image          string         `json:"image"`
	Artists        []SearchArtist `json:"artists"`
	Album          *SearchAlbum   `json:"album,omitempty"`
	Duration       int            `json:"duration"`
	Author         string         `json:"author"`
	FollowersCount int            `json:"followersCount"`
	Verified       bool           `json:"verified"`
}

// SearchGroup is one category of search results.
type SearchGroup struct {
	Hits  []SearchHit `json:"hits"`
	Total int         `json:"total"`
}

// SearchResults are the categories returned by the engine search.
type SearchResults struct {
	Tracks    SearchGroup `json:"tracks"`
	Albums    SearchGroup `json:"albums"`
	Artists   SearchGroup `json:"artists"`
	Playlists SearchGroup `json:"playlists"`
}
