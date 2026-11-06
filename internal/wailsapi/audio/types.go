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
