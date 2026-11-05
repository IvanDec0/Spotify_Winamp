# Browse Player Parity and Real Visualizer

> **Status:** In progress.
> **Branch:** `feat/cadence-v2`
> **Scope:** Bring the Browse now-playing bar to parity with the compact player
> (shuffle, repeat, seek, mute, queue) and replace the simulated visualizer with
> real audio analysis in both the compact and Browse windows.

## Constraints

- go-librespot's pipe output is a backend, not a tee: enabling it would replace
  PulseAudio. Do not use it for visualization.
- Real analysis captures the PulseAudio/PipeWire monitor via `parec`
  (`@DEFAULT_MONITOR@`). It reflects the system output, not only Cadence; the
  simulated visualizer remains the fallback when capture is unavailable.
- No new engine patch; no Web API.
- Keep the two-window/single-visible flow and the i18n guard intact.

## Tasks

### P1 — Browse player parity

- [x] `NowPlayingBar.svelte`: add shuffle and repeat buttons with active state
      (`player.state.shuffle`, `player.state.repeat`) and the same
      toggle/cycle actions as the compact player.
- [x] Replace the non-interactive progress rail with a seek range (disabled when
      `canSeek` is false), using `player.seek` and keeping the interpolated
      position.
- [x] Add a real mute button on the volume icon: save the last non-zero volume,
      set 0, restore on second click; keep the slider.
- [x] Add a Browse `queue` section: sidebar entry, `QueueView.svelte` using
      `player.queue`/`loadQueue`, current/next tags, covers, durations, empty
      state and reload on track change while open.
- [x] i18n keys for shuffle/repeat/seek/mute/queue; guard at 0. Tests for the
      bar controls and the queue view.
- Commit: `feat(browse): add player controls and queue`

#### P1 verification — 2026-11-05

| Command | Result |
|---|---|
| `npm run i18n:check` | exit 0, 0 findings |
| `npm run check` | exit 0, 0 errors / 0 warnings |
| `npm run lint` | exit 0, no findings |
| `npm run format` | exit 0 |
| `npm run test` | exit 0, 40 files / 223 tests |
| `npm run build` | exit 0, 252 modules |
| `go test $(go list ./... \| grep -v '/node_modules/')` | exit 0 (unchanged) |
| `task build` | exit 0; `bin/cadence` produced |
| `git diff --check` | exit 0 |

The bar keeps its existing heart/transport layout and adds shuffle, repeat,
seek and mute; queue rows are display-only. Live GTK verification is pending
the user.

### V1 — Real visualizer

- [x] `internal/visualizer`: pure analyzer (1024-sample FFT, ~24 log bands,
      64-point waveform, RMS, attack/decay smoothing) with sine/silence tests.
- [x] Linux capture via `parec` on `@DEFAULT_MONITOR@` (s16le/44.1k/mono), with
      start/stop, bounded reads, no Wails imports; fallback when `parec` or the
      monitor is unavailable.
- [x] `internal/wailsapi/visualizer`: `SetActive(clientID, active)` with a
      client set; start capture on the first active client and stop on the last;
      emit `visualizer:frame` at ≤30 Hz; register the event/service in `main.go`.
- [x] Frontend `visualizer-data.svelte.ts`: subscribe to frames, track freshness,
      call `setActive` per window with a unique client ID; fallback to the
      simulated visualizer when stale.
- [x] Wire the compact `Visualizer.svelte` and skin `Visualizer.svelte` to real
      bands/waveform when fresh; keep the simulated math as fallback.
- [x] Add a compact real spectrum to the Browse now-playing bar
      (`BrowseVisualizer.svelte`), using the same frames and fallback.
- [x] Tests: analyzer, client set, freshness/fallback, visualizer rendering.
- Commit: `feat(audio): add a real monitor-driven visualizer`

#### V1 verification — 2026-11-05

| Command | Result |
|---|---|
| `go test ./internal/visualizer/... ./internal/wailsapi/visualizer/...` | exit 0 |
| `go test -race ./internal/visualizer/... ./internal/wailsapi/visualizer/...` | exit 0 |
| `go test $(go list ./... \| grep -v '/node_modules/')` | exit 0 |
| `go vet $(go list ./... \| grep -v '/node_modules/')` | exit 0 |
| `gofmt -l internal/visualizer internal/wailsapi/visualizer main.go` | exit 0, no files |
| `wails3 generate bindings -clean=true -ts -i .` | regenerated; identical on rerun |
| `npm run i18n:check` | exit 0, 0 findings |
| `npm run check` | exit 0, 0 errors / 0 warnings |
| `npm run lint` | exit 0, no findings |
| `npm run format` | exit 0 |
| `npm run test` | exit 0, 43 files / 235 tests |
| `npm run build` | exit 0, 258 modules |
| `task build` | exit 0; `bin/cadence` produced |
| `git diff --check` | exit 0 |

`parec` was exercised locally for 1.5 s: 31 frames were captured and analyzed
with no errors. Live playback reaction is still pending user verification; the
monitor captures system-wide output on Linux/PulseAudio-PipeWire.

### V2 — Verification

- [ ] Go tests/race/vet/gofmt, engine patch check, bindings no diff,
      `npm run i18n:check`, frontend check/lint/format/test/build, `task build`,
      `git diff --check`.
- [ ] Live smoke: play audio, confirm the compact and Browse animations react;
      pause/seek/shuffle/repeat/mute; open the Browse queue.

## Acceptance

- The Browse bar has shuffle, repeat, seek, mute and queue parity.
- The visualizer follows the actual output in both windows; it degrades to the
  simulated animation if capture is unavailable.
- Idle behavior is preserved: capture stops when paused/hidden/inactive.
- No literals break the i18n guard; no new polling of the engine.

## Known limitations

- Monitor capture is system-wide; other apps' audio can influence the
  visualization.
- The approach is Linux/PulseAudio-PipeWire only (the app targets Arch Linux).

## Progress log

| Date | Task | Commit | Result |
|---|---|---|---|
| 2026-11-05 | P1 | `c3d4529` | Browse shuffle/repeat/seek/mute, queue section with `QueueView`, i18n keys and tests; all frontend/Go/build gates green; live GTK verification pending user |
| 2026-11-05 | V1 | `f0fbceb` | Monitor analyzer and `parec` capture, Wails client-set service/event, real-frame rendering in the compact, skin and Browse visualizers with simulated fallback; all gates green; live playback check pending user |

| 2026-11-05 | V1 correction | `fix(visualizer): release capture on pagehide` | Closing the Browse/compact webview releases its visualizer client via `pagehide`/`beforeunload`, so a stale client cannot keep the PulseAudio monitor capture alive; test added |
