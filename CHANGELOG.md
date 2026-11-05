# Changelog

All notable changes to Cadence are documented in this file.

## [0.2.0] - 2026-11-05

Cadence 0.2.0 completes the v2 architecture and Browse work. Spotify playback
requires a Spotify Premium account.

### Added

- Dedicated Browse window for search, library, podcasts, entity pages and
  settings. It swaps with the compact player so only one window is visible, and
  closing Browse restores the compact player without interrupting playback.
- Browse player parity with the compact window: shuffle, repeat, seek, mute and
  queue, plus a Browse queue section.
- Real monitor-driven visualizer. On Linux with PulseAudio or PipeWire it
  captures the system-wide default monitor through `parec`; when `parec` or a
  monitor is unavailable it falls back to the simulated animation. The
  visualizer works in the compact, declarative-skin and Browse views.
- Search tabs for tracks, albums, artists, playlists, shows and episodes,
  powered by the patched engine search groups.
- Podcast browsing and playback through the engine's episode support, plus
  library save, follow and unfollow writes for albums, artists and shows.
- Unified Browse settings for engine playback and cache options, appearance,
  language and skins.
- Runtime English/Spanish language switch with a persisted preference and a
  translation guard that rejects untranslated literals.
- Discover empty state built from the user's library, recent entities and
  genres.
- Typed engine errors and diagnostics counters for engine requests, events,
  artwork and the visualizer.

### Changed

- Split the Go code into `internal/engine`, `playback`, `catalog`, `artwork`,
  `settings`, `platform`, `wailsapi/*` and `visualizer`, with Wails confined to
  the adapters and the composition root.
- Player state and artwork now follow the engine event stream, replacing the
  fixed polling loop and redundant queue payloads.
- Pinned go-librespot to `v0.10.6-0.20261104085532-16b65bb4cd2b` with four
  local patches: search/radio, search groups (shows/episodes), library
  collection writes and HTTP proxy support.
- AppImage and tarball now bundle `CHANGELOG.md` and the `docs/` directory
  alongside the README, notices and licenses.
- `--version` prints the stamped release version and short commit, then exits
  without opening a window or starting services.

### Fixed

- The AppImage installs its icon in the hicolor theme and its desktop entry
  under `usr/share/applications`, so launchers and the desktop use the Cadence
  icon instead of the default.
- Lumina and Afterglow no longer repeat the device and settings controls that
  the titlebar already provides. Vanta's utilities row now fits the compact
  canvas, and its telemetry display uses the full visualizer layer instead of a
  short bottom strip.
- Browse and the compact player now show one window at a time instead of
  leaving a stale window behind.
- Saved shows use the show collection set rather than the episode set.
- Visualizer capture is released on `pagehide` instead of leaking the stream.
- Stream revisions, queue loading and skin progress no longer race during
  track changes.

### Removed

- The duplicated in-window library, settings popover and skin manager surfaces.
  Browse is now the single library and settings surface.
