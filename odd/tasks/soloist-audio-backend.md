# Spotify Soloist audio backend (opt-in)

## Objective

Add Spotify Soloist as an opt-in playback backend for Premium accounts where
go-librespot cannot retrieve audio keys, while keeping go-librespot for library,
search, context, queue metadata and catalog authentication. The default backend
stays go-librespot; Soloist is selected explicitly, applied at startup, and can
be reverted without data loss.

## Problem

Cadence's pinned go-librespot fails on some Premium accounts with:

```text
failed retrieving audio key: failed retrieving aes key with code 1
skipping track: Spotify refused the audio key (code 1)
```

`code 1` is a server-side refusal of the account's legacy AES key request, not
a local configuration error. Retry/backoff only helps `code 2`; bitrate, state,
re-authentication, version, DNS, CDN and DJ changes do not help. The same
account and machine play correctly through Spotify's official player, Soloist
(`crmne/spotifast#283`). Soloist has no library/search API and the public Web API
is not a viable replacement, so the only correct shape is hybrid: go-librespot
keeps the catalog session and Soloist owns audio.

## Evidence

- Cadence reproduces `code 1` persistently; `PlayPlay` is only a stub in the
  pinned engine (`playplay/impl/impl.go`).
- `krolebord/spotifast-soloist` ships a tested Linux/macOS/Windows hybrid of
  librespot metadata plus Soloist audio; its notes are closed as completed
  (2026-11-02).
- Spotify documents Soloist's CLI, WebSocket API, downloads and 90-day build
  expiry at `developer.spotify.com/documentation/soloist`.
- Soloist is never redistributable; Cadence must download it from Spotify at the
  user's request and link to the official page.
- Reference version output:
  `soloist 1.3.10.12 build 1792756912 (20261023) (g19412df6ea) (...)`.
- Soloist WebSocket commands: `play` (optional `uri`), `pause`, `skip_next`,
  `skip_prev`, `seek`, `set_volume`, `set_shuffle`, `set_repeat_context`,
  `set_repeat_track`, `add_to_queue`, `activate`, `deactivate`, plus queries
  `get_auth_state`, `get_state`, `get_queue`.
- `play.uri` accepts track, album, playlist and episode URIs; the reference
  implementation also passes artist and `spotify:user:me:collection` contexts.
  `add_to_queue.uri` accepts only track URIs. There is no offset/position on
  `play`; `seek` can be dropped while a track is still loading.

## Scope

In:

- Persisted `playback.backend` setting (`librespot` default, `soloist` opt-in),
  applied at startup; no runtime provider hot-swap.
- `internal/soloist`: WebSocket client, protocol types, version/expiry parsing,
  credential store, installer, process supervisor and facade.
- `internal/playback/soloist_provider.go`: `playback.Provider`,
  `playback.Streamer` and the catalog playback methods.
- `catalog.Playback` consumer-side seam so `PlayContext`, `PlayTrack`,
  `AddToQueue` and `StartRadio` reach the active backend.
- Additive `AudioState` fields and a merged `audio:state` event.
- Settings UI: backend selector, Soloist install/update, API key, status,
  expiry warning, feature gating and localized errors.
- Documentation, ADR, unit/integration tests and a manual E2E checklist.

Out:

- Bundling or redistributing Soloist archives/binaries.
- Spotify Web API, Web Playback SDK or DRM.
- go-librespot fork changes.
- Runtime backend switching; changing the backend requires an app restart.
- Spotify radio in Soloist mode: it needs a streaming-session radio resolver
  that go-librespot does not expose. The UI hides the action in Soloist mode.
- Queue reorder/remove beyond `add_to_queue`: Soloist's local API only appends
  tracks and cannot clear or reorder.
- go-librespot-only options in Soloist mode: bitrate, FLAC, normalisation,
  crossfade, cache, MPRIS and device group. Soloist uses its own defaults.
- Automatic background updates; v1 warns at 14 days and offers a manual update.

## Constraints

- Primary target is Linux x86_64. The installer also maps Spotify's published
  arm64/arm32 Linux, Apple Silicon macOS and Windows x86_64 archives.
- No new Go dependencies; reuse `github.com/coder/websocket v1.8.14`.
- Soloist binds only to `127.0.0.1`; use `--ws 127.0.0.1:0` and read
  `<data-dir>/ws.addr` and `<data-dir>/ws.port`.
- The API key lives in a `0600` file outside `config.json`, is passed to Soloist
  through argv (unavoidable), and is never logged or returned to the frontend.
- Builds expire 90 days after the build timestamp; expired builds exit `10`.
- Two Connect devices exist in Soloist mode: `Cadence` (Soloist) and
  `Cadence Library` (go-librespot). go-librespot MPRIS and audio cache are off.
- `settings.SchemaVersion` stays `1`; only additive optional fields are added,
  because `Load` rejects any other schema version.
- Technical artifacts are in English.

## Architecture

```text
main.go
  settings.Store ── playback.backend ──┐
                                       ├─ playback.Provider ── player.Service
  engine.Supervisor (library/auth) ────┘
  soloist.Supervisor (audio) ── soloist.Provider
  catalog.Service ── catalog.Playback ── active provider
  wailsapi/audio.Service ── merged AudioState + Soloist actions
```

- `engine` keeps owning `AudioState` and gains `PlaybackBackend` and
  `Soloist *SoloistState` (frontend DTO). `soloist.State` is the internal
  process/protocol state; the audio adapter maps it.
- `playback.Provider` is unchanged. The Soloist provider also implements the
  four catalog playback methods structurally; `catalog` declares the interface
  it consumes, so no package cycle is introduced.
- In librespot mode, `catalog` uses an internal thin wrapper over the existing
  supervisor client, preserving today's behavior exactly.
- In Soloist mode, `main` injects the Soloist provider into both the player
  service and catalog. The go-librespot provider is not constructed.
- One `audio:state` event carries both engines; one `player:state` event comes
  from the active provider. No new event names.
- Backend selection is read once at startup. The settings UI marks a backend
  change as requiring a Cadence restart; the existing engine restart button is
  not reused for this.

### Capability matrix

| Capability | librespot | Soloist |
|---|---|---|
| Library, search, context, metadata | go-librespot | go-librespot (unchanged) |
| Audio playback | go-librespot | Soloist |
| Play context | yes | yes (URI; no offset) |
| Play track | yes | yes |
| Add to queue | yes | yes (track URIs only) |
| Radio | yes | no (UI gated) |
| Seek | yes | yes (retry while loading) |
| Volume / shuffle / repeat | yes | yes |
| Bitrate / FLAC / normalisation / crossfade | yes | Soloist defaults; controls disabled |
| Cache | yes | Soloist-managed; control disabled |
| MPRIS | optional | no |
| Visualizer | system capture | system capture (unchanged) |
| Connect device | `Cadence` | `Cadence` + `Cadence Library` |

## Design decisions

- **Hybrid, not replacement.** Soloist cannot browse; go-librespot cannot play
  on affected accounts. Both stay.
- **Startup selection, not hot-swap.** The provider, catalog router, process
  lifecycle and generated engine config all depend on the backend. A restart is
  predictable; runtime swapping adds races for little benefit.
- **Versioned installs.** Download into `programs/<build-unix>/soloist` and start
  the newest build. A running Soloist is never overwritten; an update applies on
  the next start.
- **Key outside JSON.** A dedicated `0600` file avoids leaking the key in
  settings exports and keeps `config.json` shareable.
- **No undocumented prefs.** Do not write Soloist's internal prefs file for
  bitrate/normalisation in v1. Disable those controls and use Soloist defaults.
- **No radio.** go-librespot resolves radio through its private session and
  plays it locally; the resolved context is not exposed. Hide the action rather
  than fake it.
- **Track fallback for context offset.** `play` has no offset. If the user
  clicks a track inside a context, play that track URI standalone. Document the
  lost context.
- **Library device is clearly named.** `Cadence Library` signals that it is the
  catalog engine, not the playback device.

## Tasks

### Phase 0 — Spike and contract capture (manual gate)

- [ ] P0.1 On the failing PC, download the official Soloist archive, generate an
      API key in the Spotify developer dashboard, and start:
      `soloist -n Cadence -k <key> -D <data> -C <cache> -w 127.0.0.1:0`.
- [ ] P0.2 Confirm `auth_state.logged_in=true` and that a track plays audibly.
- [ ] P0.3 Capture `soloist --version`; verify the parser against:
      `soloist <version> build <unix-seconds> (<yyyymmdd>) (...)`.
- [ ] P0.4 Capture `auth_state`, `playback_state` (idle/playing/paused/buffering),
      `queue_changed`, `position_sync`, `options_changed`, `volume_changed`,
      `command_result` and `error` payloads.
- [ ] P0.5 Record which URIs `play` accepts: track, album, playlist, artist,
      `spotify:user:<user>:collection`, episode, show, `spotify:station:*`.
- [ ] P0.6 Verify `seek` immediately after `play` (does it drop?), `set_volume`,
      both repeat commands, `activate`, and `add_to_queue` with a track.
- [ ] P0.7 Confirm the failing account plays through Soloist. If it does not,
      STOP: the plan is not viable for that account.

Gate: P0.2 and P0.7 must pass. P0.5 decides whether artist/collection playback
is advertised or gated. Record results in this document before Phase 1.

### Phase 1 — `internal/soloist` foundation

- [x] P1.1 `protocol.go`: command/event structs, entity envelope decoding,
      constants and coded errors.
- [x] P1.2 `client.go`: one loopback WebSocket connection, read loop, command
      and query helpers, event waiters, `command_result`/`error` handling,
      read limit, timeouts, reconnect-on-close semantics.
- [x] P1.3 `binary.go`: discovery order, `--version` parser, 90-day expiry,
      cache by binary path + mtime.
- [x] P1.4 `credentials.go`: read/save/clear the API key with `0600` and atomic
      writes; never log it.
- [x] P1.5 `install.go`: platform/archive map, HTTPS download with redirect
      allowlist (`*.spotifycdn.com`, `*.spotify.com`), 64 MB archive and 128 MB
      binary caps, safe tar.gz/zip extraction of only the executable,
      `--version` validation, versioned install and atomic rename.
- [x] P1.6 `supervisor.go`: start/stop/restart, dirs, stale pid cleanup on Unix,
      `ws.port` polling, auth wait, exit-code 10 mapping, emitter, log file
      without argv.
- [x] P1.7 `service.go`: facade used by `wailsapi/audio` (state/start/stop/
      restart/install/key/emitter).
- [x] P1.8 Tests: version/expiry, credentials permissions, safe extraction,
      archive selection, WebSocket commands/queries, supervisor lifecycle with a
      fake helper binary and fake WS server.

Phase 1 API notes for later phases:

- `Service.Options.Installer *Installer` is the test seam; production leaves it
  nil and gets the host-allowlisted installer. `InstallerOptions.AllowAnyHost`
  is test-only and must never be set in production wiring.
- `PlaybackOptions.Modes` is `map[string]json.RawMessage` so a non-string mode
  value cannot make an entire `playback_state` frame undecodable.
- `Client.Do(..., "command_result", ...)` requires the reply to name the same
  command; an unnamed `command_result` does not satisfy a filtered waiter.
- `Supervisor.Start` is single-flight, resets its diagnostic tail per run, and
  `Stop` clears error fields. A pairing result (`CodePairingRequired`) leaves the
  process running and must be rendered as a status, not a hard failure.
- `Service.State()` may return the cached binary while another goroutine
  refreshes it; it never blocks on the 15 s `--version` probe.

### Phase 2 — Settings, state and wiring

Execution order is P3 → P4 → P2: `main.go` cannot select a Soloist
`playback.Provider` before it exists, and catalog playback needs the Phase 4
seam before the wiring can be coherent. P2 closes the loop with settings, the
additive `AudioState`, the Wails adapter, `main.go` and the ADR.

- [x] P2.1 `settings.PlaybackState` (`backend`), constants, default,
      normalization and `UpdatePlayback`; `Load` fills a missing section; no
      schema bump.
- [x] P2.2 `engine.AudioState` additive fields: `PlaybackBackend` and
      `Soloist *SoloistState`.
- [x] P2.3 `wailsapi/audio`: `PlaybackSettings`, get/set backend, merged
      `GetState`, Soloist emitter, startup/shutdown, install/key/dashboard
      methods and a `SoloistBackend` interface for tests.
- [x] P2.4 `main.go`: credentials/supervisor/service construction, provider
      selection, catalog playback injection, `Cadence Library` naming, MPRIS and
      cache forced off in Soloist mode.
- [x] P2.5 ADR `docs/adr/0004-dual-audio-backend.md`.
- [x] P2.6 Tests: settings round-trip/normalization and audio service merge/
      backend selection with a fake Soloist backend.

### Phase 3 — Soloist playback provider

- [x] P3.1 `internal/playback/soloist_provider.go`: `Provider`, `Streamer` and
      the four catalog playback methods.
- [x] P3.2 State mapping: status, track/artists/album/cover/duration, progress
      anchor, volume, shuffle, repeat, device, `canSeek`/`canControl`, errors.
- [x] P3.3 Queue mapping: current + upcoming from `get_queue`, cap 50,
      current/next flags.
- [x] P3.4 Commands: activate-before-play, play/pause/toggle/next/prev/seek/
      volume/shuffle/repeat/transfer; wait for matching events with bounded
      timeouts; two-command repeat sequences; track-only `add_to_queue`; radio
      unsupported; standalone track for `skipToURI`.
- [x] P3.5 Stream: auth state, initial snapshot, dirty refresh every 100 ms,
      position sync, return on close so the player service reconnects.
- [x] P3.6 Tests with a fake Soloist WS server: mapping, commands, event waits,
      errors and reconnection.

### Phase 4 — Catalog playback routing

- [x] P4.1 Add `catalog.Playback` and `Options.Playback`; default to an internal
      librespot wrapper that preserves current behavior.
- [x] P4.2 Route `PlayContext`, `PlayTrack`, `AddToQueue`, `StartRadio` through
      the seam; keep trim/validation behavior.
- [x] P4.3 Return the Soloist unsupported error for radio.
- [x] P4.4 Tests: default wrapper, injected fake, radio error.

### Phase 5 — Frontend and UX

- [x] P5.1 Regenerate Wails bindings with `task common:generate:bindings`.
- [x] P5.2 Add new types and `audio.ts` facade methods.
- [x] P5.3 Normalize `playbackBackend` and `soloist` in `music.svelte.ts`.
- [x] P5.4 Extend `settings.svelte.ts` with backend, Soloist actions and
      notices.
- [x] P5.5 `SettingsView.svelte`: Audio backend card (selector + restart notice)
      and Spotify Soloist card (status, version/expiry, install/update, key,
      dashboard). Disable go-librespot-only controls in Soloist mode.
- [x] P5.6 Gate radio actions in `EntityView.svelte` and `SearchView.svelte` when
      the backend is Soloist.
- [x] P5.7 Add `en.ts`/`es.ts` keys for every new string and error code.
- [x] P5.8 Tests: settings store/view, music normalization, radio gating.

### Phase 6 — Documentation

- [x] P6.1 Finish ADR 0004.
- [x] P6.2 `docs/spotify-setup.md`: Soloist section (why, key, install, switch,
      devices, limits, expiry, logs, paths).
- [x] P6.3 `docs/installation.md`: Soloist is never bundled; runtime download
      and paths.
- [x] P6.4 `docs/architecture.md`: backend seam and package ownership.
- [x] P6.5 `README.md`, `docs/README.md`, `docs/i18n.md`.
- [x] P6.6 `docs/development.md`: `CADENCE_SOLOIST` override and fake WS test
      conventions.

### Phase 7 — Verification and delivery

- [x] P7.1 Run `task check`, `task test` and `task verify`.
- [ ] P7.2 Manual E2E on the failing PC: install from Settings, save key, switch
      backend, restart, pair go-librespot, play track/album/playlist/artist/
      Liked Songs, seek, volume, shuffle/repeat, queue, visualizer and every
      error state.
- [x] P7.3 Work-unit commits on the current feature branch; no PR unless
      requested.
- [x] P7.4 Record verification evidence here and refresh the Engram mirror.

## File map

New:

- `internal/soloist/protocol.go`
- `internal/soloist/client.go`
- `internal/soloist/binary.go`
- `internal/soloist/credentials.go`
- `internal/soloist/install.go`
- `internal/soloist/supervisor.go`
- `internal/soloist/service.go`
- `internal/soloist/state.go`
- `internal/soloist/errors.go`
- `internal/soloist/*_test.go`
- `internal/playback/soloist_provider.go`
- `internal/playback/soloist_provider_test.go`
- `docs/adr/0004-dual-audio-backend.md`

Modified:

- `internal/settings/settings.go` and tests
- `internal/engine/dtos.go`
- `internal/catalog/service.go` and tests
- `internal/wailsapi/audio/service.go`, `types.go` and tests
- `main.go`
- `frontend/src/shared/api/audio.ts`, `types.ts`
- `frontend/src/lib/music.svelte.ts`
- `frontend/src/app/browse/settings.svelte.ts`, `SettingsView.svelte`
- `frontend/src/app/browse/EntityView.svelte`, `SearchView.svelte`
- `frontend/src/shared/i18n/locales/en.ts`, `es.ts`
- `docs/spotify-setup.md`, `docs/installation.md`, `docs/architecture.md`,
  `docs/development.md`, `docs/i18n.md`, `README.md`, `docs/README.md`

Not changed:

- `internal/engine/rest.go` (no new engine routes)
- `internal/engine/supervisor.go` (main adjusts options; no fork change)
- `build/patches/**`, `scripts/build-go-librespot.sh`
- packaging tasks: Soloist is never bundled
- `playback.Provider` contract

## Authorized scope

User explicitly authorized implementation on 2026-11-06:

> Vamos a implementar Soloist, hace un plan para implementarlo de forma optima,
> con buenas practicas, y solo donde sea necesario.

Authorization covers the hybrid backend described here. It does not cover
redistributing Soloist, modifying the go-librespot fork, or removing the
existing librespot default.

## Acceptance criteria

- Default install behavior is unchanged: librespot backend, no Soloist process,
  no download, no new settings file required.
- Selecting Soloist persists; after a Cadence restart, Soloist starts, the
  player controls Soloist, and go-librespot keeps library/search/context/queue
  working as `Cadence Library`.
- Missing binary, missing key, rejected key, expired build and start failure
  each produce a distinct coded, localized, actionable error.
- Installing/updating from Settings downloads only from Spotify, verifies the
  build with `--version`, and never leaves a partial binary.
- Track, album, playlist, artist and Liked Songs playback work when P0.5
  confirms the URI; otherwise the UI gates the unsupported context honestly.
- Radio is hidden in Soloist mode; `add_to_queue` accepts tracks; queue is
  read-only as today.
- Visualizer keeps working through system capture.
- `task check`, `task test` and `task verify` pass.
- No Soloist archive, binary or API key enters git or release artifacts.
- Docs and i18n are complete.

## Checks

- Go: `go test ./internal/soloist ./internal/playback ./internal/catalog
  ./internal/settings ./internal/wailsapi/audio`
- Full Go: `task test` (builds the frontend first for the embed).
- Vet/check: `task check`.
- Engine/tests: `task verify`.
- Frontend: `npm run check`, `npm run test`, `npm run lint`,
  `npm run i18n:check` in `frontend/`.
- Bindings: `task common:generate:bindings`.
- Manual: Phase 0 spike and Phase 7 E2E on the failing PC.

## Route

- P0: manual spike by the user; parent records evidence.
- P1: delegated writer (new package, multiple non-trivial files).
- P2-P4: delegated writers per phase; parent gatekeeper validates artifacts,
  paths and drift before launching the next phase.
- P5: delegated writer (frontend + bindings).
- P6: delegated writer (docs).
- P7: parent runs checks, spot-checks one command, and records evidence.
- No SDD artifacts; this is an ODD feature document.

## Delivery

- Strategy: `ask-on-risk` default. No pull request requested; deliver as
  work-unit commits on `feat/cadence-v2`.
- Forecast: about 2,200 authored lines (additions plus deletions), above the
  400-line heuristic. If a PR is requested later, split it into chained PRs and
  choose `stacked-to-main` or `feature-branch-chain` then.
- Every task closes with a Conventional Commit that keeps tests and docs with
  the behavior.

## Risks and open questions

- **P0.5 URI support.** Artist/collection playback is unverified. If unsupported,
  gate those contexts in the UI and document the gap.
- **P0.6 refusal wording.** The start-failure mapper matches the documented
  data-directory phrase plus the reference implementation's key-rejection
  phrases. The exact lines must be confirmed against a real build in Phase 0;
  if they differ, both paths degrade to `soloist_start_failed` with the raw log
  line instead of a distinct coded error.
- **Linux-only supervision.** `Setpgid`, `Pdeathsig` and `syscall.Kill` mean the
  package does not compile for Windows or macOS, matching the rest of Cadence.
  The installer still maps their archives; do not advertise those platforms
  until the process layer is build-tagged.
- **Installer ETag HEAD.** A reusable installed build is short-circuited with a
  HEAD request and the stored ETag. A missing ETag costs one archive GET; a
  Spotify-side HEAD behavior change must degrade to a normal download, not a
  failed install.
- **API key in argv.** Local `ps` exposure is unavoidable with Soloist's CLI.
  Never log argv; document the local-only threat model.
- **Seek while loading.** Soloist may drop a seek sent before the track is
  ready. Retry with a bounded loop (reference uses 500 ms for up to 15 s).
- **Two Connect devices.** The user must pick `Cadence`, not `Cadence Library`.
  Add a small guard that pauses the library device if it starts playing in
  Soloist mode; drop the guard if it proves unstable.
- **Build expiry.** Updates are manual. Warn at 14 days, block at expiry, and
  show the official download page.
- **No download checksums.** Spotify publishes no checksum. Rely on HTTPS, host
  allowlisting and `--version` validation.
- **No request IDs in the WS protocol.** Queries resolve on the next matching
  event; a spontaneous event of the same type is still a valid current snapshot.
- **Event races.** Wait for `command_result` and the matching state event with a
  short timeout; an accepted command without an event must not fail the action.
- **Windows/macOS supervision.** Stale pid cleanup is Unix-only; Windows relies
  on clean shutdown and surfaces a data-dir-in-use error. Cadence is Linux-first.
- **Bindings drift.** Regenerate bindings after every Wails signature change.
- **Unofficial protocol.** Pin behavior to the documented v1 API and keep
  protocol types isolated in `internal/soloist`.

## Progress

- [x] Problem diagnosed and workarounds ruled out.
- [x] Soloist official docs, CLI, WS API, downloads and expiry confirmed.
- [x] Reference hybrid implementation reviewed for version parsing, start
      arguments, command/event semantics, retry behavior and installer rules.
- [x] Cadence seams mapped (provider, engine, catalog, settings, Wails,
      frontend, packaging, tests).
- [x] Plan written and mirrored to Engram.
- [x] Phase 1 implemented: `internal/soloist` (4,858 lines including tests),
      four work-unit commits `b5dce70`, `151a724`, `e748bfc`, `96bbb4b`.
- [x] Phase 3 implemented: `playback.SoloistProvider` and its tests (`5e0b10f`).
- [x] Phase 4 implemented: `catalog.Playback` seam and Soloist catalog methods
      (`587b40d`).
- [x] Phase 2 implemented: settings, additive `AudioState`, Wails adapter,
      `main.go` wiring and ADR 0004 (`26571eb`).
- [x] Phase 5 implemented: bindings, frontend store, Settings cards, radio
      gating, i18n and tests (`689a864`).
- [x] Install-before-switch fix: the Soloist service is constructed in both
      modes and its state is returned to Settings; only the Soloist backend
      starts the process (`5e4947e`).
- [x] Phase 6 implemented: docs, README and ADR (`0141302`).
- [x] 14-day expiry warning in Settings (`5670f2b`).
- [ ] Phase 0 spike on the failing PC.
- [ ] Phase 7 manual E2E on the failing PC.

## Verification evidence

Phase 1, after the independent verification correction:

- `gofmt -l internal/soloist`: no output.
- `go vet ./internal/soloist`: no findings.
- `go test ./internal/soloist -count=1`: `ok cadence/internal/soloist`.
- `go test ./internal/soloist -count=10`: `ok 9.660s`.
- `go test -race ./internal/soloist -count=3`: `ok 4.290s`.
- `go build ./...`: ok.
- `git status --short`: only the pre-existing untracked `.atl/` and
  `plan/cadence-v3-roadmap.md`.

Independent verification found one critical concurrency defect: concurrent
`Supervisor.Start` calls could spawn two Soloist processes and orphan one.
The correction added a lifecycle mutex spanning start/stop/restart, reused the
service supervisor for concurrent starts, and added regression tests for
single-flight start, process reaping, stale-pid safety, tail isolation, key
trimming, host/HTTPS download validation, strict `command_result` matching and
non-blocking binary inspection. The corrected suite passes `-count=10` and
`-race -count=3`.

Phases 2–6 and the expiry warning:

- `task verify`: passed. Go vet, Svelte type checks, Prettier, all Go tests,
  44 frontend test files / 259 tests, and `go-librespot` patches apply cleanly.
- Focused suites: `go test ./internal/settings ./internal/engine
  ./internal/wailsapi/audio ./internal/catalog ./internal/playback -count=1`
  and `-race` on settings/audio.
- `gofmt -l` clean on every touched Go package; `npm run check`,
  `npm run lint`, `npm run i18n:check` clean.
- Independent Phase 5 review surfaced the install-before-switch gap: the
  Soloist service is now constructed in both modes and its state is available
  to Settings, while only the Soloist backend starts the process. The UI hides
  Start/Stop/Restart in librespot mode and keeps install/key available.
- The 14-day expiry warning is implemented and tested.
- Still unverified: real Soloist playback, URI support, exact refusal log
  strings, and the full E2E checklist. These require Phase 0 on the failing PC.

## Next step

Run Phase 0 on the failing PC: confirm `P0.2` and `P0.7` (the account plays
through Soloist), capture the URI matrix and WS payloads, and confirm the exact
start-failure strings. Then run the Phase 7 E2E from Settings: install, save
key, switch backend, restart, pair `Cadence`, play track/album/playlist/artist/
Liked Songs, seek, volume, shuffle/repeat, queue, visualizer and every error
state. Record the results here.

## Engram mirror

- Topic: `odd/soloist-audio-backend/tasks`
- Project: `spotify_winamp`
