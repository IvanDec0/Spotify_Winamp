# Cadence v2 — Execution Tracker

> **Design source:** [`../plan/cadence-v2.md`](../plan/cadence-v2.md)
> **Branch:** `feat/cadence-v2`
> **Authorized scope:** Phase 0 (complete) and Phase 1 (user approved on
> 2026-11-04). The runtime baseline capture is explicitly deferred by the user;
> capture it before the Phase 4 performance work.
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
- Do not change the go-librespot pin, patches, or product features (Phase 1+).
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
- [x] `main.go`: enable `DeviceIsGroup` so Spotify Jam can include Cadence.
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

- [ ] Update the pin in `README.md`, `plan/nombre_plan.md` and
      `docs/performance-baseline.md`.
- [ ] Document `device_is_group`, `accesspoint_ports` and `server.image_size` in
      `docs/spotify-setup.md`; mention both patches in `docs/development.md`.
- [ ] Run and record: `go test`, `go vet`, `gofmt`, `task audio:check`,
      `task audio:build`.
- Commit: `docs: document the engine upgrade and options`

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
| 2026-11-04 | T1.1 | `build: pin go-librespot master for device_is_group support` | Pin updated to `16b65bb4`; both patches apply unchanged with `--fuzz=0`; `task audio:build` exit 0 |
| 2026-11-04 | T1.2 | `feat(engine): advertise as a group and expose image size config` | `device_is_group: true`, validated `server.image_size`, `accesspoint_ports` override documented, config tests added |
| 2026-11-04 | T1.3 | `feat(engine): surface the running go-librespot version` | Log parsing for `running go-librespot`, `Runtime.Version()`, `AudioState.EngineVersion`, `engine_version` in `/debug/stats`, tests |
