# ADR 0001 — Module boundaries

- **Status:** Accepted
- **Date:** 2026-11-04
- **Phase:** 3 (implemented)
- **Source:** [`../../plan/cadence-v2.md`](../../plan/cadence-v2.md) §4.1
- **Companion:** [`../architecture.md`](../architecture.md)

## Context

`internal/golibrespot` used to mix process supervision, HTTP transport, domain
mapping and Wails RPC in one package, while `internal/player` mixed the
provider-neutral player model with the Wails ticker service. `MusicBrowser.svelte`
carries browsing UI that the v2 catalog needs to grow. The plan requires
behavior-preserving modularization before new features land.

## Decision

Adopt this package layout and dependency rule:

```text
main.go                     composition root
internal/
  engine/                   supervisor, typed REST client, raw engine DTOs
  catalog/                  library/search/context orchestration and models
  playback/                 Provider, PlayerState, DemoProvider, EngineProvider
  artwork/                  bounded album-art proxy and diagnostics counters
  skins/                    domain only (manager/types/validate/css/layout/zip/asset)
  settings/                 user settings store
  diagnostics/              counters and transport instrumentation
  platform/                 native window/GTK; the only non-adapter Wails boundary
  wailsapi/
    audio/                  package audioapi
    player/                 package playerapi
    skins/                  package skinsapi
```

- Dependency direction is `domain <- adapters <- app`; domain packages never
  import Wails.
- Only `internal/wailsapi/**` and `internal/platform` import
  `github.com/wailsapp/wails/v3/pkg/application`. `platform` keeps that exception
  because resizing a mapped GTK window needs native window calls that Wails does
  not expose correctly.
- Type ownership:
  - `engine`: `Root`, `Track`, `Status`, `Pairing`, `AudioState`, `authCode`,
    `valueResponse`, `ContextTrack`, `ContextTracks`, and the library/search wire
    DTOs (`LibraryPlaylist`, pages, `SearchHit`, `SearchGroup`, `SearchResults`).
    `engine.Name` is the disabled/demo engine label.
  - `catalog`: service orchestration, artwork key mapping, `ContextTrackInfo` and
    `LibraryContext`. It returns engine wire DTOs directly where the shape is
    already frontend-facing; it does not duplicate them.
  - `playback`: `Provider`, `PlayerState`, `Track`, `QueueItem`, `Device`,
    `Status`, `RepeatMode`, `PlayerError`, `DemoProvider`, `EngineProvider`.
  - `wailsapi/{audio,player,skins}`: service structs plus `types.go` aliases for
    every type used in service signatures.
- Constructors return interfaces owned by the consumer. Blocking calls take
  `context.Context`. Errors are wrapped with `%w` and classified at the edge.
  One `log/slog` logger is used instead of `fmt.Println`.
- `internal/engine/rest.go` is the only place that knows engine routes.

## Consequences

- Phase 3 moved logic and tests without changing behavior; bindings are
  regenerated from the thin `wailsapi` adapters.
- `internal/golibrespot`, the old `internal/player` adapter package and
  `internal/artcache` are deleted. `git log --follow` still traces the moved
  files.
- The split increases the number of packages and import paths, and requires one
  binding regeneration plus a frontend facade update; the `shared/api` facade
  keeps stores and components stable.
- New features must land in the target packages. `engine` must not import
  `catalog`, and domain packages must not import Wails.
