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
| `internal/wailsapi/audio` | `service.go` (startup/shutdown/emitter/browser + delegated catalog methods), `types.go` aliases |
| `internal/wailsapi/player` | `service.go` (500 ms ticker, mutations, emit), `types.go` aliases |
| `internal/wailsapi/skins` | `service.go` (dialogs/events/manager calls), `types.go` aliases |
| `internal/platform` | native window/GTK code; the only non-adapter Wails boundary |

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

## Generated bindings

The generator writes services and models under
`frontend/bindings/cadence/internal/`:

- `wailsapi/audio` — `Service` plus alias models for `AudioState`,
  `PlaylistPage`, `AlbumPage`, `ArtistPage`, `SearchResults`, `LibraryContext`
- `wailsapi/player` — `Service` plus alias models for `PlayerState`, `Device`,
  `QueueItem`
- `wailsapi/skins` — `Service` plus alias models for `SkinCatalog`,
  `SkinRuntime`, `InstallResult`
- `platform` — `WindowService`
- domain models: `engine/models.ts`, `catalog/models.ts`, `playback/models.ts`,
  `skins/models.ts`

`frontend/src/shared/api` is the only frontend folder allowed to import
`frontend/bindings/**` or `@wailsio/runtime`.
