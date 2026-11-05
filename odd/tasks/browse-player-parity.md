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

- [ ] `NowPlayingBar.svelte`: add shuffle and repeat buttons with active state
      (`player.state.shuffle`, `player.state.repeat`) and the same
      toggle/cycle actions as the compact player.
- [ ] Replace the non-interactive progress rail with a seek range (disabled when
      `canSeek` is false), using `player.seek` and keeping the interpolated
      position.
- [ ] Add a real mute button on the volume icon: save the last non-zero volume,
      set 0, restore on second click; keep the slider.
- [ ] Add a Browse `queue` section: sidebar entry, `QueueView.svelte` using
      `player.queue`/`loadQueue`, current/next tags, covers, durations, empty
      state and reload on track change while open.
- [ ] i18n keys for shuffle/repeat/seek/mute/queue; guard at 0. Tests for the
      bar controls and the queue view.
- Commit: `feat(browse): add player controls and queue`

### V1 — Real visualizer

- [ ] `internal/visualizer`: pure analyzer (1024-sample FFT, ~24 log bands,
      64-point waveform, RMS, attack/decay smoothing) with sine/silence tests.
- [ ] Linux capture via `parec` on `@DEFAULT_MONITOR@` (s16le/44.1k/mono), with
      start/stop, bounded reads, no Wails imports; fallback when `parec` or the
      monitor is unavailable.
- [ ] `internal/wailsapi/visualizer`: `SetActive(clientID, active)` with a
      client set; start capture on the first active client and stop on the last;
      emit `visualizer:frame` at ≤30 Hz; register the event/service in `main.go`.
- [ ] Frontend `visualizer-data.svelte.ts`: subscribe to frames, track freshness,
      call `setActive` per window with a unique client ID; fallback to the
      simulated visualizer when stale.
- [ ] Wire the compact `Visualizer.svelte` and skin `Visualizer.svelte` to real
      bands/waveform when fresh; keep the simulated math as fallback.
- [ ] Add a compact real spectrum to the Browse now-playing bar
      (`BrowseVisualizer.svelte`), using the same frames and fallback.
- [ ] Tests: analyzer, client set, freshness/fallback, visualizer rendering.
- Commit: `feat(audio): add a real monitor-driven visualizer`

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
