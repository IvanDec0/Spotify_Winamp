# ADR 0001 — Module boundaries

- **Status:** Proposed
- **Date:** 2026-11-04
- **Phase:** 3 (planned)
- **Source:** [`../../plan/cadence-v2.md`](../../plan/cadence-v2.md) §4.1

## Context

`internal/golibrespot` currently mixes process supervision, HTTP transport,
domain mapping and Wails RPC in one package, and `MusicBrowser.svelte` carries
browsing UI that the v2 catalog needs to grow. The plan requires behavior-
preserving modularization before new features land.

## Decision

Adopt the package layout and dependency rule from plan §4.1:

- Dependency direction is `domain <- adapters <- app`; domain packages never
  import Wails.
- Target packages: `internal/app` (composition root, lifecycle, window, local
  HTTP mux), `internal/engine` (supervisor, config, typed REST client, `/events`
  client, raw DTOs), `internal/catalog`, `internal/playback`, `internal/library`,
  `internal/artwork`, `internal/settings`, `internal/skins/{domain,storage,assets,validate}`,
  `internal/i18n`, `internal/platform` and `internal/wailsapi`.
- Only `internal/wailsapi` imports Wails. Constructors return interfaces owned by
  the consumer. Blocking calls take `context.Context`. Errors are wrapped with
  `%w` and classified at the edge. One `log/slog` logger is used instead of
  `fmt.Println`.
- `internal/engine/rest.go` is the only place that knows engine routes.

## Consequences

- Phase 3 moves logic and tests without changing behavior; bindings are
  regenerated from the thin `wailsapi` adapters.
- `internal/golibrespot` is deleted only after the new packages carry its tests
  and the frontend uses the regenerated bindings.
- The split temporarily increases the number of packages and import paths, and
  needs a migration slice that keeps `go test` and generated bindings green.
- New features must land in the target packages, not in `internal/golibrespot`.
