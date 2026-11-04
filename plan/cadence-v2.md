# Cadence v2 — Architecture, Performance and Catalog Plan

> **Status:** Proposed. No production code changes yet.
> **Base commit:** `24f7f4c` on `feat/cadence-v2` (branched from `feat/go-librespot-http-proxy`).
> **Language policy:** technical artifacts and code stay in English. UI copy is externalized (see §7).
> **Last updated:** 2026-11-04

---

## 1. Executive summary

1. **The app already tracks go-librespot `master`.** The pinned build is
   `v0.10.6-0.20261102145552-522adcae8d65` (2026-11-02), only three commits behind
   today's `master` (`16b65bb4`) and *newer* than the latest tagged release
   (`v0.10.5`, 2026-10-16). Podcast episodes and Jam participation are already in
   this build. The upgrade is a small, safe delta — not the source of the missing
   features.
2. **The real performance bug is the polling design, not the engine.** Cadence polls
   the engine ~3 times per second and re-sends up to 50 queue items on every tick,
   while the engine already exposes a WebSocket event stream (`GET /events`) that the
   app never opens.
3. **Most "Spotify Web" browsing is reachable without new credentials.** Playlist,
   album, artist and show contents all resolve through the existing
   `GET /context/tracks?uri=...`. The gaps are search groups for shows/episodes,
   saved shows, and artist discographies.
4. **Architecture is the scaling bottleneck.** `internal/golibrespot` mixes process
   supervision, HTTP transport, domain mapping and Wails RPC in one package;
   `MusicBrowser.svelte` is 1,427 lines. Both must be split before features land.
5. **i18n is absent.** ~220-240 English literals in the frontend and ~180 in Go; the
   backend returns preformatted English prose. Introduce typed catalogs and stable
   error codes early, then migrate per module.

---

## 2. Current state (evidence)

### 2.1 Runtime flow

```
Svelte stores ──Wails RPC/events──▶ Go services ──loopback HTTP──▶ go-librespot
   music.svelte.ts (2 s poll)          player.Service (500 ms ticker)
   player.svelte.ts (event only)       golibrespot.Service (RPC + library)
                                       golibrespot.Runtime (process)
```

### 2.2 Measured design facts at `24f7f4c`

| Fact | Evidence |
|---|---|
| Idle engine traffic ≈ 2.5 req/s steady | `internal/player/service.go` 500 ms ticker calls `Provider.State()` → `GET /status`; `frontend/src/lib/music.svelte.ts:98` adds a 2 s `GetState`; `internal/golibrespot/provider.go` fetches `/context/tracks` only until the context is `Ready` (≈0.2 req/s extra while warming up or retrying) |
| Every 500 ms tick ships the full player state, queue included | `internal/player/service.go` `emit()`; queue capped at 50 items |
| No WebSocket client | no `/events` reference outside the vendored engine |
| Context lists render unvirtualized | `frontend/src/lib/MusicBrowser.svelte:417` renders every item; engine cap is `metadata.max_tracks: 800` in `internal/golibrespot/runtime.go` |
| Cover art is full-resolution, per-row, no coalescing | `internal/artcache/cache.go` (96 entries / 48 MB, 3 MB per image, no single-flight, no disk) |
| Frame loops never idle | `frontend/src/lib/components/Progress.svelte:76-84`; both visualizers draw every frame while mounted |
| Package boundaries leak | `golibrespot.Service` combines Wails RPC, library browsing, HTTP calls, pairing and artwork URL rewriting (`internal/golibrespot/service.go`) |
| No i18n infrastructure | no i18n dependency in `frontend/package.json`; `frontend/index.html` is `lang="en"` |
| Tests cover adapters, not domain flows | no test for `golibrespot/service.go`, `player/service.go`, `platform`, or `MusicBrowser` |

### 2.3 go-librespot capability reality

Confirmed against the vendored source and upstream `master`:

| Capability | State | Notes |
|---|---|---|
| Podcast episode playback | ✅ available | Episodes are first-class media; `ApiTrack` unifies tracks and episodes |
| Episode resume points | ✅ available, automatic | engine resumes and reports progress; **no API field exposes it** |
| Spotify Jam participation | ✅ available | engine handles incoming `set_options` modes and Jam transfers (`daemon/controls.go`, `daemon/player.go`) |
| JAM create/join API | ❌ does not exist | JAM is a Spotify Connect mode, not a public API |
| WebSocket state stream | ✅ available | `GET /events`, 15 event types, per-client 64-event queue |
| Library writes | ⚠️ partial | liked tracks + playlist add/remove exist; saved albums/followed artists need a generic collection endpoint |
| Search | ⚠️ partial | tracks/albums/artists/playlists only; no shows/episodes group decoded |
| Show/artist/album/playlist detail | ✅ via context | `GET /context/tracks?uri=spotify:{type}:{id}` |
| Artist discography / saved shows | ❌ no endpoint | needs a patch or an enrichment source |
| Playback speed for podcasts | ❌ not supported | `supports_playback_speed` not advertised, no command handler |
| `server.image_size` | ✅ available | `default`/`small`/`large`/`xlarge` |
| On-disk audio cache | ✅ available (off) | `cache.enabled`, `cache.size_limit` default `1GB` |
| MPRIS / media keys | ✅ available (off) | `mpris_enabled` default `false` |
| `device_is_group` | ✅ on `master` only | added 2026-10-28, not in the current pin |
| Web API passthrough | ❌ removed upstream | dropped in `v0.9.1` (`remove dead Web API passthrough`) |

**Decision:** the plan does not promise features the engine cannot expose. Podcast
*discovery* and richer library writes require two small additive patches (§4.4);
artist discographies and saved shows are flagged as gated work.

---

## 3. Goals and non-goals

### Goals

- Behavior-preserving modularization of Go and Svelte code before feature work.
- Event-driven state end to end; no fixed polling of the engine.
- Rich catalog browsing: search (tracks, albums, artists, playlists, shows,
  episodes), entity pages, paged library, podcasts, library write actions.
- Externalized, typed, runtime-switchable UI language (English first, Spanish second).
- Measurable performance gates, not impressions.
- A delivery chain that keeps every PR reviewable (≤400 authored changed lines).

### Non-goals (v2)

- Official Spotify client integration, Web Playback SDK, DRM, audio capture.
- Creating or joining Spotify Jams programmatically (not exposed).
- Spotify Web API enrichment by default (see D4; optional and feature-flagged).
- Playback speed, chapter navigation, audiobooks UI.
- A visual skin editor, native Winamp skin loading, executable skins.

---

## 4. Target architecture

### 4.1 Go package layout

Dependency rule: `domain ← adapters ← app`. Domain packages never import Wails.

```
main.go                          # flags only; calls app.Run
internal/
  app/                           # composition root, lifecycle, window, local HTTP mux
  engine/                        # go-librespot sidecar — no Wails imports
    supervisor.go                # start/stop/restart, health, lockfile, Pdeathsig
    config.go                    # typed config generation + validation
    rest.go                      # typed client, single-flight, retries, error mapping
    events.go                    # /events WebSocket, reconnect with backoff
    dtos.go                      # raw engine JSON
  catalog/                       # browsing use cases
    service.go                   # Search, Entity, ContextPage, LibraryPage
    entity.go                    # track/album/artist/playlist/show/episode mapping
    models.go                    # domain types, stable IDs, image refs
    paging.go
  playback/
    service.go                   # transport intents + snapshot state
    provider.go                  # PlayerProvider interface (engine, demo)
    state.go                     # revisioned snapshot
  library/
    service.go                   # like/save/follow, playlist add/remove
  artwork/
    cache.go                     # memory LRU + disk LRU + single-flight
    variants.go                  # resize 64/128/240/640
    http.go                      # /art/{key}/{size}
  settings/
  skins/                         # split by responsibility
    domain/  storage/  assets/  validate/
  i18n/                          # locale resolution for server-originated codes
  platform/
  wailsapi/                      # thin adapters; only package importing Wails
    app.go  catalog.go  player.go  skins.go  window.go
```

Conventions:

- Constructors return interfaces owned by the consumer, not the implementation.
- Every blocking call takes `context.Context`; shutdown is graceful and ordered.
- Errors are wrapped (`fmt.Errorf("...: %w", err)`) and classified once at the edge.
- One structured logger (`log/slog`) with a stable field set; no `fmt.Println`.
- `internal/engine/rest.go` is the only place that knows engine routes.
- A `diagnostics` package exposes counters (requests, events, payload bytes,
  reconnect count) behind a debug-only endpoint and `--diagnostics` flag.

### 4.2 Frontend layout

```
frontend/src/
  app/                 # boot, shell, window layout, route state machine
  features/
    player/            # transport, progress, volume, queue
    browse/            # home, search, entity pages, navigation stack
    library/           # playlists, albums, artists, shows, liked
    podcast/           # shows, episodes, resume affordances
    skins/             # runtime + manager UI
    settings/          # preferences, language, engine options
  shared/
    api/               # typed wrappers over generated bindings (only import site)
    ui/                # Button, Dialog, List, VirtualList, Skeleton, Toast
    i18n/              # catalogs, t(), formatters, locale store
    stores/            # session, preferences
    lib/               # pure helpers
```

Rules:

- Components never import `frontend/bindings/**` directly; they import `shared/api`.
- Feature folders do not import each other; shared code is the only cross-feature path.
- All user-visible text goes through `t()`; the ESLint guard enforces attributes and
  text nodes in `.svelte` files.
- One navigation store with a typed back stack replaces ad-hoc `open/close` flags.
- One app clock drives progress and visualizers; it runs only while playing and the
  window is visible.

### 4.3 Event and state contract

Engine event → domain snapshot → Wails event, revisioned:

```go
type Snapshot struct {
    Revision   uint64    // monotonic; frontend drops stale payloads
    Playback   Playback  // status, track, position, volume, repeat, shuffle
    Queue      *Queue    // nil unless the queue changed
    QueueRev   uint64
    Engine     EngineInfo // connected, version, authenticated
}
```

Rules:

- `/events` is the primary source; a 15 s `GET /status` reconciliation exists only to
  repair missed events after a reconnect.
- `/context/tracks` is fetched on demand (entity open), never on a timer.
- Coalesce bursts with a 50 ms debounce; emit only when the snapshot changes.
- Position is sent as `position_ms` + `captured_at`; the frontend interpolates locally
  at 10 Hz instead of asking the engine.
- Queue state is fetched when the queue panel opens and kept current by queue events.

### 4.4 Engine pin and patch strategy

- Upgrade the pin from `522adcae8d65` to `master` (`16b65bb4`) to pick up
  `device_is_group`, `accesspoint_ports`, and the station-page fix.
- Replace free-form patch maintenance with a **pinned fork branch** in the build
  script, or keep additive patches but add a CI `--check` mode that verifies every
  patch applies cleanly to the pinned tree. Recommended: keep patches (small, easy to
  review) plus the CI check; revisit a fork only if patch count exceeds ~4.
- Patch budget: at most two new additive patches in v2.
  1. `search-groups`: decode `shows` and `episodes` groups in
     `spclient/searchview.go` and pass them through `/search`.
  2. `library-collection`: a generic collection write/contains route so albums,
     artists and shows can be saved/followed, not only tracks.
- Every engine-facing feature must degrade gracefully when a route is missing
  (capability negotiation at startup, feature flags in the catalog service).

---

## 5. Performance workstream

| Symptom | Root cause | Fix | Expected effect |
|---|---|---|---|
| UI feels laggy while playing | 500 ms full-state emission + 2 s duplicate poll | `/events` WebSocket; revisioned diffs; remove ticker and poll | Idle engine traffic ~2.5 req/s → ~0.07 req/s; IPC payload 15-30 KB/tick → <4 KB/change |
| Slow scroll in long playlists | 800 rows × cover images, unvirtualized | `VirtualList` + paged library (50/page) + lazy images | Constant DOM size; 60 fps with 800 items |
| Memory and network growth on cover art | full-size covers, per-row fetch, no coalescing, memory only | size variants + single-flight + disk LRU (`xdg.CacheHome`) + `Cache-Control` retained | 40-70% fewer image bytes; no duplicate downloads; survives restart |
| CPU while paused | rAF loops always running | single clock, stop on pause/hidden, 10 Hz text updates | Near-zero idle CPU from UI |
| Duplicate library calls | every action re-fetches | TTL cache + explicit invalidation after writes | Fewer engine round-trips |
| Playlist open blocks on enumeration | `/context/tracks` cold | request once, show progressive readiness, cancel on navigate | Perceived latency drops; no timer retries |
| Engine restart churn | config rewritten with broad settings | typed config + only restart on explicit settings change | Fewer interruptions |

Measurement plan (P0):

- `--diagnostics` flag + `/debug/stats` counters: engine requests by route, event rate,
  bytes emitted, WS reconnects, artwork hits/misses/bytes, goroutines.
- Documented baseline: idle CPU/RSS, request rate, frame budget on a 800-track list.
- `go test -bench` for catalog mapping and artwork variant selection.
- Playwright-free manual checklist for frame timing; Chrome DevTools for the webview.

Acceptance gates for the performance phase:

- Idle request rate ≤ 0.2 req/s.
- Wails event payload ≤ 4 KB per change; no event while paused and unchanged.
- 800-item list scrolls without dropped frames on the reference machine.
- Idle CPU < 2% and RSS stable over 10 minutes.

---

## 6. Catalog and product features

### 6.1 Capability matrix

| Feature | Engine support | Work |
|---|---|---|
| Home / recently played | ❌ no endpoint | Out of scope for v2; "Continue" uses current context |
| Search tracks/albums/artists/playlists | ✅ | UI + paging of groups (limit ≤ 50) |
| Search shows/episodes | ⚠️ needs `search-groups` patch | Spike: capture raw `/search` JSON for a podcast query; add groups |
| Playlist page | ✅ context | Entity page + metadata from library/search |
| Album page | ✅ context | Entity page, play, queue |
| Artist page | ⚠️ context resolves; top-tracks semantics unverified | Ship top tracks + "radio" first; discography requires patch/enrichment |
| Show page / episodes | ✅ context | Episode list, resume, play |
| Your Episodes | ✅ context (`...:collection:your-episodes`) | Library tab |
| Saved shows | ❌ no endpoint | Gated on D4 or a third patch |
| Like / unlike track | ✅ `POST /library/liked` | UI + optimistic state |
| Save album / follow artist | ⚠️ needs `library-collection` patch | Generic collection write |
| Playlist add/remove | ✅ | Track context menu |
| Queue view/edit | ⚠️ add only over REST | Add to queue + show queue; reorder needs dealer commands (out of scope) |
| Jam participation | ✅ | `device_is_group: true`; explain "join from Spotify" in settings |
| Local party mode | ❌ not built | Optional v2.1; local HTTP page + guest queue |

### 6.2 Browse surface (decision D1)

Two viable surfaces:

- **Option A — expand the compact window in "Browse mode".** One window, no new
  process surface, but fights the shaped/transparent skin window and the 420 px canvas.
- **Option B — separate Browse window (recommended).** A normal Wails window
  (e.g. 960×640) that hosts search, entity pages, library and podcasts; the compact
  skin window stays untouched. Thematically consistent with Winamp's separate Media
  Library window and isolates heavy UI from the skin runtime.

Phases 0-5 are independent of D1; only Phase 6 needs the answer.

### 6.3 Podcasts in detail

- Search shows/episodes; show page lists episodes via `spotify:show:{id}`.
- Playback uses the existing `player/play` with an episode URI; the engine handles
  resume automatically.
- UI shows episode duration, release date, and "resume" badge when the engine reports
  a position. **Limitation:** the REST API does not expose the stored resume point, so
  the UI cannot render an exact progress bar before playback starts. Documented as an
  engine limitation; patch only if it becomes a must.
- "Your Episodes" tab via the collection context.

---

## 7. Internationalization

- **Catalogs:** ICU MessageFormat JSON at `frontend/src/shared/i18n/locales/{en,es}.json`.
- **Runtime:** `intl-messageformat` for ICU, `Intl` for numbers, dates, relative time.
- **Typed keys:** a generator reads `en.json` and emits `messages.d.ts`, so a missing
  key is a TypeScript error. No string-key typos at runtime.
- **Loading:** active locale loaded eagerly; others via dynamic `import()`.
- **Selection:** first run uses `navigator.language` matched against available
  locales; persisted in preferences; switchable in Settings; updates `document.lang`.
- **Go is not localized.** Backend returns `{code, params}`; the frontend renders
  `errors.<code>`. Migrate `AudioState.Error`, `SkinCatalog.LastError`, validation
  warnings and pairing messages to codes.
- **Skins:** optional `i18n` map in `manifest.json`; fallback to the raw label.
- **Guards:** ESLint rule for literal `aria-label`/`title`/`placeholder`/`alt` and text
  nodes in `.svelte`; a test that asserts key parity and no empty values.
- **Migration:** infrastructure in Phase 2; per-module migration as modules are
  touched; final sweep gate before release (Phase 7). New code must use `t()` from day
  one.

---

## 8. Delivery roadmap

Delivery strategy: `ask-on-risk` recorded; because the forecast exceeds 400 authored
lines, the recommended chain is **feature-branch-chain** with tracker branch
`feat/cadence-v2` (D2, confirm before the second PR). Every slice keeps tests and docs
with its behavior; every PR states start, end, dependencies and rollback.

### Phase 0 — Baseline and safety net (~250 lines)

- Diagnostics counters, `--diagnostics` flag, `/debug/stats`.
- Baseline document: idle CPU/RSS, request rate, event payload, list scroll.
- CI: verify engine patches apply; build the engine once and cache it; add `task verify`.
- ADRs: event model, error codes, module boundaries.

**Gate:** baseline recorded; CI green; no behavior change.

### Phase 1 — Engine upgrade and configuration (~150 lines)

- Pin `master@16b65bb4`; add `device_is_group`, keep `accesspoint_ports` documented.
- Choose the engine `server.image_size` variant from measured pixel dimensions (the
  enum is `default`/`small`/`large`/`xlarge`) and use it as the source for local
  variants; keep `metadata.max_tracks` bounded.
- Surface engine version in diagnostics; document the delta.

**Gate:** pairing + playback + library verified; engine version visible.

### Phase 2 — Frontend foundation (~900 lines, 2 slices)

- `shared/api` wrapper (typed, error-normalizing) over generated bindings.
- i18n infrastructure + `en` catalog for shared primitives.
- `shared/ui`: Button, Dialog, List, VirtualList, Skeleton, Toast.
- Navigation store + shell skeleton; no visual redesign yet.

**Gate:** existing screens render through the new shell; `task check`/`test` green.

### Phase 3 — Go decomposition (~1,500 lines, 3 slices)

- `engine` package: supervisor, config, REST, events (no behavior change).
- `catalog`, `artwork`, `library`, `playback` packages; move logic and tests.
- `wailsapi` adapters; regenerate bindings; delete `internal/golibrespot`.

**Gate:** all Go tests green; behavior identical; bindings regenerate cleanly.

### Phase 4 — Event-driven state and artwork (~900 lines, 2 slices)

- `/events` client, snapshot state machine, revisioned Wails events.
- Remove the 500 ms ticker and the 2 s poll; add 15 s reconciliation.
- Artwork variants, single-flight, disk LRU; frontend lazy images.
- Single app clock; pause rAF when idle.

**Gate:** performance gates from §5 met; no duplicate state updates.

### Phase 5 — Catalog backend (~1,100 lines, 2-3 slices)

- `search-groups` patch + capability detection; shows/episodes in `/search`.
- Entity pages through `/context/tracks`; paged library; "Your Episodes".
- `library-collection` patch; like/save/follow, playlist add/remove.
- Engine settings surface: bitrate, crossfade, normalization, disk cache, FLAC, MPRIS,
  `device_is_group` (applied on explicit restart).

**Gate:** every feature degrades gracefully when a route is absent; write actions
verified against a real account.

### Phase 6 — Browse UI (~2,000 lines, 3-4 slices)

- Browse surface (per D1), search with grouped results, entity pages, library tabs,
  podcast pages, track context menu, empty/error/skeleton states.
- Virtualized lists, keyboard navigation, focus management, a11y labels via `t()`.

**Gate:** full manual pass on a real account; no hardcoded strings in new UI; frame
budget holds.

### Phase 7 — i18n sweep, tests and polish (~800 lines, 2 slices)

- Migrate remaining literals; Spanish catalog complete; key parity test.
- Store/component tests for the new flows; error-code mapping tests.
- Documentation: architecture, catalog, i18n, engine patches.

**Gate:** `rg` shows no user-facing literals outside catalogs; locale switch verified.

### Phase 8 — Packaging and release (~300 lines)

- AppImage/tar.gz include the new engine; version stamping; release notes.
- Clean-machine install test; uninstall path; cache location documented.

**Gate:** release artifact runs on a clean Arch install.

---

## 9. Risks and mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Engine upgrade breaks patches | Build failure | CI apply check; additive patches only; fork escape hatch |
| Search view lacks show/episode groups | Podcast search delayed | Spike in Phase 5; fallback to Web API enrichment (D4) or name search |
| Wails bindings churn during refactor | Frontend breakage | `shared/api` isolates bindings; regenerate in one slice |
| `context/tracks` truncates at 800 | Long playlists incomplete | Document; page-aware UI copy; consider engine paging later |
| Episode resume position not exposed | Limited podcast UI | Document; optional patch if required |
| No public Jam API | Feature expectation mismatch | Ship device-level Jam participation; party mode optional |
| Large refactor stalls | Delivery risk | Feature-branch chain, behavior-preserving slices, gates per phase |
| Token/credential leakage | Security | Tokens never cross to the frontend; Web API (if enabled) stays in Go |

---

## 10. Decisions recorded for later

- **D1 (blocks Phase 6):** Browse surface — separate window (recommended) vs expanded
  compact window.
- **D2 (before PR #2):** chain strategy — `feature-branch-chain` (recommended) vs
  `stacked-to-main`.
- **D3 (Phase 1):** keep additive patches (recommended) vs pinned fork.
- **D4 (Phase 5):** optional Web API enrichment for artist discographies and saved
  shows. Default: no. Enable only if the engine cannot expose the data and the user
  accepts the extra terms/rate-limit surface.
- **D5 (post-v2):** local party mode (guest queue page) vs Jam participation only.

---

## 11. Definition of done

- Go: domain packages have no Wails imports; >70% coverage on catalog, playback state,
  artwork and engine client; `go vet` and tests green.
- Frontend: no direct binding imports outside `shared/api`; feature folders isolated;
  `svelte-check`, ESLint, Prettier and Vitest green.
- Performance gates from §5 met and recorded in the baseline document.
- `en` and `es` catalogs complete, key parity enforced in CI.
- Engine pin documented; patches apply cleanly in CI; version visible at runtime.
- Every phase ships with docs and a rollback note.

---

## Appendix A — Reference points at `24f7f4c`

- Engine pin: `scripts/build-go-librespot.sh:5`.
- Engine config: `internal/golibrespot/runtime.go` (`writeConfig`).
- REST surface: `internal/golibrespot/client.go`; patched routes in
  `build/patches/go-librespot-search-radio.patch`.
- State ticker: `internal/player/service.go` (`emitInterval`, `startTicker`).
- Duplicate poll: `frontend/src/lib/music.svelte.ts:98`.
- Context refresh: `internal/golibrespot/provider.go` (5 s refresh).
- Artwork: `internal/artcache/cache.go`, `main.go` (`/art/` route).
- Largest UI units: `frontend/src/lib/MusicBrowser.svelte` (1,427),
  `frontend/src/lib/skins/SkinManager.svelte` (712), `internal/skins/manager.go` (844).
- Generated bindings: `frontend/bindings/cadence/**`.
- Engine event stream: `build/audio/go-librespot-src/daemon/api_server.go`
  (`/events`), `daemon/api_gen.go` (routes).
- Engine search: `build/audio/go-librespot-src/spclient/searchview.go`.
- Engine library writes: `.../spclient/library_write.go`,
  `.../daemon/api_server.go`.
