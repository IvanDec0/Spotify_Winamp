# Cadence v2 — Execution Tracker

> **Design source:** [`../plan/cadence-v2.md`](../plan/cadence-v2.md)
> **Branch:** `feat/cadence-v2`
> **Authorized scope:** Phase 2 (user approved on 2026-11-04) after Phase 0 and
> Phase 1. The runtime baseline capture remains explicitly deferred until before
> the Phase 4 performance work.
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

- [ ] `src/shared/ui`: `Button.svelte`, `Dialog.svelte` (focus trap, Escape,
      backdrop, `aria-modal`), `VirtualList.svelte` + pure `visibleRange` helper,
      `Skeleton.svelte`, `Toast.svelte` + `toasts.svelte.ts` store,
      `ToastHost.svelte`.
- [ ] `src/app/navigation.svelte.ts`: typed panels and route stack, with tests.
      It is not wired into App.svelte in this phase; Phase 6 does that.
- [ ] Component tests for Dialog, VirtualList, Toast and the navigation store.
- Commit: `feat(ui): add shared primitives and navigation state`

### T2.4 — Verification

- [ ] `npm run check`, `npm run lint`, `npm run format`, `npm run test`,
      `npm run build`.
- [ ] Confirm `rg 'bindings/|@wailsio/runtime' frontend/src` only matches
      `src/shared/api` and test mock paths.
- [ ] Record exact results in this tracker.
- Commit: `chore: record Phase 2 verification`

### Phase 2 deviations

- App.svelte keeps the existing panel booleans; the new navigation store is
  introduced and tested but wired in Phase 6 to keep Phase 2 behavior-preserving.
- i18n catalogs are TS modules with literal key types instead of JSON plus a
  generated `.d.ts`; this removes a build step and keeps key parity compile-time.
- Existing screens are not translated yet; Phase 7 performs the sweep.

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
| 2026-11-04 | T2.1 | `refactor(frontend)` | Typed `shared/api` boundary with ApiError normalization, player/audio/skins/window/event facades and client/event/window tests; all binding imports migrated |
| 2026-11-04 | T2.2 | `feat(i18n)` | `intl-messageformat@12.1.4`, literal-key TS catalogs with compile-time parity, cached ICU formatters, locale detection/persistence and `Intl` helpers; ADR-0003 and plan §7 updated |
