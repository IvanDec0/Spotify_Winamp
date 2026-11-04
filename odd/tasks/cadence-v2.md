# Cadence v2 — Execution Tracker

> **Design source:** [`../plan/cadence-v2.md`](../plan/cadence-v2.md)
> **Branch:** `feat/cadence-v2`
> **Authorized scope:** Phase 0 only (user approved "Arranca" on 2026-11-04).
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

- [ ] `artcache` and `player.Service` accept an optional recorder (setter, nil-safe).
- [ ] `main.go`: `--diagnostics` flag, loopback listener on `127.0.0.1:0`,
      `/debug/stats` and `/debug/pprof/*`, logged URL, clean shutdown.
- [ ] Preserve the 12 s engine HTTP timeout when wrapping the transport.
- Commit: `feat(diagnostics): expose loopback stats and pprof behind a flag`

### T0.3 — Engine patch verification in CI

- [ ] `scripts/build-go-librespot.sh --check` applies both patches and exits
      without building.
- [ ] `Taskfile.yml`: `audio:check` and `verify` tasks.
- [ ] CI runs `task audio:check`.
- Commit: `build: verify go-librespot patches without building`

### T0.4 — Baseline and decision records

- [ ] `docs/performance-baseline.md`: procedure, static baseline, measurement
      table to fill on the reference machine, Phase 4 gates.
- [ ] `docs/adr/0001-module-boundaries.md`
- [ ] `docs/adr/0002-event-driven-engine-state.md`
- [ ] `docs/adr/0003-error-codes-and-i18n.md`
- Commit: `docs: add performance baseline and v2 ADRs`

### T0.5 — Phase 0 verification

- [ ] `go test $(go list ./... | grep -v '/node_modules/')`
- [ ] `go vet $(go list ./... | grep -v '/node_modules/')`
- [ ] `gofmt -l` over non-vendored Go directories
- [ ] `./scripts/build-go-librespot.sh --check`
- [ ] Record exact results in this file.
- Commit: `chore: record Phase 0 verification`

## Progress log

| Date | Task | Commit | Result |
|---|---|---|---|
| 2026-11-04 | Plan | `5cb1d2b` | Plan committed; no code changes |
| 2026-11-04 | T0.1 | `feat(diagnostics): add engine and artwork metrics recorder` | Recorder, snapshot and transport wrapper with tests |
