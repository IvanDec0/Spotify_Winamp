# Cadence v2 — Execution Tracker

> **Design source:** [`../plan/cadence-v2.md`](../plan/cadence-v2.md)
> **Branch:** `feat/cadence-v2`
> **Authorized scope:** Phase 7 (user approved on 2026-11-05) after Phases 0-6.
> Scope: remove dead compact code, complete the frontend i18n sweep (compact,
> skins, Browse), add a language selector with cross-window sync, map the known
> engine/player error states to typed catalog keys, and finish the docs/tests.
> **TDD:** off — no project or session TDD configuration exists. Ordinary functional
> checks apply (`go test`, `go vet`, `gofmt`, frontend checks when touched).
> **Delivery:** one work-unit commit per task on the feature branch. Push, PRs and
> merges remain the user's decision; chain strategy D2 is still open.

## Objective

Build the observability and safety net that makes the rest of the v2 work
measurable: local diagnostics counters, a profiling endpoint, an engine patch
check in CI, a reproducible performance baseline, and the architecture decision
records that the module split will follow.

## Constraints

- No behavior change when diagnostics are disabled.
- No Wails imports in `internal/diagnostics`.
- No new global state; the recorder is wired in `main.go`.
- Do not change the go-librespot pin, patches, or product features outside the
  authorized phase. Phase 1 changes the pin and engine config as planned; later
  phases must not change them again without a new decision.
- Do not touch the frontend in Phase 0.
- Every task keeps its tests and docs in the same commit.

## Acceptance criteria

- `cadence --diagnostics` logs a loopback URL and serves `GET /debug/stats` plus
  `net/http/pprof` handlers on an ephemeral `127.0.0.1` port.
- Counters cover engine requests (by route, bytes, latency), artwork cache
  hits/misses/bytes, and player event payloads.
- With the flag off, no listener starts and the recorder is nil.
- `task audio:check` validates both patches against the pinned engine source
  without building the binary.
- CI runs the patch check.
- `docs/performance-baseline.md` defines the measurement procedure, static
  baseline, and Phase 4 target gates.
- Three ADRs record module boundaries, the event model, and error-code ownership.
- `go test`, `go vet` and `gofmt` are clean.

## Phase 0 tasks

### T0.1 — Diagnostics recorder and engine transport

- [x] `internal/diagnostics` package: nil-safe `Recorder`, atomic/mutex counters,
      stable `Snapshot` JSON, `WrapTransport` round tripper for engine requests.
- [x] Unit tests: concurrent increments, snapshot shape, transport accounting,
      error counting, no-op with a nil recorder.
- Commit: `feat(diagnostics): add engine and artwork metrics recorder`

### T0.2 — Instrumentation and local diagnostics endpoint

- [x] `artcache` and `player.Service` accept an optional recorder (setter, nil-safe).
- [x] `main.go`: `--diagnostics` flag, loopback listener on `127.0.0.1:0`,
      `/debug/stats` and `/debug/pprof/*`, logged URL, clean shutdown.
- [x] Preserve the 12 s engine HTTP timeout when wrapping the transport.
- Commit: `feat(diagnostics): expose loopback stats and pprof behind a flag`

### T0.3 — Engine patch verification in CI

- [x] `scripts/build-go-librespot.sh --check` applies both patches and exits
      without building.
- [x] `Taskfile.yml`: `audio:check` and `verify` tasks.
- [x] CI runs `task audio:check`.
- Commit: `build: verify go-librespot patches without building`

### T0.4 — Baseline and decision records

- [x] `docs/performance-baseline.md`: procedure, static baseline, measurement
      table to fill on the reference machine, Phase 4 gates.
- [x] `docs/adr/0001-module-boundaries.md`
- [x] `docs/adr/0002-event-driven-engine-state.md`
- [x] `docs/adr/0003-error-codes-and-i18n.md`
- Commit: `docs: add performance baseline and v2 ADRs`

### T0.5 — Phase 0 verification

- [x] `go test $(go list ./... | grep -v '/node_modules/')`
- [x] `go vet $(go list ./... | grep -v '/node_modules/')`
- [x] `gofmt -l` over non-vendored Go directories
- [x] `./scripts/build-go-librespot.sh --check`
- [x] Record exact results in this file.
- Commit: `chore: record Phase 0 verification`

## Phase 0 verification — 2026-11-04

Environment: Go `go1.27.3-X:nodwarf5 linux/amd64`, Task `3.54.0`, branch
`feat/cadence-v2`.

| Command | Result |
|---|---|
| `go test $(go list ./... \| grep -v '/node_modules/')` | exit 0 |
| `go vet $(go list ./... \| grep -v '/node_modules/')` | exit 0, no findings |
| `gofmt -l $(go list -f '{{.Dir}}' ./... \| grep -v '/node_modules/')` | exit 0, no files listed |
| `./scripts/build-go-librespot.sh --check` | exit 0, both patches applied |
| `git diff --check` | exit 0, no whitespace errors |

`go test` output:

```text
ok  	cadence	0.046s
ok  	cadence/internal/artcache	0.007s
ok  	cadence/internal/diagnostics	0.007s
ok  	cadence/internal/golibrespot	0.164s
?   	cadence/internal/platform	[no test files]
ok  	cadence/internal/player	0.067s
ok  	cadence/internal/settings	(cached)
ok  	cadence/internal/skins	(cached)
```

Patch check output:

```text
patching file daemon/api_server.go
patching file daemon/player.go
patching file daemon/search.go
patching file daemon/track_meta_cache.go
patching file spclient/context_resolver.go
patching file spclient/searchview.go
patching file spclient/spclient.go
patching file ap/ap.go
patching file ap/proxy.go
go-librespot patches apply cleanly to v0.10.6-0.20261102145552-522adcae8d65; skipping build (--check)
```

Additional checks run during the phase: `go test -race` for
`internal/diagnostics`, `internal/artcache`, `internal/player` and the root
package; all passed. `task --list-all` shows `audio:check` and `verify`.

### Commit map

| Task | Commit |
|---|---|
| T0.1 | `5e7449f` |
| T0.2 | `db2f968` |
| T0.3 | `67143fe` |
| T0.4 | `939e949` |
| T0.5 | `14644ca` |

## Phase 0 status

- **Code and docs:** complete at `1e02e7f`.
- **Verification:** all commands green (see the progress log and
  `docs/performance-baseline.md`).
- **Pending:** runtime baseline capture on the reference machine
  (`./bin/cadence --diagnostics`, 60 s idle, fill the measurement table in
  `docs/performance-baseline.md`). Deferred by the user on 2026-11-04; capture it
  before the Phase 4 performance work so the before/after comparison stays honest.
- **Next:** Phase 2 (frontend foundation) or Phase 3 (Go decomposition) after
  Phase 1 closes.

## Phase 1 tasks

### T1.1 — Pin go-librespot `master@16b65bb4`

- [x] Set `VERSION="v0.10.6-0.20261104085532-16b65bb4cd2b"` in
      `scripts/build-go-librespot.sh`.
- [x] Verify both patches still apply with `--fuzz=0`; rebase them if the
      station-page fix conflicts. No rebase was needed.
- [x] Build the engine with `task audio:build` and confirm the binary runs.
- Commit: `build: pin go-librespot master for device_is_group support`

### T1.2 — Engine configuration

- [x] `RuntimeOptions`: add `DeviceIsGroup bool` and `ImageSize string`
      (validated against `default`/`small`/`large`/`xlarge`).
- [x] `writeConfig`: emit `device_is_group`, `server.image_size`, and a comment
      documenting `accesspoint_ports` as the override for networks that intercept
      443.
- [x] `main.go`: keep `DeviceIsGroup` off by default (group icon only; Jam uses
      the engine's Connect mode independently).
- [x] Tests for the generated config and the image-size validation.
- Commit: `feat(engine): advertise as a group and expose image size config`

### T1.3 — Surface the running engine version

- [x] Parse the engine log line `running go-librespot <version>` after startup and
      expose it through `Runtime.Version()`.
- [x] Populate `AudioState.EngineVersion` (the field and frontend model already
      exist but were always empty).
- [x] Add `engine_version` to `GET /debug/stats`.
- [x] Tests with a temporary log file; no version parse when the log is missing.
- Commit: `feat(engine): surface the running go-librespot version`

### T1.4 — Documentation and verification

- [x] Update the pin in `README.md`, `plan/nombre_plan.md` and
      `docs/performance-baseline.md`.
- [x] Document `device_is_group`, `accesspoint_ports` and `server.image_size` in
      `docs/spotify-setup.md`; mention both patches in `docs/development.md`.
- [x] Run and record: `go test`, `go vet`, `gofmt`, `task audio:check`,
      `task audio:build`.
- Commit: `docs: document the engine upgrade and options`

## Phase 1 verification — 2026-11-04

Environment: Go `go1.27.3-X:nodwarf5 linux/amd64`, Task `3.54.0`, branch
`feat/cadence-v2`. The pin upgrade needed **no patch rebase**: both patches
applied unchanged with `--fuzz=0`.

| Command | Result |
|---|---|
| `go test $(go list ./... \| grep -v '/node_modules/')` | exit 0 |
| `go vet $(go list ./... \| grep -v '/node_modules/')` | exit 0, no findings |
| `gofmt -l $(go list -f '{{.Dir}}' ./... \| grep -v '/node_modules/')` | exit 0, no files listed |
| `./scripts/build-go-librespot.sh --check` | exit 0, both patches applied |
| `task audio:build` | exit 0, engine binary built |
| `git diff --check` | exit 0, no whitespace errors |

`go test` output:

```text
ok  	cadence	(cached)
ok  	cadence/internal/artcache	(cached)
ok  	cadence/internal/diagnostics	(cached)
ok  	cadence/internal/golibrespot	(cached)
?   	cadence/internal/platform	[no test files]
ok  	cadence/internal/player	(cached)
ok  	cadence/internal/settings	(cached)
ok  	cadence/internal/skins	(cached)
```

Patch check output:

```text
patching file daemon/api_server.go
patching file daemon/player.go
patching file daemon/search.go
patching file daemon/track_meta_cache.go
patching file spclient/context_resolver.go
patching file spclient/searchview.go
patching file spclient/spclient.go
patching file ap/ap.go
patching file ap/proxy.go
go-librespot patches apply cleanly to v0.10.6-0.20261104085532-16b65bb4cd2b; skipping build (--check)
```

`task audio:build` produced `bin/go-librespot` and exited 0. Before the version
fix, a smoke start logged `running go-librespot fb41f3cf` — Cadence's VCS
revision, because the copied engine source lives inside this repository, so Go
stamped it with Cadence's `vcs.revision` instead of the go-librespot pin. The
build now injects the real pin with
`-ldflags "-X github.com/devgianlu/go-librespot.version=${VERSION#v}"` and
`-buildvcs=false`. Verified first line on the rebuilt binary:

```text
time="2026-11-04T11:10:20-03:00" level=info msg="running go-librespot 0.10.6-0.20261104085532-16b65bb4cd2b"
```

**Pending user verification:** live pairing, playback, library browsing and
Spotify Jam participation require the user's Spotify account and audio device.
They are not claimed as verified here.

### Phase 1 review corrections — 2026-11-04

- `device_is_group` is **off by default** and only advertises a speaker-group icon;
  it does not enable Jam. Jam participation uses the engine's Connect/Jam mode
  independently, and live behavior remains unverified.
- `config.yml` is generated on every start; the generated-file comment says so and
  no longer invites `accesspoint_ports` edits. `prefer_firewall_friendly_ports: true`
  stays active; `accesspoint_ports` is not exposed by Cadence yet.
- Engine version injection strips the leading `v` (`${VERSION#v}`) and
  `-buildvcs=false` prevents Cadence's revision from leaking into
  `SpotifyLikeClientVersion()`. The first log line is the go-librespot pin.
- `Version()` is empty while stopped, cached after parsing, and `State` retries a
  failed parse at most every 2 s. Start/Stop clear the cached version.

Verification for this round: `go test`, `go test -race ./internal/golibrespot/...`,
`go vet`, `gofmt -l`, `./scripts/build-go-librespot.sh --check`, `task audio:build`
and `git diff --check` all exited 0. Smoke first line:
`msg="running go-librespot 0.10.6-20261104085532-16b65bb4cd2b"`.

### Commit map — Phase 1

| Task | Commit |
|---|---|
| T1.1 | `2b10f55` |
| T1.2 | `05adf37` |
| T1.3 | `fb41f3c` |
| T1.4 | `2b7a982` |
| Engine version fix | `f2efa8e` |
| Review corrections 2 | `59dc4d4` |

## Phase 1 status

- **Code and docs:** complete at `59dc4d4`.
- **Verification:** `go test`, `go test -race ./internal/golibrespot/...`, `go vet`,
  `gofmt`, `task audio:check`, `task audio:build` and `git diff --check` are green.
  The rebuilt binary logs the real go-librespot pin.
- **Deviations:** `server.image_size` is written as `default`; the source variant
  is selected in Phase 4 after local resizing exists. `device_is_group` is off by
  default because it only changes the Spotify group icon, not Jam participation.
- **Pending user verification:** live pairing, playback, library browsing and Jam
  behavior need the user's Spotify account and audio device.
- **Next:** Phase 2 (frontend foundation) is authorized and in progress.

## Phase 2 tasks

### T2.1 — Typed API boundary

- [x] `src/shared/api`: `client.ts` (`ApiError`, `toApiError`, `call`),
      `player.ts`, `audio.ts`, `skins.ts`, `window.ts`, `events.ts`, `types.ts`,
      `index.ts`. Only this folder may import `bindings/**` and
      `@wailsio/runtime`.
- [x] Migrate the player, music and skins stores, App.svelte, the skin runtime
      components and component type imports to the wrapper.
- [x] Keep the generated Wails bindings untouched and the existing test mocks
      working. Add tests for error normalization, event subscription and the
      window wrapper.
- Commit: `refactor(frontend): route bindings through a typed api boundary`

### T2.2 — i18n foundation

- [x] Add `intl-messageformat` at an exact version (12.1.4) and update the
      lockfile.
- [x] `src/shared/i18n`: typed `en`/`es` catalogs as TS modules (compile-time key
      parity), `I18n` runtime with `IntlMessageFormat`, cached formatters, locale
      detection, `localStorage` persistence and `document.documentElement.lang`.
- [x] Call `initI18n()` before mounting in `main.ts`. Add `ui.*` keys used by the
      shared primitives; do not sweep existing screens yet (Phase 7).
- [x] Update ADR-0003 and plan §7 to record TS catalogs instead of JSON plus a
      generator.
- [x] Tests: interpolation, plural, locale switch, persistence, fallback.
- Commit: `feat(i18n): add typed catalogs and a locale runtime`

### T2.3 — Shared UI primitives and navigation state

- [x] `src/shared/ui`: `Button.svelte`, `Dialog.svelte` (focus trap, Escape,
      backdrop, `aria-modal`), `VirtualList.svelte` + pure `visibleRange` helper,
      `Skeleton.svelte`, `Toast.svelte` + `toasts.svelte.ts` store,
      `ToastHost.svelte`.
- [x] `src/app/navigation.svelte.ts`: typed panels and route stack, with tests.
      It is not wired into App.svelte in this phase; Phase 6 does that.
- [x] Component tests for Dialog, VirtualList, Toast and the navigation store.
- Commit: `feat(ui): add shared primitives and navigation state`

### T2.4 — Verification

- [x] `npm run check`, `npm run lint`, `npm run format`, `npm run test`,
      `npm run build`.
- [x] Confirm `rg 'bindings/|@wailsio/runtime' frontend/src` only matches
      `src/shared/api` and test mock paths.
- [x] Record exact results in this tracker.
- Commit: `chore: record Phase 2 verification`

### Phase 2 deviations

- App.svelte keeps the existing panel booleans; the new navigation store is
  introduced and tested but wired in Phase 6 to keep Phase 2 behavior-preserving.
- i18n catalogs are TS modules with literal key types instead of JSON plus a
  generated `.d.ts`; this removes a build step and keeps key parity compile-time.
- Existing screens are not translated yet; Phase 7 performs the sweep.

## Phase 2 verification — 2026-11-04

Environment: Node `v22.23.4`, npm `12.2.1`, Vite `8.3.5`, branch
`feat/cadence-v2`, commands run from `frontend/`.

| Command | Result |
|---|---|
| `npm run check` | exit 0, `svelte-check found 0 errors and 0 warnings` |
| `npm run lint` | exit 0, no findings |
| `npm run format` | exit 0, `All matched files use Prettier code style!` |
| `npm run test` | exit 0, 24 files, 124 tests passed |
| `npm run build` | exit 0, 226 modules transformed, built in 1.10 s |
| `rg -n 'bindings/\|@wailsio/runtime' frontend/src` | only `src/shared/api/**` imports and `vi.mock` paths |
| `git diff --check` | exit 0, no whitespace errors |

`npm run build` output:

```text
dist/index.html                   0.43 kB │ gzip:  0.27 kB
dist/assets/index-CcI2SMLs.css   63.62 kB │ gzip: 10.71 kB
dist/assets/es-wzTepdfd.js        0.03 kB │ gzip:  0.05 kB
dist/assets/es-BtaKyWmH.js        0.30 kB │ gzip:  0.21 kB
dist/assets/index-D0Zql8id.js   235.40 kB │ gzip: 72.37 kB
✓ built in 1.10s
```

The separate `es-*` chunks prove the Spanish catalog is loaded lazily.

### Commit map — Phase 2

| Task | Commit |
|---|---|
| T2.1 | `e4c9cb7` |
| T2.2 | `c0b8c20` |
| T2.3 | `7adecc5` |
| T2.4 | `9210648` |
| Review corrections | `4a28390` |
| Final review fixes | `fix(frontend): skip non-tabbable dialog elements and stale locale errors` |

### Phase 2 review corrections — 2026-11-04

- `player.seek` and `player.setVolume` round to integers before calling the
  binding; Wails/Go reject fractional ints. Added `shared/api/player.test.ts`.
- `initI18n(instance = i18n)` falls back to English when the selected catalog
  loader rejects, so startup never blocks mount. Locale switches are
  latest-wins via a request token.
- Dialog focus filtering skips `type="hidden"`, `[hidden]`,
  `[aria-hidden="true"]`, `[inert]` and `tabindex < 0`; an empty dialog
  focuses its panel, Escape stops propagation, and App ignores already
  prevented key events.
- `visibleRange` includes a fractional trailing row; VirtualList documents the
  `key` prop for reordering. Added fractional, overscan-0 and measured-height
  tests.
- Button forwards native button attributes through a typed rest. ToastHost no
  longer adds a second live region; each toast announces itself.
- The i18n barrel no longer statically exports `es`; the lazy chunk remains.
  Added same-locale cache invalidation, Svelte template reactivity and ICU
  syntax coverage.
- `system.invoke` is typed `string` and documented as fire-and-forget.
  ADR-0003 now marks the ESLint literal guard as Phase 7 work.
- Final pass: Dialog excludes `tabindex="-1"` from the sequential trap, and a
  stale locale loader rejection no longer surfaces after a newer switch wins.

Correction verification, run from `frontend/`:

| Command | Result |
|---|---|
| `npm run check` | exit 0, 0 errors and 0 warnings |
| `npm run lint` | exit 0, no findings |
| `npm run format` | exit 0, all files match Prettier |
| `npm run test` | exit 0, 24 files, 124 tests passed |
| `npm run build` | exit 0, 226 modules; one lazy `es-*` chunk |
| `rg -n 'bindings/\|@wailsio/runtime' src` | only `src/shared/api/**` imports and `vi.mock` paths |
| `git diff --check` | exit 0, no whitespace errors |

## Phase 2 status

- **Code and docs:** complete on `feat/cadence-v2`, including the review
  correction round.
- **Verification:** check, lint, format, test, build, binding-import grep and
  `git diff --check` are green (106 tests before corrections, 124 after the final fixes).
- **Behavior:** unchanged; the navigation store and toast host are not wired into
  `App.svelte` until Phase 6, and existing screens remain English until Phase 7.
- **Next:** Phase 3 (Go decomposition) is authorized and in progress.

## Phase 3 tasks

Goal: behavior-preserving split. No JSON field, event name, or product behavior
may change. Each commit must leave the tree compiling and the Go tests green.

### T3.1 — Engine and artwork extraction

- [x] `internal/engine`: supervisor (process/config/version), typed REST client,
      raw engine DTOs (`Root`, `Track`, `Status`, `Pairing`, `AudioState`,
      `ContextTrack(s)`).
- [x] `internal/artwork`: rename `internal/artcache` and keep the cache behavior
      and diagnostics counters identical.
- [x] Update the engine-backed provider, the audio service and `main.go` to use
      the new packages. Keep temporary Wails adapters in the old packages so the
      application still compiles.
- [x] Move the engine tests with the code.
- Commit: `refactor(engine): extract the supervisor, transport and artwork cache`
- Note: `engine.Options.Client` is an explicit test seam for providers that do
  not start the sidecar; production leaves it nil, so `Start` still creates the
  client.

### T3.2 — Catalog and playback split

- [x] `internal/catalog`: library, search, context and search models plus the
      audio service logic currently in `golibrespot.Service` (no Wails imports).
      `OpenPairing` browser launching stays out of the domain.
- [x] `internal/playback`: provider interface, player state, demo provider and
      the engine-backed provider currently in `golibrespot/provider.go`.
- [x] Update `main.go` and the temporary adapters. Move the tests with the code.
- Commit: `refactor(core): split catalog and playback from the Wails adapters`

### T3.3 — Wails adapters, bindings and frontend

- [x] `internal/wailsapi/{audio,player,skins}`: the three Wails service structs,
      each in its own package so the generator stays predictable. Only these
      adapters (plus `internal/platform`) import Wails.
- [x] Delete `internal/golibrespot`, the old `internal/player` adapter and
      `internal/artcache`.
- [x] Regenerate bindings with
      `wails3 generate bindings -clean=true -ts -i .`.
- [x] Update `frontend/src/shared/api` imports, `frontend/bindings` consumers and
      the `vi.mock` paths. Keep the facade API stable for the app.
- Commit: `refactor(api): move Wails services to wailsapi and regenerate bindings`

### T3.4 — Architecture docs and verification

- [x] Update ADR-0001 with the final layout and the `internal/platform`
      exception for native window/GTK code. Add `docs/architecture.md`.
- [x] Run and record: `go test`, `go vet`, `gofmt`, `task audio:check`,
      `wails3 generate bindings` (no diff), `npm run check/lint/format/test/build`.
- Commit: `docs: record the v2 package layout and Phase 3 verification`

### Phase 3 acceptance

- [x] Domain packages (`engine`, `catalog`, `playback`, `artwork`, `skins`) do not
      import Wails.
- [x] No JSON/event/API behavior changes; existing tests keep passing.
- [x] Bindings regenerate cleanly and the frontend builds with the facade intact.
- [x] `internal/golibrespot`, the old `internal/player` adapter and
      `internal/artcache` are gone.

## Phase 3 verification — 2026-11-04

Environment: Go `go1.27.3-X:nodwarf5 linux/amd64`, Wails `v3.0.0-beta.34`,
Node `v22.23.4`, branch `feat/cadence-v2`.

| Command | Result |
|---|---|
| `go test $(go list ./... \| grep -v '/node_modules/')` | exit 0; all packages `ok`, three adapter packages have no test files |
| `go test -race ./internal/engine/... ./internal/catalog/... ./internal/playback/... ./internal/artwork/...` | exit 0; engine 1.248s, catalog 1.033s, playback 1.029s, artwork 1.025s |
| `go test -cover ./internal/catalog/` | exit 0, `coverage: 80.4% of statements` |
| `go vet $(go list ./... \| grep -v '/node_modules/')` | exit 0, no findings |
| `gofmt -l $(go list -f '{{.Dir}}' ./... \| grep -v '/node_modules/')` | exit 0, no files listed |
| `task audio:check` | exit 0; all nine patches apply cleanly to `v0.10.6-0.20261104085532-16b65bb4cd2b` |
| `wails3 generate bindings -clean=true -ts -i .` | exit 0; 291 packages, 4 services, 39 methods, 2 enums, 38 models, 3 events |
| `git status --porcelain frontend/bindings` after regeneration | no output (no binding diff) |
| `npm run check` | exit 0, `svelte-check found 0 errors and 0 warnings` |
| `npm run lint` | exit 0, no findings |
| `npm run format` | exit 0, `All matched files use Prettier code style!` |
| `npm run test` | exit 0, 24 files, 124 tests passed |
| `npm run build` | exit 0, 230 modules transformed, built in 550 ms |
| Domain Wails import scan | `none` for `engine`, `catalog`, `playback`, `artwork`, `skins`; only `main`, `platform` and the three `wailsapi` packages import Wails |
| `git diff --check` | exit 0, no whitespace errors |

Generated binding paths after regeneration:

```text
frontend/bindings/cadence/internal/wailsapi/audio/{index,models,service}.ts
frontend/bindings/cadence/internal/wailsapi/player/{index,models,service}.ts
frontend/bindings/cadence/internal/wailsapi/skins/{index,models,service}.ts
frontend/bindings/cadence/internal/platform/{index,windowservice}.ts
frontend/bindings/cadence/internal/engine/{index,models}.ts
frontend/bindings/cadence/internal/catalog/{index,models}.ts
frontend/bindings/cadence/internal/playback/{index,models}.ts
frontend/bindings/cadence/internal/skins/{index,models}.ts
```

### Commit map — Phase 3

| Task | Commit |
|---|---|
| T3.1 | `61d3ba1` |
| T3.2 | `ca2dc58` |
| T3.3 | `8b8ab2e` |
| T3.4 | `d8887e4` |
| Review: catalog tests | `b16a453` |
| Review: docs alignment | `85de8a9` |

## Phase 3 status

- **Code and docs:** complete at `85de8a9`. The old `internal/golibrespot`,
  `internal/player` adapter and `internal/artcache` are gone.
- **Verification:** all Go tests and race tests green, `go vet`, `gofmt`,
  `task audio:check`, binding regeneration with no diff, 124 frontend tests,
  `npm run check/lint/format/build`, and catalog coverage at **80.4%**.
- **Behavior:** unchanged. Only `main.go`, `internal/platform` and the three
  `wailsapi` packages import Wails; domain packages are independent.
- **Pending user verification:** a live GUI/Spotify smoke test (pairing,
  playback, library, skins) after the refactor.
- **Pending before Phase 4:** capture the runtime performance baseline in
  `docs/performance-baseline.md`. The user deferred it; Phase 4 changes the
  state transport, so the before/after comparison needs this capture first.
- **Next:** Phase 4 (event-driven engine state and artwork variants).

## Phase 4 tasks

Measured baseline: idle engine traffic **2.5 req/s**, 396 B/request, 0.87 ms/request
(150 requests in 60 s). Targets: ≤0.2 req/s idle, ≤4 KB per `player:state`
change, no event while paused and unchanged, idle CPU <2%.

### T4.1 — Event-driven state transport

- [x] `internal/engine/events.go`: `/events` WebSocket client using
      `github.com/coder/websocket`, typed event envelope, bounded reads.
- [x] `internal/playback`: `Streamer` interface and `EngineProvider.Stream`:
      initial `/status` snapshot, debounced refresh on engine events, 15 s
      reconciliation, monotonic `Revision`, `CapturedAt`, and emit only when the
      state changes. No queue in the streamed state.
- [x] `Provider.Queue(ctx)`: move the queue building out of `State`; implement it
      for the engine and demo providers. `wailsapi/player.GetQueue` uses it.
- [x] `wailsapi/player`: replace the 500 ms ticker with `Stream` for providers
      that implement it (demo keeps a fallback ticker). Emit on revision change;
      keep actions and recorder behavior.
- [x] Frontend: remove the 2 s audio poll; poll only while unauthenticated (5 s).
      Add `player.queue` + `loadQueue`, use it in the main and skin Queue
      components, add a `positionMs` getter from `capturedAt` for the seek
      shortcuts, and drop stale states by revision.
- [x] Tests: engine event parsing, provider stream refresh/reconnect, queue
      separation, adapter no-duplicate emit, frontend store/queue/position.
- Commit: `feat(state): drive player state from the engine event stream`

### T4.2 — Artwork variants, single-flight and disk cache

- [x] `internal/artwork`: size variants (64/128/240/640) on
      `GET /art/{key}?size=`, single-flight for concurrent misses, bounded disk
      LRU under `$XDG_CACHE_HOME/cadence/art`, unchanged default behavior.
- [x] Frontend: request the right variant from `CoverArt`/`MediaCover` and the
      skin components, with `loading="lazy"`, `decoding="async"` and explicit
      dimensions.
- [x] Tests for variant selection, single-flight, disk eviction and fallback.
- Commit: `perf(art): add size variants, single-flight and a disk cache`

### T4.3 — Idle rendering and long lists

- [x] Gate the `Progress` rAF loop while paused/hidden; add interpolation to the
      skin `Progress`; pause visualizers when not playing or the window is hidden.
- [x] Virtualize the context list if its row height is fixed; otherwise report the
      measured row behavior and defer to Phase 6 instead of forcing it.
- [x] Frontend tests for the position getter and the idle gate helper.
      Covered by the player position test and the Progress/skin Progress frame
      and interval tests; visualizers keep the existing math tests.
- Commit: `perf(ui): stop idle animation work and virtualize long lists`

#### T4.3 virtualization decision

Deferred to Phase 6 after inspecting the layout; the 800-row scroll gate is **not
claimed as met**.

Measured list geometry: `.track-row` resolves to **34 px** (`min-height: 34px`,
`box-sizing: border-box`, 28 px cover + 4 px vertical padding), and
`.track-list` adds a **2 px** flex gap, so the row pitch is **36 px**.

Why virtualization was not forced:

- `.library-body` is the single `overflow: auto` scroll container shared by the
  search, pairing, context and tabbed library views. `VirtualList` owns its own
  `overflow-y: auto` viewport, so using it here nests scroll containers. Making
  the list fill the body requires changing `.library-body` overflow and turning
  `.context-panel` into a height-constrained flex column for the context branch.
- The context header (`.context-actions`) currently sits inside the shared
  scroll container and scrolls with the rows. Keeping it outside the virtual
  viewport would pin it — a layout/behavior change beyond this task.
- The rows are an `<ol class="track-list">` with `<li>` children, while
  `VirtualList` renders its window with `div` wrappers and absolute
  positioning. Adopting it would require changing the list markup or adding a
  wrapper-tag option to the shared primitive.
- The 2 px flex gap would need to become a row margin (or a 36 px row box) to
  match the virtual `itemHeight`.

Phase 6 (library shell/navigation) can restructure the panel and then virtualize
the context list without nested scrolling or sticky-header regressions.

### T4.4 — After-baseline and verification

- [x] Run the app with `--diagnostics` and capture the same idle sample; record
      the after numbers in `docs/performance-baseline.md`.
- [x] Run `go test`, race on the touched packages, `go vet`, `gofmt`, bindings
      regeneration with no diff, `task audio:check`, and frontend
      check/lint/format/test/build.
- Commit: `docs: record the Phase 4 after-baseline and verification`

### Phase 4 acceptance

- Idle engine request rate ≤0.2 req/s (measured).
- Streamed `player:state` ≤4 KB per change; no event while paused and unchanged.
- Queue is fetched on demand and no longer inflates every state.
- Idle UI work stops when paused or hidden.

### Commit map — Phase 4

| Task | Commit |
|---|---|
| T4.1 | `26b8c62` |
| T4.1 review corrections | `9ae6a79` |
| T4.2 | `7a2fc8d` |
| T4.3 | `a28d8ea` |


### T4.1 review corrections — 2026-11-04

- Engine loss publishes a stopped `audio_unavailable` snapshot before `Stream`
  returns, so the frontend stops interpolating the last position.
- Fetches carry a capture sequence; a slow fetch can no longer stamp a newer
  revision with an older snapshot.
- Queue panels show a loading placeholder instead of a false empty state, and
  reload only when the tracked track URI changes rather than on every applied
  state.
- The skin progress control interpolates from `player.positionMs` with a 250 ms
  tick that stops while paused, dragging or hidden.
- `player.start()` subscribes before the first fetch, so a failed initial RPC
  still receives streamed states.
- The event watcher starts before the initial status fetch; a reconcile clears
  the dirty flag so one event cannot cause two `/status` requests.
- The demo state no longer embeds the queue; `Queue()` remains the on-demand
  path. Reconnect attempts are logged with the error and backoff.

## Phase 4 verification (static) — 2026-11-04

| Command | Result |
|---|---|
| `go test $(go list ./... \| grep -v '/node_modules/')` | exit 0, all Go packages ok |
| `go test -race ./internal/engine/... ./internal/playback/... ./internal/artwork/... ./internal/wailsapi/player/...` | exit 0 |
| `go vet`, `gofmt -l` | exit 0, no findings |
| `wails3 generate bindings -clean=true -ts -i .` | exit 0; `frontend/bindings` unchanged |
| `task audio:check` | exit 0, both patches apply |
| `npm run check` | exit 0, 0 errors/warnings |
| `npm run lint`, `npm run format` | exit 0 |
| `npm run test` | exit 0, 28 files / 143 tests |
| `npm run build` | exit 0, 231 modules |
| `git diff --check` | exit 0 |

## Phase 4 status

- **Code:** complete at `a28d8ea`.
- **Idle traffic (structural):** the 500 ms ticker and the 2 s frontend poll are
  gone. Authenticated idle is only the 15 s `/status` reconciliation, about
  **0.067 req/s**. The audio poll runs only while unauthenticated (5 s).
- **Payload (structural):** streamed `PlayerState` no longer carries the queue;
  the remaining fields are roughly 0.8-1.2 KB per change. The queue is fetched
  on demand with `GetQueue`.
- **Idle work:** progress and visualizer loops stop when paused or hidden; the
  visualizer caps at 30 fps.
- **After measurement:** captured on the reference machine. Engine traffic is
  **0.067 req/s** (4 requests in 60 s, down from 2.5), `player:state` is
  **541 B/event**, and the paused/unchanged delta is **0 events**. The single
  error is a startup error and does not change.
- **Gates:** request rate and payload/no-idle-event are green. Idle CPU remains
  unproven: only a 3.2% `ps` lifetime average was captured, not the 60 s mean
  the gate asks for. RSS is a 299,060 KiB point sample; 10-minute stability was
  not captured.
- **Deferred:** context-list virtualization moves to Phase 6. The measured row
  pitch is 36 px inside the shared `.library-body` scroll container; forcing
  `VirtualList` there would nest scroll containers and change the context header
  behavior.
- **Next:** **stopped here by user instruction.** Phase 5 has not started.
  When work resumes, the open decisions are Phase 5 catalog work and D1 (Browse
  surface) for Phase 6.

## Phase 5 tasks

### Spike result — 2026-11-05

A standalone engine run with a copied config/state (credentials never printed,
temp dir removed) queried the internal searchview for `podcast`. The raw
`results` keys are:

`tracks`, `albums`, `artists`, `playlists`, `shows`, **`audioepisodes`**,
`profiles`, `genres`, `topHit`.

`shows.hits[0]` has `name`, `uri`, `image`, `showType`, `musicAndTalk`.
`audioepisodes.hits[0]` has `name`, `uri`, `image`, `explicit`, `duration` (0 in
search), `musicAndTalk`. The public `/search` patch currently decodes only the
four original groups, so the new groups are lost before the HTTP response.

### T5.1 — Search groups (shows and episodes)

- [x] Add `build/patches/go-librespot-search-groups.patch`, applied after the
      search/radio patch: decode `shows` and `audioepisodes` from the raw
      searchview, expose them as `shows` and `episodes` in `/search`, and add
      `showType`, `musicAndTalk` and `explicit` to search hits.
- [x] Add the patch to `scripts/build-go-librespot.sh`, the Taskfile
      `audio:ensure` status list and the engine docs/notices.
- [x] Extend `internal/engine` search DTOs and the catalog test with show and
      episode hits, including artwork rewriting.
- [x] Add `shows`/`episodes` to the frontend `SearchResultsView` mapping; the
      tabs land in Phase 6.
- [x] Live-verify the patched `/search` returns non-empty `shows` and `episodes`
      (parent-run with copied credentials; no secrets in output). Verified:
      `q=podcast&limit=5` returned shows=5, episodes=5, tracks=5, playlists=5;
      show hits expose `showType`/`musicAndTalk`, episode hits `explicit`.
- Commit: `feat(search): expose podcast shows and episodes`

### T5.2 — Library collection writes and podcasts

- [x] Add `build/patches/go-librespot-library-collection.patch`: generic
      collection write/contains routes for the `collection` (liked tracks and
      saved albums) and `artist` (followed artists) sets, plus a `show` set if
      the engine accepts it. Keep the existing liked/playlist routes.
- [x] Add engine REST methods and `catalog`/`library` use cases: like/unlike a
      track, save/remove an album, follow/unfollow an artist, save/remove a
      show, check saved state.
- [x] Add `YourEpisodes()` to the catalog: build the user-scoped collection URI
      from `/status` and return the context listing.
- [x] Add playlist add/remove use cases on top of the existing routes.
- [x] Expose the write methods through `wailsapi/audio`, regenerate bindings,
      and add fake-engine tests. Defer the like/save buttons to Phase 6.
- Commit: `feat(library): add collection writes and podcast helpers`

### T5.3 — Engine settings surface

- [x] Extend `engine.Options`/`writeConfig` with bitrate, crossfade,
      normalization (disabled/album gain/pregain), disk audio cache
      (enabled/dir/size), FLAC, MPRIS and device-group/image-size.
- [x] Persist engine settings in Cadence's settings store and expose
      `GetEngineSettings`/`SetEngineSettings` through a Wails adapter; apply on
      an explicit engine restart, never silently mid-playback.
- [x] Tests for config generation, defaults and persistence. Defer the settings
      UI to Phase 6/7; the bindings and typed facade are ready.
- Commit: `feat(settings): expose engine playback and cache options`

### T5.4 — Verification

- [x] `go test`, race on the touched packages, `go vet`, `gofmt`,
      `task audio:check`, `task audio:build`, bindings regeneration with no
      diff, frontend check/lint/format/test/build.
- [x] Live read-only search verification (`/search` 5 shows + 5 episodes) and
      collection `contains` for album/artist/show. Actual saves/follows still
      need the user's explicit account check.
- Commit: `docs: record Phase 5 verification`

### Phase 5 acceptance

- `/search` returns `shows`/`episodes` with artwork keys and the original four
  groups unchanged.
- Library writes use the generic collection route and degrade with a clear error
  when the engine lacks it.
- Engine settings persist and apply on restart; no silent restart.
- No frontend regression; UI for the new surfaces remains Phase 6.

### Commit map — Phase 5

| Task | Commit |
|---|---|
| T5.1 | `ed74d9d` |
| T5.2 | `8d069d3` |
| T5.2 correction | `b01bfcb` |
| T5.3 | `1d845ee` |

## Phase 5 verification — 2026-11-05

| Command | Result |
|---|---|
| `go test $(go list ./... \| grep -v '/node_modules/')` | exit 0, all packages ok |
| `go test -race ./internal/engine/... ./internal/catalog/... ./internal/settings/... ./internal/wailsapi/audio/...` | exit 0 |
| `go vet`, `gofmt -l` | exit 0, no findings |
| `./scripts/build-go-librespot.sh --check` | exit 0; all four patches apply with `--fuzz=0` |
| `task audio:build` | exit 0; engine rebuilt |
| `wails3 generate bindings -clean=true -ts -i .` | exit 0; 4 services, 52 methods, 40 models; no diff after commit |
| `npm run check` | exit 0, 0 errors/warnings |
| `npm run lint`, `npm run format` | exit 0 |
| `npm run test` | exit 0, 28 files / 143 tests |
| `npm run build` | exit 0, 232 modules |
| `git diff --check` | exit 0 |

Live read-only checks (copied config/state, temp dir removed, no secrets
printed):

- `/search?q=podcast&limit=5` returned shows=5, episodes=5, tracks=5,
  playlists=5; show hits carry `showType`/`musicAndTalk`, episode hits
  `explicit`.
- `/library/collection/contains` returned 200 for `collection` + album,
  `artist` + followed artist, and `collection` + show.
- Saved shows use the `show` set: The Wild Project returned true while three
  other shows returned false; `collection` returned false for all four.

## Phase 5 status

- **Code:** complete at `1d845ee`.
- **Search:** the third engine patch exposes `shows`/`episodes`; the original
  four groups are unchanged and artwork is still rewritten by the catalog.
- **Library:** the fourth engine patch adds generic collection write/contains;
  the catalog has like/save/follow/show helpers, playlist add/remove and
  `YourEpisodes`. Saved shows use the `show` set.
- **Settings:** engine playback/cache options persist in the Cadence settings
  store and apply on an explicit engine restart; no settings UI yet.
- **Not yet exercised:** actual save/follow/unfollow writes against the real
  account, and the saved-shows listing. The UI for all new surfaces is Phase 6.
- **Next:** Phase 6 (Browse UI) after the user decides D1 (separate Browse
  window vs expanded compact window).

## Phase 6 tasks

### D1 decision — 2026-11-05

Separate Browse window (~960×640, opaque, normal frame) that mounts a dedicated
`BrowseApp` through `/?surface=browse`. The window is created on demand and
focuses an existing instance. It is skin-aware: a hidden shadow/light probe
extracts the active skin theme's custom properties (or Clarity's
`.clarity[data-theme]` tokens) and applies them to the Browse root. An optional
`browse.css` in the skin manifest lets a skin restyle the Browse subtree; the
compact pixel layout and the skin's `styles.css` are never injected into Browse.

### T6.1 — Browse window host and skin theme bridge

- [x] `platform.WindowService`: `OpenBrowse`, `CloseBrowse`, `BrowseOpen`.
      Create an opaque 960×640 (min 720×480) window named `browse` with
      `URL:"/?surface=browse"`, `Zoom:1`, and a normal frame; focus an existing
      one instead of duplicating.
- [x] `main.ts`: branch on `?surface=browse` with a dynamic import so the
      compact window does not load the Browse chunk.
- [x] `app/browse/BrowseApp.svelte` + shell: sidebar, top bar, content area and
      now-playing bar placeholders using `shared/ui`, i18n and the theme tokens.
      No full search/library yet (T6.2).
- [x] Compact app: a Browse button/shortcut that calls `openBrowse` through
      `shared/api/window`.
- [x] Theme bridge: hidden shadow probe for declarative skins (load
      `stylesheetUrl` + theme stylesheet, read the token list) and a hidden
      `.clarity[data-theme]` probe for the built-in skin. Apply tokens as inline
      custom properties on `.cadence-browse`; re-apply on `skins:changed`,
      theme changes and cross-window `storage` changes.
- [x] Optional skin `browse.css`: manifest schema `browse:{css}`, Go
      `Manifest.Browse`/validation/allowed-stylesheet handling, `SkinRuntime`
      `browseStylesheetUrl`, and a Browse `<link>` loaded after the base style.
      Existing skins stay valid; Clarity uses the default Browse styling.
- [x] Tests: manifest validation for `browse.css`, token extraction helper,
      theme re-application, Browse shell render, window-service compile.
- Commit: `feat(browse): add the separate skin-aware Browse window`

#### T6.1 verification — 2026-11-05

| Command | Result |
|---|---|
| `go test $(go list ./... \| grep -v '/node_modules/')` | exit 0 |
| `go vet $(go list ./... \| grep -v '/node_modules/')` | exit 0, no findings |
| `gofmt -l $(go list -f '{{.Dir}}' ./... \| grep -v '/node_modules/')` | exit 0, no files listed |
| `wails3 generate bindings -clean=true -ts -i .` | exit 0; 4 services, 55 methods, 40 models; only Windows service and SkinRuntime changed |
| `npm run check` | exit 0, 0 errors and 0 warnings |
| `npm run lint` | exit 0, no findings |
| `npm run format` | exit 0, all files match Prettier |
| `npm run test` | exit 0, 30 files / 149 tests |
| `npm run build` | exit 0 |
| `./scripts/build-go-librespot.sh --check` | exit 0, all patches apply |
| `git diff --check` | exit 0, no whitespace errors |

Live window smoke testing is **not** claimed: this environment cannot open the
GTK window. The unit and DOM checks cover the service's no-application path,
the Browse shell render and the Clarity token re-application. jsdom does not
compute `:host` custom properties inside a shadow root, so the declarative
shadow probe is not unit-tested; it mirrors the working `SkinHost.svelte`
pattern and needs a live window check.

### T6.2 — Search, library, entities and podcasts

- [x] Browse store/API: search with tracks/albums/artists/playlists/shows/
      episodes; library pages for playlists/albums/artists/Your Episodes;
      entity pages via `contextTracks`; navigation back stack. Saved-shows
      listing stays deferred as planned.
- [x] Use `VirtualList` for long context lists (the Browse window owns the scroll
      container, so this also closes the 800-row gate).
- [x] Entity actions: play, add to queue, like/save/follow/remove using the
      Phase 5 APIs; playlist add/remove; show save/remove.
- [x] Podcast show pages and Your Episodes; now-playing bar controls.
- [x] Use `t()` for all new strings and add the `browse.*` keys to `en`/`es`.
- [x] Tests for the browse store, search mapping, entity actions and the
      virtualized list.
- Commit: `feat(browse): add search, library and podcast surfaces`

#### T6.2 verification — 2026-11-05

| Command | Result |
|---|---|
| `npm run check` | exit 0, 0 errors and 0 warnings |
| `npm run lint` | exit 0, no findings |
| `npm run format` | exit 0, all files match Prettier |
| `npm run test` | exit 0, 35 files / 164 tests |
| `npm run build` | exit 0, 256 modules; `browse-*.js` 46.45 kB |
| `go test $(go list ./... \| grep -v '/node_modules/')` | exit 0, unchanged |
| `git diff --check` | exit 0, no whitespace errors |

Live UI behavior is **not** claimed: this environment cannot open the Browse
window. The store is covered with mocked audio calls (latest-search-wins,
paging, context retry/cancel, save/follow rollback, playlist add/remove) and
the views are covered with component tests including a virtualized long-list
case. No Go API was missing; no bindings changed.

### Commit map — Phase 6

| Task | Commit |
|---|---|
| T6.1 | `698a185` |
| T6.2 | `1996a77` |
| T6.3 | `b55a8c1` |
| Correction: LOCAL opens Browse | `fix(browse): route LOCAL directly to the Browse window` |
| Correction: one visible window | `fix(browse): show one window at a time` |
| Correction: one settings surface | `fix(settings): unify appearance and engine settings in Browse` |

### T6.3 — Settings, accessibility and polish

- [x] Settings panel for bitrate, crossfade, normalization, audio cache, FLAC,
      MPRIS, device group and image size; save + explicit restart confirmation.
- [x] Keyboard navigation, focus management, ARIA labels, empty/error/loading
      states; responsive at 960×640 and 720×480.
- [x] Visual polish with tokens only; no hardcoded dark colors that break light
      skin themes.
- Commit: `feat(browse): add settings and accessibility polish`

#### T6.3 verification — 2026-11-05

| Command | Result |
|---|---|
| `npm run check` | exit 0, 0 errors and 0 warnings |
| `npm run lint` | exit 0, no findings |
| `npm run format` | exit 0, all files match Prettier |
| `npm run test` | exit 0, 37 files / 177 tests |
| `npm run build` | exit 0, 259 modules; `browse-*.js` 65.32 kB |
| `go test $(go list ./... \| grep -v '/node_modules/')` | exit 0, unchanged |
| `git diff --check` | exit 0, no whitespace errors |

Live UI behavior is **not** claimed: this environment cannot open the Browse
window. jsdom covers the settings form/store, tab keyboard navigation, focus
on the entity back button and return to the section heading, and the save /
restart notices. Native select popups, real window resizing and actual engine
restart behavior were not exercised.

### T6.4 — Verification

- [x] Go tests/race/vet/gofmt, all four engine patches, bindings no diff,
      frontend check/lint/format/test/build.
- [ ] Live UI smoke: open Browse from compact, search podcasts, play, save/
      follow, change skin/theme, restart engine settings. (Pending: the
      environment cannot open the GTK window; the user runs `./bin/cadence`.)
- Commit: `docs: record Phase 6 verification`

### Phase 6 acceptance

- Browse opens as a separate window, focuses instead of duplicating, and does
  not disturb the compact skin window.
- The Browse window follows the active skin theme tokens; `browse.css` is
  optional and isolated to the Browse subtree.
- Search exposes shows/episodes; library pages, entity pages, podcasts and
  actions work; long lists are virtualized.
- Existing compact window behavior and performance are unchanged.

## Phase 6 verification — 2026-11-05

| Command | Result |
|---|---|
| `go test $(go list ./... \| grep -v '/node_modules/')` | exit 0, all packages ok |
| `go test -race ./internal/engine/... ./internal/catalog/... ./internal/settings/... ./internal/wailsapi/audio/... ./internal/wailsapi/player/...` | exit 0 |
| `go vet`, `gofmt -l` | exit 0, no findings |
| `./scripts/build-go-librespot.sh --check` | exit 0, all four patches apply with `--fuzz=0` |
| `wails3 generate bindings -clean=true -ts -i .` | exit 0, 4 services / 55 methods / 40 models; no diff after commit |
| `npm run check` | exit 0, 0 errors / 0 warnings |
| `npm run lint`, `npm run format` | exit 0 |
| `npm run test` | exit 0, 37 files / 177 tests |
| `npm run build` | exit 0, 259 modules; `browse-*.js` 65.32 kB |
| `task build` | exit 0; `bin/cadence` produced (16 MB) |
| `git diff --check` | exit 0 |

## Phase 6 status

- **Code:** complete at `b55a8c1`, with the LOCAL→Browse routing correction
  above; the full Wails app builds as `bin/cadence`.
- **Window:** Browse is a separate 960×640 opaque window opened directly from
  the compact `LOCAL` badge (Library section); re-opening focuses the existing
  window and sends `browse:navigate`. Opening Browse hides the compact player
  (without destroying it) and closing Browse restores it. It is a native-frame
  window, not a shaped/transparent skin window.
- **Skin-aware:** the active skin’s theme tokens are read through a hidden
  shadow/Clarity probe and applied inline; the compact `styles.css` is never
  injected. An optional manifest `browse.css` lets a skin restyle the Browse
  subtree; Lumina has a working example. Clarity uses the default Browse styling
  plus its `.clarity[data-theme]` tokens.
- **Surfaces:** six-group search (tracks/albums/artists/playlists/shows/episodes),
  paged library (playlists/albums/artists/Your Episodes, Liked Songs card),
  entity pages with a back stack, show/episode browsing, like/save/follow,
  playlist add/remove, a now-playing bar and unified Browse settings: engine
  playback/cache/features with an explicit restart confirmation, appearance
  (theme/visualizer/motion) and skins management.
- **Performance:** entity track lists use `VirtualList` at a 44 px pitch; the
  Browse window owns its scroll container.
- **Pending live smoke:** the environment cannot open the GTK window. The user
  should run `./bin/cadence`, confirm the compact badge says Browse, open Browse
  from it, restore the compact player with `Back to player`/native close, then
  open Settings from the compact button and change appearance/skins; also
  search podcasts, play an episode, save/follow and restart the engine.
- **Deferred:** saved-shows listing (the `show` collection set is identified),
  per-row liked hearts (the now-playing bar has the current-track like), any
  native window resize/frame polish if the live smoke finds it, and removal of
  the now-unused `MusicBrowser.svelte`, `SettingsPopover.svelte` and
  `SkinManager.svelte` in Phase 7.
- **Next:** live smoke, then Phase 7 (i18n sweep/tests/docs) or Phase 8
  (packaging).

### Correction — 2026-11-05: LOCAL opens Browse directly

The compact flow previously opened the old in-window Library panel first and
required a second `Browse` click. The correction routes `LOCAL` straight to the
dedicated window.

- [x] `platform.WindowService.OpenBrowse(section)`: validates
      `search`/`library`/`podcasts`/`settings` (empty = `library`), opens
      `/?surface=browse&section=<section>`, and focuses + emits
      `browse:navigate` for an existing window.
- [x] Registered the `string` Browse navigation event in `main.go`; regenerated
      bindings and added the typed `onBrowseNavigate` facade.
- [x] Compact app: `LOCAL`/skin handlers call `openBrowse("library")`; removed
      `musicOpen`, the `MusicBrowser` render, and all size/shortcut references.
      Provider badge strings and the Skin Manager link now say Browse; pair copy
      points at Browse.
- [x] Browse owns pairing: the entry starts `music`, reads `section=`, the shell
      subscribes to `browse:navigate`, and `EngineGate` handles engine
      unavailable, pairing, starting and copy/reset states. Settings remains
      reachable before authentication.
- [x] Tests: window section forwarding, Browse event unwrapping, App redirect
      without the old panel, EngineGate states/copy/reset, initial section and
      navigation event, plus updated Player copy.
- Commit: `fix(browse): route LOCAL directly to the Browse window`

#### Correction verification — 2026-11-05

| Command | Result |
|---|---|
| `go test $(go list ./... \| grep -v '/node_modules/')` | exit 0 |
| `go vet $(go list ./... \| grep -v '/node_modules/')` | exit 0, no findings |
| `gofmt -l $(go list -f '{{.Dir}}' ./... \| grep -v '/node_modules/')` | exit 0, no files listed |
| `wails3 generate bindings -clean=true -ts -i .` | exit 0; 4 services / 55 methods / 4 events; regenerated binding diff |
| `npm run check` | exit 0, 0 errors / 0 warnings |
| `npm run lint`, `npm run format` | exit 0 |
| `npm run test` | exit 0, 38 files / 183 tests |
| `npm run build` | exit 0, 252 modules; `browse-*.js` 75.23 kB |
| `task build` | exit 0; `bin/cadence` produced |
| `./scripts/build-go-librespot.sh --check` | exit 0, all patches apply |
| `git diff --check` | exit 0 |

Live GTK flow is **not** claimed: the user will verify that clicking `LOCAL`
opens the Browse window directly in the Library section and that a second click
focuses it without a duplicate.

### Correction — 2026-11-05: one visible window at a time

Opening Browse now hides the compact player instead of leaving both windows
visible. The compact window is only hidden, never destroyed, so playback and
state survive. Closing Browse restores it.

- [x] `OpenBrowse(section)` shows/focuses Browse, emits `browse:navigate` for an
      existing window, then hides `main` only after Browse is visible. A failed
      show leaves `main` visible.
- [x] New Browse windows register a `WindowClosing` listener that shows and
      focuses `main`; the close itself is not cancelled. `CloseBrowse` also
      restores `main` explicitly as a fallback.
- [x] Added the discoverable `Back to player` sidebar action calling
      `closeBrowse()`; the native close paths remain supported.
- [x] Updated docs and tests; the swap itself is user-verified because no live
      GTK window can be opened here.
- Commit: `fix(browse): show one window at a time`

#### Correction verification — 2026-11-05

| Command | Result |
|---|---|
| `go test $(go list ./... \| grep -v '/node_modules/')` | exit 0 |
| `go vet $(go list ./... \| grep -v '/node_modules/')` | exit 0, no findings |
| `gofmt -l $(go list -f '{{.Dir}}' ./... \| grep -v '/node_modules/')` | exit 0, no files listed |
| `npm run check` | exit 0, 0 errors / 0 warnings |
| `npm run lint`, `npm run format` | exit 0 |
| `npm run test` | exit 0, 38 files / 184 tests |
| `npm run build` | exit 0, 252 modules |
| `task build` | exit 0; `bin/cadence` produced |
| `git diff --check` | exit 0 |

The live hide/show swap is not claimed; the user will verify that opening
Browse hides the compact player and closing Browse restores it.

### Correction — 2026-11-05: one settings surface in Browse

The compact settings button and skin settings actions now open Browse Settings;
the compact settings/skin overlays are no longer reachable.

- [x] Compact provider badge shows `Browse`/`Explorar` for every provider; queue
      notes use localized playback/demo labels; compact player engine copy no
      longer says “Local”.
- [x] Compact App opens `openBrowse("settings")` from the settings button and
      skin handlers; removed `settingsOpen`/`skinManagerOpen`, the popover and
      SkinManager renders, and all window sizing/keyboard references.
- [x] Browse `SettingsView` adds appearance (theme, visualizer, motion) and
      skins (catalog, activate, install, uninstall, reset, third-party toggle)
      alongside the engine settings. Appearance and skins stay available if the
      engine settings fail to load. Engine restart confirmation is unchanged.
- [x] Preferences mirror theme, visualizer and motion across windows through
      `startStorageSync()` called by both surfaces; the Browse-only storage
      listener was removed.
- [x] Tests updated for the Browse Settings redirect, appearance/skins actions
      and preference storage sync.
- Commit: `fix(settings): unify appearance and engine settings in Browse`

#### Correction verification — 2026-11-05

| Command | Result |
|---|---|
| `npm run check` | exit 0, 0 errors / 0 warnings |
| `npm run lint`, `npm run format` | exit 0 |
| `npm run test` | exit 0, 38 files / 188 tests |
| `npm run build` | exit 0, 248 modules; `browse-*.js` 83.36 kB |
| `go test $(go list ./... \| grep -v '/node_modules/')` | exit 0 |
| `task build` | exit 0; `bin/cadence` produced |
| `git diff --check` | exit 0 |

Live GTK behavior is not claimed; the user will verify the Browse badge label,
the compact settings redirect and the appearance/skin controls.

## Phase 7 tasks

### T7.1 — Dead code removal and i18n guard

- [ ] Delete the unreachable compact components: `MusicBrowser.svelte`,
      `SearchRow.svelte`, `LibrarySkeleton.svelte`, `MediaCover.svelte`,
      `SettingsPopover.svelte`, `skins/SkinManager.svelte` (and any tests that
      only cover them). Remove the unused imports/mocks.
- [ ] Trim `music.svelte.ts` to the engine/pairing state used by Browse's
      EngineGate (audio, pairing, bus, refresh/reset/restart); remove the
      library/search/context methods that only the deleted MusicBrowser used.
- [ ] Add `frontend/scripts/check-i18n.mjs` (Svelte compiler AST walk: literal
      text nodes and literal `title`/`aria-label`/`placeholder`/`alt` values,
      with a small brand/symbol allowlist). Add `npm run i18n:check` and run it
      in the Taskfile/CI frontend checks.
- [ ] Add a language selector to Browse Settings (English/Español) using
      `i18n.setLocale`; add cross-window locale sync on the `cadence.locale`
      storage key without write loops; both windows update.
- Commit: `chore(i18n): remove dead UI and add the translation guard`

### T7.2 — Compact and skin i18n sweep

- [ ] Migrate every user-facing literal in the live compact and declarative-skin
      components (TitleBar, Player, TrackInfo, Transport, Progress, Volume,
      Queue, DevicePopover, DevicePicker, Repeat, Shuffle, SettingsButton,
      AlbumArt, Visualizer, App error/notice copy) to `t()` keys.
- [ ] Localize the visualizer mode labels and any remaining compact copy; keep
      brand names (`CADENCE`, `Clarity`, `Spotify`) as proper nouns.
- [ ] Complete `es.ts` with neutral Spanish for every key; keep `en.ts` as the
      typed source. Add the language selector labels.
- [ ] Error copy: map the existing `PlayerError` codes (`not_authenticated`,
      `audio_unavailable`, etc.) through catalog keys; keep unknown raw errors
      as a diagnostic detail.
- [ ] Tests: locale switch in compact components, key parity, ICU syntax, the
      i18n guard script, and the language selector.
- Commit: `feat(i18n): translate the compact player and skin UI`

### T7.3 — Error codes, docs and verification

- [ ] Add a typed `ErrorCode` to `engine.AudioState` for the known engine
      states (not installed, start failed, not ready, disabled in demo, not
      running) and set it at the source; expose it through the bindings and map
      it in the frontend. Keep the raw `Error` string as a detail.
- [ ] Add `docs/i18n.md` (catalog workflow, key groups, locale sync, guard,
      backend error policy) and update `docs/README.md`, `docs/architecture.md`,
      `docs/development.md` and the README feature list.
- [ ] Run the full gates: Go tests/race/vet/gofmt, all four engine patches,
      bindings no diff, `npm run check/lint/format/i18n:check/test/build`,
      `task build`, `git diff --check`.
- Commit: `docs: record the i18n and Phase 7 verification`

### Phase 7 acceptance

- No user-facing literal remains in the live `.svelte` UI outside the catalogs;
  the guard script enforces it.
- The language selector switches English/Spanish in both windows and persists.
- The compact player, declarative skins and Browse are fully translated.
- Known engine/player error states use catalog keys; unexpected errors keep a
  raw diagnostic detail.
- The dead compact Library/Settings/SkinManager code is gone.

## Progress log

| Date | Task | Commit | Result |
|---|---|---|---|
| 2026-11-04 | Plan | `5cb1d2b` | Plan committed; no code changes |
| 2026-11-04 | T0.1 | `5e7449f` | Recorder, snapshot and transport wrapper with tests |
| 2026-11-04 | T0.2 | `db2f968` | Artcache/player instrumentation, loopback stats and pprof, 12 s timeout preserved |
| 2026-11-04 | T0.3 | `67143fe` | `--check` mode, `audio:check`/`verify` tasks, CI step |
| 2026-11-04 | T0.4 | `939e949` | Baseline procedure + static estimate + gates; ADRs 0001-0003 |
| 2026-11-04 | T0.5 | `14644ca` | All verification commands green |
| 2026-11-04 | Review corrections | `1e02e7f` | Atomic artwork recorder, `missing` counter, successful-write-only `served`, emission-only event accounting, corrected baseline provenance, transport edge cases, strict patch fuzz; all checks green |
| 2026-11-04 | T1.1 | `2b10f55` | Pin updated to `16b65bb4`; both patches apply unchanged with `--fuzz=0`; `task audio:build` exit 0 |
| 2026-11-04 | T1.2 | `05adf37` | `device_is_group` config (default off), validated `server.image_size`, `accesspoint_ports` note removed in correction round, config tests added |
| 2026-11-04 | T1.3 | `fb41f3c` | Log parsing for `running go-librespot`, `Runtime.Version()`, `AudioState.EngineVersion`, `engine_version` in `/debug/stats`, tests |
| 2026-11-04 | T1.4 | `2b7a982` | Pin and engine options documented; both patches documented; Phase 1 verification green; live Spotify checks pending user |
| 2026-11-04 | Engine version fix | `f2efa8e` | Injected `-X ...version` and `-buildvcs=false`; rebuilt binary now logs the real pin `0.10.6-20261104085532-16b65bb4cd2b` |
| 2026-11-04 | Review corrections 2 | `59dc4d4` | `device_is_group` default off and group-icon-only docs; generated-config notice; version injection `${VERSION#v}` with `-buildvcs=false`; version cache cleared on start/stop with throttled `State` retry; stricter config/version tests; GPL notice updated |
| 2026-11-04 | T2.1 | `e4c9cb7` | Typed `shared/api` boundary with ApiError normalization, player/audio/skins/window/event facades and client/event/window tests; all binding imports migrated |
| 2026-11-04 | T2.2 | `c0b8c20` | `intl-messageformat@12.1.4`, literal-key TS catalogs with compile-time parity, cached ICU formatters, locale detection/persistence and `Intl` helpers; ADR-0003 and plan §7 updated |
| 2026-11-04 | T2.3 | `7adecc5` | Button, focus-trapped Dialog, virtual list helper/component, Skeleton, toast store/components and typed navigation store; not wired into App yet |
| 2026-11-04 | T2.4 | `9210648` | All frontend checks green; 23 test files / 106 tests; build emits lazy `es` chunks; binding imports confined to `shared/api` and mocks |
| 2026-11-04 | Phase 2 corrections | `4a28390` | Rounded seek/volume, non-blocking i18n startup, latest-wins locale switches, dialog focus/escape hardening, fractional virtual rows, Button attribute forwarding, single live region, lazy `es` barrel fix, extra i18n tests and docs |
| 2026-11-04 | Phase 2 final fixes | `fix(frontend): skip non-tabbable dialog elements and stale locale errors` | Excluded `tabindex="-1"` from the dialog trap; ignored stale locale loader rejections; 124 tests green |
| 2026-11-04 | T3.1 | `61d3ba1` | `engine` owns supervisor/rest/DTOs, `artwork` replaces `artcache`; old adapter imports the new packages; Go tests, vet and gofmt green |
| 2026-11-04 | T3.2 | `ca2dc58` | `catalog` owns engine-backed library/search/context orchestration; `playback` owns provider, state, demo and engine provider; temporary adapters delegate; Go tests, vet and gofmt green |
| 2026-11-04 | T3.3 | `8b8ab2e` | Wails adapters moved to `internal/wailsapi/{audio,player,skins}` with `audioapi`/`playerapi`/`skinsapi`; old packages deleted; bindings regenerated (4 services, 39 methods, 38 models, 3 events); frontend facades and mocks repointed; Go and frontend checks green |
| 2026-11-04 | T3.4 | `d8887e4` | ADR-0001 accepted with the final layout and `internal/platform` exception; `docs/architecture.md` linked from README and docs index; verification table recorded; all Go, engine, binding and frontend checks green |
| 2026-11-04 | T3.4 review correction 1 | `b16a453` | Added `internal/catalog/service_test.go` (`catalog_test`) against a fake engine: demo-disabled state, library pages/artwork rewriting, context mapping, all search groups, playback/queue/radio bodies, 500 propagation and missing-client error; `go test -cover ./internal/catalog/` = 80.4% |
| 2026-11-04 | T3.4 review correction 2 | `85de8a9` | README repository tree, plan header and docs index aligned with the Phase 3 split; ADR-0001 constructors/logging claims corrected; architecture Wails boundary includes `main.go`; performance-baseline paths point at the new packages without changing measurements |
| 2026-11-04 | Phase 3 close | `docs(odd): close Phase 3` | Split complete at 85de8a9; 80.4% catalog coverage; bindings and frontend green; live smoke test and runtime baseline pending |
| 2026-11-04 | T4.1 | `feat(state)` | `/events` WebSocket client, `Streamer` + revisioned `EngineProvider.Stream` (100 ms coalesce, 15 s reconcile, change-only emit), on-demand `Provider.Queue`, streaming player adapter with fallback ticker, frontend queue/position/revision stores and authenticated-only audio polling; Go race and frontend checks green |
| 2026-11-04 | T4.1 review corrections | `fix(state)` | Stopped state on engine loss, capture-ordered revisions, queue loading placeholders, interpolated skin progress, track-scoped queue reload, subscribe-before-fetch, watcher-before-fetch with reconcile dirt clearing, demo state without the queue and reconnect logging |
| 2026-11-04 | T4.2 | `perf(art)` | `GET /art/{key}?size=` variants (64/128/240/640, JPEG q85, no upscale), per-`key@size` single-flight, 256 MiB disk LRU of resized variants with scan/write eviction, `disk_hit` diagnostics, 256-entry/64 MiB memory bounds, `coverURL`/`coverLoading` wired into the frontend covers |
| 2026-11-04 | T4.3 | `perf(ui)` | Progress and both visualizers now start rAF loops only while playing, visible and (for visualizers) not reduced-motion; loops cancel on pause/hide with a final static frame; visualizer frame cap at 30 fps; skin progress paused-state and unmount cleanup covered by tests; context-list virtualization deferred to Phase 6 (34 px row + 2 px gap, shared scroll container) |

| 2026-11-04 | Phase 4 verification | `e301b99` | Static gates green; idle structure is 0.067 req/s; after-baseline pending reference-machine run |
| 2026-11-04 | T4.4 after-baseline | `docs: record the Phase 4 after-baseline` | Measured 0.067 req/s (4/60 s), 541 B/event, 0 idle events; CPU/RSS caveats recorded; Phase 5 not started |

| 2026-11-05 | T5.1 | `ed74d9d` | Search-groups patch exposes shows/episodes (raw `audioepisodes` mapped to `episodes`), new hit fields, catalog artwork rewriting, frontend store mapping; live `/search` returned 5 shows + 5 episodes |
| 2026-11-05 | T5.2 | `8d069d3` | Fourth engine patch adds generic `collection`/`artist`/`show` write and contains routes; engine client, catalog typed use cases (`SetLiked`, `SaveAlbum`, `SaveShow`, `FollowArtist`, playlist add/remove, `YourEpisodes`), adapter, bindings, fake-engine tests and docs; no live writes performed |
| 2026-11-05 | T5.2 correction | `b01bfcb` | Live read-only `contains` check: `set:"show"` returned `true` for `spotify:show:5iKz9gAsyuQ1xLG6MFLtQg` (The Wild Project) and `false` for three other shows, while `set:"collection"` returned `false` for all four. `SaveShow`/`IsShowSaved` now use `show`; liked tracks/albums stay on `collection`, follows on `artist`; saved-shows listing stays Phase 6 |
| 2026-11-05 | T5.3 | `1d845ee` | `settings.EngineState` persists bitrate/crossfade/normalisation/cache/FLAC/MPRIS/device-group/image-size with defaults; engine `PlaybackConfig` normalizes and applies under the supervisor mutex; generated config emits the new keys; adapter `Get`/`SetEngineSettings` persists without restarting; bindings and typed facade ready; tests for defaults, round-trip, normalization, config generation and adapter persistence |

| 2026-11-05 | T5.4 | `docs(odd): record Phase 5 verification` | All Go/engine/binding/frontend gates green; live read-only search and collection checks passed; actual writes and UI remain Phase 6 |
| 2026-11-05 | T6.1 | `698a185` | Separate `browse` window host (open/close/status, focus instead of duplicate), `?surface=browse` dynamic entry, app-owned Browse shell with sidebar/top bar/now-playing, hidden shadow/Clarity token probe applied inline with storage re-sync, optional manifest `browse.css` with schema/validation/runtime URL, Lumina example; Go and frontend checks green; live window smoke not run |
| 2026-11-05 | T6.2 | `1996a77` | Browse store with latest-wins six-group search, paged library tabs, retried entity contexts and back stack; Search/Library/Podcasts/Entity views, VirtualList context rows (44 px), save/follow/like and playlist add/remove with optimistic rollback, now-playing heart and playlist picker; no Go or binding changes; 35 files / 164 frontend tests green; live UI not run |
| 2026-11-05 | T6.3 | `b55a8c1` | Engine settings view/store (bitrate, crossfade, normalisation, cache, FLAC, MPRIS, device group, image size), explicit Dialog restart flow, tablist/roving keyboard focus, search/entity focus management, loading/empty/error-retry and aria-busy states, responsive token-only styling, tooltip labels; 37 files / 177 frontend tests green; live UI/native restart not run |

| 2026-11-05 | T6.4 | `docs(odd): record Phase 6 verification` | Go/engine/binding/frontend gates green; full `task build` produces `bin/cadence`; Browse window, search/library/entities/podcasts/settings code complete; live GTK smoke pending user |
| 2026-11-05 | Correction: LOCAL opens Browse | `fix(browse): route LOCAL directly to the Browse window` | `OpenBrowse(section)` + `browse:navigate`, compact App redirect without the MusicBrowser panel, Browse-owned music/pairing `EngineGate`, initial section parsing, provider/pair copy and tests; Go/frontend gates green; live GTK flow pending user |
| 2026-11-05 | Correction: one visible window | `fix(browse): show one window at a time` | Opening Browse hides the non-destroyed compact window; Browse close (native or `Back to player`) restores/focuses it, with an explicit `CloseBrowse` fallback; docs/tests updated; live GTK swap pending user |
| 2026-11-05 | Correction: one settings surface | `fix(settings): unify appearance and engine settings in Browse` | Compact badge says Browse; queue notes localized; compact settings/skin overlays replaced by `openBrowse("settings")`; Browse Settings adds appearance and skins management; cross-window preference storage sync; tests updated; live GTK redirect pending user |
