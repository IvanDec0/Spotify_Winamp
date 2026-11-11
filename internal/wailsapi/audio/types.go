package audioapi

import (
	"cadence/internal/catalog"
	"cadence/internal/engine"
	"cadence/internal/settings"
)

// Type aliases keep the Wails service signatures readable while the domain
// types stay owned by engine and catalog.
type AudioState = engine.AudioState
type PlaylistPage = engine.PlaylistPage
type AlbumPage = engine.AlbumPage
type ArtistPage = engine.ArtistPage
type SearchResults = engine.SearchResults
type LibraryContext = catalog.LibraryContext
type EngineSettings = settings.EngineState
type PlaybackSettings = settings.PlaybackState

// WebPlayerStatus is the web player state shown by the settings UI. It never
// carries tokens.
type WebPlayerStatus struct {
	Available        bool   `json:"available"`
	LoggedIn         bool   `json:"loggedIn"`
	Running          bool   `json:"running"`
	RuntimeInstalled bool   `json:"runtimeInstalled"`
	RuntimeVersion   string `json:"runtimeVersion"`
	DeviceID         string `json:"deviceId"`
	Account          string `json:"account"`
	LastError        string `json:"lastError"`
	ClientID         string `json:"clientId"`
	RestartRequired  bool   `json:"restartRequired"`
}
