# Architecture

This document describes the Go package layout introduced in Phase 3 of the v2
plan. It is the implementation-level companion to
[ADR-0001](adr/0001-module-boundaries.md).

## Dependency rule

```text
domain  <-  adapters  <-  app
```

- **Domain** packages own behavior and data: `engine`, `catalog`, `playback`,
  `artwork`, `skins`, `settings`, `diagnostics`.
- **Adapters** translate domain services to a delivery mechanism:
  `wailsapi/audio`, `wailsapi/player`, `wailsapi/skins`.
- **App** is the composition root in `main.go`.

Domain packages never import Wails. Only `internal/wailsapi/**`, the native
window exception `internal/platform`, and the `main.go` composition root import
`github.com/wailsapp/wails/v3/pkg/application`.

## Package graph

```text
main.go
  ├── engine        supervisor, REST transport, raw engine DTOs
  ├── catalog       library/search/context orchestration, frontend context models
  ├── playback      Provider, PlayerState, DemoProvider, EngineProvider
  ├── artwork       bounded album-art proxy with variants, single-flight and a disk LRU
  ├── skins         skin manager, types, validation, CSS, layout, ZIP, assets
  ├── settings      user settings store
  ├── diagnostics   counters and round tripper
  ├── platform      native window/GTK (Wails exception)
  └── wailsapi
        ├── audio   package audioapi  -> catalog + engine
        ├── player  package playerapi -> playback
        └── skins   package skinsapi  -> skins

engine     <- catalog, playback, wailsapi/audio
artwork    <- catalog, playback, main
playback   <- wailsapi/player
catalog    <- wailsapi/audio
skins      <- wailsapi/skins
```

`engine` does not import `catalog`. `catalog` may return `engine` wire DTOs
directly because they are the same frontend-facing shape; it owns the compact
`ContextTrackInfo` and `LibraryContext` models built from `engine.ContextTracks`.

## File map

| Package | Files |
|---|---|
| `internal/engine` | `supervisor.go` (process, config, version, pairing state), `rest.go` (typed REST client, `APIError`), `dtos.go` (raw engine JSON) |
| `internal/catalog` | `service.go` (GetState, ResetSession, Restart, Playlists, Albums, Artists, ContextTracks, YourEpisodes, Search with tracks/albums/artists/playlists/shows/episodes, PlayContext, PlayTrack, AddToQueue, StartRadio, SetLiked/IsLiked, SaveAlbum/IsAlbumSaved, SaveShow/IsShowSaved, FollowArtist/IsArtistFollowed, AddToPlaylist/RemoveFromPlaylist, artwork URL mapping) |
| `internal/playback` | `provider.go` (`Provider` interface), `state.go` (`PlayerState`, `Track`, `Device`, `QueueItem`, `PlayerError`), `demo.go` (`DemoProvider`), `engine_provider.go` (`EngineProvider`) |
| `internal/artwork` | `cache.go` (memory 256 entries / 64 MiB, disk LRU of resized variants, single-flight per `key@size`, diagnostics counters, `GET /art/{key}?size=64\|128\|240\|640`) |
| `internal/skins` | domain only: `manager.go`, `types.go`, `validate.go`, `css.go`, `layout.go`, `zip.go`, `asset.go` |
| `internal/settings` | `settings.go` (schema version, skins and engine sections, atomic store) |
| `internal/wailsapi/audio` | `service.go` (startup/shutdown/emitter/browser + delegated catalog methods), `types.go` aliases |
| `internal/wailsapi/player` | `service.go` (500 ms ticker, mutations, emit), `types.go` aliases |
| `internal/wailsapi/skins` | `service.go` (dialogs/events/manager calls), `types.go` aliases |
| `internal/platform` | native window/GTK code, main-window resize and the dedicated Browse window host; the only non-adapter Wails boundary |

## Event flow

```text
engine.Supervisor emitter ──▶ audioapi.Service ──▶ Wails "audio:state"
                                                      │
playback.Provider ──▶ playerapi.Service ──▶ Wails "player:state"
      (500 ms ticker + mutations)                     │
                                                      ▼
skins.Manager emitter ──▶ skinsapi.Service ──▶ Wails "skins:changed"
                                                      │
                                                      ▼
                                   frontend/src/shared/api/events.ts
```

Event names and payload shapes are unchanged from v1:

- `audio:state` carries `engine.AudioState`.
- `player:state` carries `playback.PlayerState`.
- `skins:changed` carries `skins.SkinCatalog`.

The Wails adapters own application lifecycle and event emission; domain
services expose plain Go methods and never touch `application.App`.

## Browse window

The catalog has its own native window, separate from the compact player.

- `internal/platform.WindowService.OpenBrowse(section)` creates the window
  named `browse` on demand with a normal frame, 960×640 logical size,
  720×480 minimum and `/?surface=browse&section=<section>`. The section is
  validated (`search`, `library`, `podcasts`, `settings`; empty means
  `library`). An existing window is shown, focused and sent
  `browse:navigate` so it switches sections without reloading. `CloseBrowse`
  and `BrowseOpen` complete the host API.
- The compact window's `LOCAL` provider badge calls `openBrowse("library")`
  directly; the old in-window Library panel is no longer rendered. Its handlers
  and skin `open-library` actions route to the same Browse opener.
- Opening Browse hides the `main` window only after the Browse window is
  visible; the compact window is never destroyed, so playback and frontend
  state survive. Closing Browse (native close or the sidebar `Back to player`
  action) shows and focuses `main` again. A `WindowClosing` listener on Browse
  covers native X/Alt+F4/WM paths, and `CloseBrowse` restores `main` explicitly
  as a fallback. The hidden main window keeps the app alive while Browse is
  open; closing Browse restores it, and closing the main window quits normally.
- `frontend/src/main.ts` branches on `surface=browse` and dynamically imports
  the Browse entry, so the compact window never downloads the Browse chunk.
  `initI18n()` stays shared by both surfaces. The entry reads `section=` before
  mounting and starts the audio/music store.
- `frontend/src/app/browse/BrowseApp.svelte` is app-owned: sidebar, search
  field, section views and the now-playing bar use `shared/ui`, `shared/i18n`
  and the existing player/skin stores. It does not import `lib/components`.
  It subscribes to `browse:navigate`, and while `music.authenticated` is false
  it replaces catalog content with `EngineGate`; the sweep keeps pairing,
  engine-retry and copy/reset actions in Browse. Settings remains reachable
  before pairing.
- `frontend/src/app/browse/browse.svelte.ts` owns the Browse state: latest-wins
  search across the six groups, paged library tabs (playlists, albums, artists
  and Your Episodes), the entity back stack, retried `contextTracks` loading,
  save/follow/like state and playlist add/remove actions. Search, library,
  podcast and entity rendering live in `SearchView`, `LibraryView`,
  `PodcastsView`, `EntityView`, `NowPlayingBar` and `PlaylistPicker`.
- Long entity lists use `shared/ui/VirtualList` with a 44 px row pitch. The
  entity panel is `flex:1; min-height:0` and the virtual list owns the scroll
  container at `height:100%`, so the Browse window never nests scroll areas.
- `frontend/src/app/browse/settings.svelte.ts` and `SettingsView` load and save
  `EngineSettings` through the audio facade. Saving marks a restart as pending;
  `audio.restart()` only runs from the `Dialog` confirmation. Search and
  library tabs use `tablist`/`tab` semantics with roving keyboard focus, and
  async views expose skeleton, empty, error/retry and `aria-busy` states.
- `frontend/src/app/browse/theme.svelte.ts` probes the active skin in a hidden
  shadow host (declarative) or a hidden `.clarity[data-theme]` element
  (built-in), then applies the known token list inline to `.cadence-browse`.
  Re-application follows `skins:changed`, theme changes and cross-window
  `storage` events for `cadence.theme`.
- The compact skin `styles.css` is never injected into the Browse document. A
  skin may declare an optional `browse.css`; the manager exposes it as
  `SkinRuntime.browseStylesheetUrl` and the Browse app loads it as
  `<link id="cadence-browse-skin">` after the app-owned base stylesheet.

## Adding a catalog surface

1. Add the engine route and wire type in `internal/engine/rest.go` and
   `internal/engine/dtos.go` if it is new.
2. Add the use case to `internal/catalog/service.go`, including artwork URL
   mapping through `artwork.Cache.Register`.
3. Delegate the method from `internal/wailsapi/audio/service.go` and add any
   type alias to `internal/wailsapi/audio/types.go`.
4. Regenerate bindings from the repository root:

   ```bash
   wails3 generate bindings -clean=true -ts -i .
   ```

5. Add the typed facade method in `frontend/src/shared/api/audio.ts` and the
   model exports in `frontend/src/shared/api/types.ts` if needed.
6. Update `vi.mock` paths only when a new generated module is introduced. The
   facade names stay stable for stores and components.

## Library write surface

The catalog exposes typed write/read pairs over the engine's generic
collection routes: `SetLiked`/`IsLiked`, `SaveAlbum`/`IsAlbumSaved`,
`SaveShow`/`IsShowSaved`, `FollowArtist`/`IsArtistFollowed`,
`AddToPlaylist`/`RemoveFromPlaylist`, and `YourEpisodes`, which resolves the
user-scoped collection URI from `/status`. The patched engine serves
`POST /library/collection/write` and `POST /library/collection/contains` for
the `collection`, `artist` and `show` sets; the original `/library/liked`
and playlist routes keep their behavior. Saved shows use the `show` set, while
liked tracks and saved albums use `collection` and followed artists use
`artist`. The saved-shows listing UI is Phase 6.

## Engine settings

Engine playback and cache options persist in `settings.Config.Engine` under
`$XDG_CONFIG_HOME/cadence/config.json`. The audio adapter validates and saves
them through `Store.UpdateEngine`, then calls
`Supervisor.ApplyPlaybackConfig`; the running engine is never restarted
implicitly. New values are used by the next `Start`, or after the Browse
settings view calls `Restart` following the explicit dialog confirmation.

## Generated bindings

The generator writes services and models under
`frontend/bindings/cadence/internal/`:

- `wailsapi/audio` — `Service` plus alias models for `AudioState`,
  `EngineSettings`, `PlaylistPage`, `AlbumPage`, `ArtistPage`,
  `SearchResults`, `LibraryContext`
- `wailsapi/player` — `Service` plus alias models for `PlayerState`, `Device`,
  `QueueItem`
- `wailsapi/skins` — `Service` plus alias models for `SkinCatalog`,
  `SkinRuntime`, `InstallResult`
- `platform` — `WindowService`
- domain models: `engine/models.ts`, `catalog/models.ts`, `playback/models.ts`,
  `skins/models.ts`

`frontend/src/shared/api` is the only frontend folder allowed to import
`frontend/bindings/**` or `@wailsio/runtime`.
