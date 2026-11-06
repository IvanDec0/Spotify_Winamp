# Cadence v3 Roadmap

> **Status:** Draft for execution<br>
> **Baseline:** Cadence 0.2.0 (`feat/cadence-v2`, 2026-11-05)<br>
> **Platform:** Arch Linux x86_64 only<br>
> **Scope:** every gap and feature identified in the final 0.2.0 analysis<br>
> **Last updated:** 2026-11-05

---

## 1. Purpose

This roadmap turns the 0.2.0 gap analysis into ordered, reviewable work. It
covers player parity, desktop integration, the Winamp-skin identity, music
depth, release maturity and the engineering debt that protects all of it.

The order is deliberate: close the friction users feel every day before adding
new surface area, then build the differentiator (native Winamp skins), then add
music depth, then harden the release pipeline.

---

## 2. Baseline and constraints

### What 0.2.0 already has

- Local Spotify playback through a managed `go-librespot` sidecar.
- Compact skinnable player plus a dedicated Browse window.
- Search, library, podcasts, radio, queue display and settings.
- Declarative skins with validation, ZIP safety, shaped windows and fallback.
- English/Spanish i18n with a translation guard.
- Real monitor-driven visualizer with four modes.
- CI test, package and release workflows.
- Reproducible AppImage and tarball packaging.

### Constraints

- **Arch Linux x86_64 only.** No Windows or macOS work.
- **The engine is pinned.** New engine behaviour means a numbered patch in
  `build/patches/` plus `task audio:check` coverage. Patches are a maintenance
  cost and need an upstream-PR attempt where practical.
- **No JavaScript in skins.** The skin format stays declarative.
- **The frontend never receives credentials.**
- **No telemetry.** Any network feature must be opt-in or clearly documented.
- **Review budget:** one work unit per PR, authored additions plus deletions
  at or below 400 lines. Larger work is sliced, never compressed.
- **One visible window at a time** between the compact player and Browse.
- **The demo player must keep working** without Spotify or network access.

### Effort legend

| Size | Rough effort (solo dev + AI) |
|---|---|
| S | up to 2 working days |
| M | 3–5 working days |
| L | 1–2 working weeks |
| XL | more than 2 working weeks |

Effort is relative, not a delivery promise.

---

## 3. Release map

| Release | Theme | Main outcome | Depends on |
|---|---|---|---|
| **0.2.1** | Hardening | Quality gates, i18n errors, a11y, logging, artwork allowlist, release smoke enablers | 0.2.0 |
| **0.3.0** | Player completeness | Full/jump queue, output selector, saved shows, desktop integration, cache controls | 0.2.1 |
| **0.4.0** | Winamp native | `.wsz` import, skin gallery, viscolor mapping | 0.3.0 (stable base) |
| **0.5.0** | Music depth | Lyrics, scrobbling, equalizer, playlist CRUD, search/discover | 0.4.0 |
| **0.6.0** | Release maturity | Updater, clean Arch automation, diagnostics export, signed/reproducible releases | 0.5.0 |

### 3.1 Coverage of the 0.2.0 analysis

Every gap and feature from the final analysis maps to at least one work item:

| 0.2.0 analysis item | Roadmap |
|---|---|
| Queue read-only and capped at 50 | P1 (full/jump), P2 (edit, spike-gated) |
| Saved shows not listed | P4 |
| Episode resume | P4 (already engine-side; verify and document) |
| Audio output fixed to `pulseaudio`/`default` | P3 |
| Single instance missing | P5a |
| Tray missing | P5b |
| Notifications missing | P5c |
| Autostart missing | P5d |
| Global media keys | P5e (MPRIS default in P5f) |
| MPRIS off by default | P5f |
| Cache controls missing | P6 |
| Raw English backend errors | H5 |
| Updater missing | R1 |
| Clean Arch/VM test pending | R2 |
| Synced lyrics | M1 |
| Last.fm scrobbling | M2 |
| Skin gallery/registry | W2 |
| 10-band equalizer | M3 (spike-gated) |
| `.wsz` read-only importer | W0, W1 |
| Playlist CRUD | M4 (spike-gated) |
| Search pagination | M5 |
| Discover refresh | M5 |
| Visualizer modes | Already present; W3 adds `viscolor` mapping |
| CI race, coverage, E2E, SBOM | H1, R4, R5 |
| Structured logging, rotation, diagnostics | H3, R3 |
| Artwork proxy allowlist | H2 |
| A11y live region and axe | H4 |
| i18n backend errors, time formats, TS guard | H5 |
| Signed/reproducible releases | R4 |

Two items are **spike-gated** and are not release exit criteria until the
engine feasibility is proven:

- Full queue remove/reorder (P2).
- Equalizer (M3) and playlist CRUD (M4).

If a spike fails, the item is documented as deferred instead of being forced
into a release.

---

## 4. Phase 0.2.1 — Hardening

**Goal:** make the next releases safe to build. This phase is mostly invisible
to users and closes the engineering debt from the 0.2.0 analysis.

**Exit criteria**

- CI runs `go test -race`, coverage ratchets, `govulncheck` and the frontend
  checks.
- No known backend error reaches the UI as raw English text.
- Screen readers announce track changes.
- Logs are structured, rotated and token-free.
- The artwork proxy only fetches allowlisted hosts.
- A packaged AppImage can be smoke-tested headlessly in CI.
- `task verify` and `task package` stay green.

### H1. CI quality gates

- **Goal:** catch regressions before they reach a release.
- **Scope:** add a `-race` job, coverage collection with a ratchet, Go
  vulnerability scanning, frontend audit for high/critical issues, and a
  machine-readable dependency inventory.
- **Files:** `.github/workflows/ci.yml`, `Taskfile.yml`, `frontend/package.json`.
- **Acceptance:** CI fails on a new high/critical vulnerability, a coverage
  drop below the recorded baseline, or a race in the Go suite.
- **Tests:** the gates themselves; run the full suite locally first to record
  the baseline.
- **Effort:** S. **Depends on:** none.
- **PR slices:** CI workflow and Taskfile in one PR; fix any failures the new
  gates expose in follow-up work units.

### H2. Artwork host allowlist

- **Goal:** close the open-proxy defence gap in the album-art cache.
- **Scope:** only HTTPS; allowlisted Spotify image hosts; revalidate every
  redirect; reject loopback, link-local and private addresses; keep the
  existing cache behaviour.
- **Files:** `internal/artwork/cache.go`, `internal/artwork/cache_test.go`.
- **Acceptance:** existing album art still loads; a request for
  `http://127.0.0.1/...` or an unknown host is rejected; redirects to a
  disallowed host are rejected.
- **Tests:** table-driven URL tests, redirect tests, cache regression tests.
- **Effort:** S. **Depends on:** none.
- **PR slices:** one PR.

### H3. Structured logging and rotation

- **Goal:** make support and diagnosis possible without reading raw stdout.
- **Scope:** introduce `internal/logging` with `log/slog`, a level from
  `CADENCE_LOG_LEVEL`, a file sink under `$XDG_CACHE_HOME/cadence/logs/`, and
  size-based rotation. Rotate the engine log per start and keep the last N
  files. Add the log path to the diagnostics payload.
- **Files:** `internal/logging/*`, `main.go`, `internal/engine/supervisor.go`,
  `internal/diagnostics/*`, `docs/development.md`.
- **Acceptance:** logs are structured, rotated, and never contain Spotify
  tokens or pairing secrets; a long session does not grow one file without
  bound.
- **Tests:** rotation writer tests, redaction test, level parsing.
- **Effort:** M. **Depends on:** none.
- **PR slices:** logging package + wiring; engine log rotation; docs.

### H4. Accessibility announcements and automated checks

- **Goal:** make playback state usable with a screen reader and catch obvious
  regressions.
- **Scope:** add a visually hidden `aria-live="polite"` region in the compact
  player and Browse Now Playing bar announcing `Now playing: title — artist`.
  Add `vitest-axe` smoke tests for the compact player, Browse search and
  Settings. Fix critical violations found.
- **Files:** `frontend/src/App.svelte`,
  `frontend/src/app/browse/NowPlayingBar.svelte`,
  `frontend/src/**/*.test.ts`, `frontend/package.json`.
- **Acceptance:** a track change is announced once; no duplicate or noisy
  announcements; axe smoke tests pass.
- **Tests:** fake-timer announcement test, axe tests.
- **Effort:** S–M. **Depends on:** none.
- **PR slices:** live region; axe setup and fixes.

### H5. Typed error envelope and i18n completion

- **Goal:** stop showing raw English backend messages and finish the i18n
  contract.
- **Scope:** add a typed Wails error envelope `{ code, message, details }`;
  map known codes to en/es keys in the frontend; keep an unknown-error fallback.
  Extend `check-i18n.mjs` to scan TypeScript UI modules and additional
  attributes. Make duration formatting locale-aware and support `h:mm:ss`.
- **Files:** `internal/wailsapi/*`, `frontend/src/shared/api/*`,
  `frontend/src/shared/i18n/*`, `frontend/scripts/check-i18n.mjs`,
  `frontend/src/lib/format.ts`.
- **Acceptance:** every known failure shows a translated message; en/es
  catalogs stay in parity; the guard exits 0; no raw backend string is rendered
  for a known error.
- **Tests:** error-mapping unit tests, catalog parity, formatter tests.
- **Effort:** M. **Depends on:** none.
- **PR slices:** Go error envelope; frontend mapping and catalogs; guard and
  formatter.

### H6. Release smoke enablers

- **Goal:** make packaged builds testable without a human clicking dialogs.
- **Scope:** add `--install-skin <path>`, `--list-skins` and `--smoke-test`
  (starts the app, waits for the first window, exits 0). The smoke path runs
  under `xvfb-run` in CI with `--demo`.
- **Files:** `main.go`, `internal/skins/*`, `docs/development.md`,
  `.github/workflows/ci.yml`.
- **Acceptance:** `cadence --install-skin skins-dev/lumina` works headlessly;
  `cadence --smoke-test --demo` exits 0; `--version` unchanged.
- **Tests:** CLI parsing tests, smoke script in CI.
- **Effort:** S. **Depends on:** H1.
- **PR slices:** one PR.

---

## 5. Phase 0.3.0 — Player completeness

**Goal:** remove the daily friction that makes Cadence feel unfinished. This is
the release that should make the official client unnecessary for normal use.

**Exit criteria**

- The queue is not capped at 50 items and an item can be played directly.
- A second launch focuses the existing instance instead of starting a second
  engine.
- Audio output can be switched without restarting playback.
- Saved shows are listed and open correctly.
- The tray, notifications, autostart and optional global hotkeys work on
  Arch/Sway with a documented fallback.
- The cache can be measured and cleared safely.
- Existing episode resume behaviour is verified and documented.

### P1. Full queue and jump-to-item

- **Goal:** make the queue usable for long contexts.
- **Scope:** remove the 50-item cap; add paged queue reads; add a
  `PlayQueueIndex(index)` service call that maps the index to the context URI
  and calls `PlayContext(contextURI, itemURI)`; virtualize or page the queue UI.
- **Technical notes:** `internal/playback/engine_provider.go` already caches the
  current context tracks. Reuse that cache instead of refetching. The engine's
  `POST /player/play` accepts `skip_to_uri`, so jumping needs no engine patch.
  Do not send a 5,000-item payload to the webview; page with `offset`/`limit`
  and a `total`.
- **Acceptance:** a 1,000+ track playlist can be scrolled, item 900 can be
  played directly, and the current/next markers stay correct.
- **Tests:** provider bounds and cache tests, service jump tests, UI keyboard
  and paging tests.
- **Effort:** M. **Depends on:** H5 (error envelope).
- **PR slices:** provider queue page + service; UI paging and play-now; docs.

### P2. Queue remove and reorder (spike-gated)

- **Goal:** true queue editing, not just jumping.
- **Scope:** a design spike over `go-librespot`'s context resolution
  (`daemon/controls.go`, `tracks.List`) to answer: can the resolved queue be
  mutated without desyncing Spotify Connect state? If yes, add engine endpoints
  for remove and move, then Cadence RPCs and UI controls. If no, document the
  limitation and defer.
- **Acceptance (if shipped):** removing an item and moving an item update the
  queue and survive the next skip; rollback is a single engine patch.
- **Tests:** engine handler tests, provider mutation tests, UI drag/keyboard
  tests.
- **Effort:** XL. **Depends on:** P1.
- **PR slices:** spike + ADR; engine patch; Cadence RPC; UI. Do not start the
  UI until the spike proves the engine path.

### P3. Audio output selector

- **Goal:** let users choose the sink instead of being locked to the default.
- **Scope:** add `internal/audiooutput` that enumerates PulseAudio/PipeWire
  sinks (`pactl list short sinks`, with `wpctl`/`aplay -L` fallbacks), validates
  the selection, and caches the list. Add a Settings > Playback output section:
  backend (pulseaudio, alsa) and device. Device changes call
  `POST /player/output` live; backend changes require the existing restart
  confirmation. The engine's `pipe` backend stays out of the UI until Cadence
  can configure and validate an output pipe.
- **Acceptance:** switching between two sinks during playback works without a
  restart; the choice persists; a missing `pactl` shows an actionable message
  instead of breaking playback.
- **Tests:** parser fixtures for `pactl`/`wpctl`/`aplay` output, service tests
  with a fake enumerator, manual test on PipeWire and PulseAudio.
- **Effort:** M. **Depends on:** H5.
- **PR slices:** enumerator; service + settings; UI.

### P4. Saved shows and episode resume verification

- **Goal:** finish Podcasts in Browse.
- **Scope:** add a fifth local engine patch
  (`build/patches/go-librespot-library-shows.patch`) with a paged
  `GET /library/shows` that reuses the collection paging and resolves show
  metadata. Add it to `scripts/build-go-librespot.sh` and `task audio:check`.
  Add `catalog.SavedShows`, the audio adapter and a "Saved shows" tab in
  `PodcastsView`. Episode resume already exists in the engine
  (`daemon/resume.go`); verify it end to end in the UI and document it instead
  of reimplementing it.
- **Acceptance:** saved shows are listed with artwork and names; opening a show
  lists episodes; playing an episode resumes at the server-side position;
  removing the patch is documented.
- **Tests:** patch apply/check, catalog paging tests, service tests, UI empty
  and error states, manual resume test.
- **Effort:** M. **Depends on:** H6 (smoke); uses the existing engine patch workflow.
- **PR slices:** engine patch + `task audio:check`; catalog/service; UI.
- **Note:** this is the only 0.3 item that touches the pinned engine. Keep the
  patch small and upstream it if possible.

### P5. Desktop integration

- **Goal:** make Cadence a desktop application, not just a window.
- **Scope:**
  - **P5a Single instance:** `application.SingleInstance` with `UniqueID:
    appID`; on a second launch, focus the visible window and exit.
  - **P5b Tray:** `app.SystemTray.New()` with Show player, Browse, Play/Pause,
    Next, Previous and Quit; update the play/pause label from player events;
    continue without a tray if the host does not support one.
  - **P5c Notifications:** D-Bus `org.freedesktop.Notifications` via the
    existing `godbus` dependency; notify on track change only when the player
    window is hidden; setting to disable.
  - **P5d Autostart:** write/remove an XDG autostart desktop entry; add a
    `--hidden` start mode that stays in the tray; setting to disable. If the
    tray host is unavailable, `--hidden` starts visible instead of leaving the
    app unreachable.
  - **P5e Global hotkeys:** `GlobalShortcutManager` for play/pause, next and
    previous; opt-in because media keys are already served by MPRIS on many
    desktops; report conflicts instead of failing silently.
  - **P5f MPRIS default:** new installs default MPRIS on; existing users keep
    their stored value and see a one-time Settings hint. Do not silently flip a
    user's choice.
- **Acceptance:** second launch focuses the first; tray menu controls playback;
  notification appears; autostart survives a logout; hotkeys work when enabled;
  MPRIS default is documented.
- **Tests:** single-instance manual test, tray menu tests where possible,
  notification payload test, autostart file test, shortcut conflict test.
- **Effort:** L. **Depends on:** H3 (logging), H5.
- **PR slices:** single instance; tray; notifications + autostart; hotkeys +
  MPRIS default/docs.

### P6. Cache controls

- **Goal:** make the cache section actionable.
- **Scope:** add `CacheStats` (size, file count, path) and actions: clear cache
  (confirm; stop/restart the engine so the current stream is not corrupted),
  open folder, change folder (restart required). Show the engine's current
  limit and whether it is being enforced.
- **Acceptance:** the displayed size matches the directory; clearing frees
  space; changing the folder starts clean; playback is not broken by an
  in-flight clear.
- **Tests:** temp-dir stats tests, clear safety test, settings persistence.
- **Effort:** S–M. **Depends on:** H5.
- **PR slices:** service + tests; UI.

---

## 6. Phase 0.4.0 — Winamp native

**Goal:** build the differentiator. No mainstream Spotify client imports
classic Winamp skins.

**Exit criteria**

- A user-supplied classic `.wsz` can be imported, previewed, installed,
  activated and removed.
- The result is a normal declarative skin: no JavaScript, no remote assets, no
  Maki.
- A curated skin gallery can install and update declarative skins.
- Visualizer colours can follow a skin's `viscolor.txt`.
- The importer never bundles or redistributes third-party skins.

### W0. Skin window size decision (spike + ADR)

- **Question:** classic Winamp main windows are 275×116. Cadence skins are
  fixed at 420 logical width with a 220–460 height. Choose one:
  - **Option A:** allow `window.width` from 275 to 420 in a schema v2. Most
    faithful, but touches window sizing, zoom, all bundled skins and docs.
  - **Option B:** scale the classic layout to 420 width and use a 220 height
    with a documented letterbox/composition area. Cheaper, less faithful.
- **Deliverable:** an ADR under `docs/adr/` with the chosen option, the scaling
  rules and the compatibility impact.
- **Effort:** M. **Depends on:** none.
- **PR slices:** one ADR PR; implementation follows in W1.

### W1. `.wsz` read-only importer

- **Goal:** import classic Winamp 2.x main-window skins into the declarative
  skin format.
- **Scope:**
  - Accept `.wsz`/`.zip`; reuse the hardened ZIP validation (path traversal,
    type, size, compression limits).
  - Decode BMP with `golang.org/x/image/bmp` (already a dependency) and convert
    every used bitmap to PNG, because the skin validator does not accept BMP.
  - Supported inputs: `main.bmp`, `cbuttons.bmp`, `titlebar.bmp`, `numbers.bmp`,
    `text.bmp`, `playpaus.bmp`, `posbar.bmp`, `volume.bmp`, `balance.bmp`,
    `shufrep.bmp`, optional `monoster.bmp`, `viscolor.txt`, `region.txt`,
    `pledit.txt`.
  - Generate a complete skin package: `skin.json`, `layout.json`, `style.css`
    and PNG assets. Map the classic main window, transport, progress, volume,
    track text, visualizer and queue colours. Do not implement the classic EQ
    or playlist windows; map the queue to Cadence's queue panel.
  - Add `--import-wsz <path>` for tests and a Settings > Skins import action.
  - Reject Maki, cursors, AVS and any executable/unknown content.
- **Acceptance:** a real, freely licensed classic skin renders recognizably;
  malformed archives are rejected with a translated error; safe mode still
  falls back to Clarity; no third-party skin is committed.
- **Tests:** synthetic `.wsz` golden fixture, BMP decode limits, coordinate
  mapping, viscolor parsing, region parsing, invalid archive cases, install and
  rollback.
- **Effort:** XL. **Depends on:** W0.
- **PR slices:** parser and BMP conversion; skin generator and golden tests;
  CLI/UI import; region/shaped-window support (conditional); docs.

### W2. Skin gallery and updates

- **Goal:** make skins discoverable and updatable without manual ZIP hunting.
- **Scope:** a signed manifest (`id`, `name`, `version`, `author`, `preview`,
  `download`, `sha256`, `minAppVersion`, `license`); an embedded public key;
  fetch over HTTPS, verify the signature and checksum, install through the
  existing manager, and compare versions for updates. Start with a curated
  manifest hosted from a separate repository; submissions are validated in CI.
  No auto-install without a user action.
- **Acceptance:** a gallery skin installs; a tampered manifest or package is
  rejected; an installed skin reports an available update; the gallery works
  with the network offline by showing a clear error.
- **Tests:** signature verification, checksum mismatch, version comparison,
  manifest parsing, install rollback.
- **Effort:** L. **Depends on:** W1 (content), H2/H3 (security/logging).
- **PR slices:** manifest schema and verifier; client fetch/install; UI and
  update check; submission CI and docs.

### W3. Visualizer colour mapping

- **Goal:** let `.wsz` skins drive the visualizer palette.
- **Scope:** map the 24 `viscolor.txt` colours to CSS variables consumed by the
  existing spectrum/radial/oscilloscope/grid modes; expose the active mode and
  palette through the skin runtime; document the mapping. Do not add new
  analyser modes unless a concrete need appears.
- **Acceptance:** an imported skin changes the visualizer colours; existing
  skins are unchanged; the palette is validated and bounded.
- **Tests:** colour parser and CSS variable snapshot tests.
- **Effort:** S–M. **Depends on:** W1.
- **PR slices:** one PR.

---

## 7. Phase 0.5.0 — Music depth

**Goal:** add the features that make a music player feel complete beyond
playback.

**Exit criteria**

- Lyrics can be fetched, cached and synced, and can be disabled.
- Scrobbling works with an offline queue and clear privacy documentation.
- Search can show more than one page and Discover can be refreshed.
- The equalizer and playlist CRUD land only if their spikes prove feasible;
  otherwise they are documented as deferred.

### M1. Synced lyrics

- **Goal:** show time-synced lyrics without touching Spotify playback.
- **Scope:** `internal/lyrics` client for LRCLIB (`/api/get`, `/api/search`),
  LRC parser, on-disk cache keyed by track URI and duration, TTL, backoff.
  Frontend: a Lyrics panel in Browse and a compact toggle; the current line is
  derived from the playback position, with no per-tick backend call. Setting:
  "Fetch lyrics online" (opt-in on first use, with a privacy note).
- **Acceptance:** lyrics appear within a reasonable time; the active line
  tracks playback; missing lyrics and offline mode degrade gracefully; no
  lyrics request is made when disabled.
- **Tests:** LRC parser edge cases, HTTP client with `httptest`, cache and TTL,
  disabled setting, error paths.
- **Effort:** M. **Depends on:** H5.
- **PR slices:** parser/client/cache; service; UI and setting; docs.

### M2. Last.fm scrobbling

- **Goal:** scrobble played tracks with a reliable offline queue.
- **Scope:** `internal/scrobbling` with Last.fm auth (`auth.getToken`,
  `auth.getSession`), `track.updateNowPlaying`, `track.scrobble`, MD5
  signatures, session key stored with `0600` permissions, a bounded offline
  queue, scrobble thresholds (50% or 4 minutes, minimum 30 seconds), and a
  podcast exclusion. API key handling: prefer a user-provided key/secret stored
  locally with `0600`; a build-time injected key is allowed only with a
  documented caveat that it is extractable from the binary. Never commit a key
  or secret.
- **Acceptance:** connect/disconnect works; a played track is scrobbled once;
  a network failure queues and retries without duplicates; invalid sessions
  show an actionable error.
- **Tests:** signature fixtures, threshold logic, queue dedupe/retry, auth
  state, disabled mode.
- **Effort:** M. **Depends on:** H3 (logging), H5.
- **PR slices:** client + auth; scrobble engine and queue; settings/UI; docs.

### M3. 10-band equalizer (spike-gated)

- **Goal:** a Winamp-class EQ.
- **Spike options:**
  - **A. Engine DSP patch:** wrap the audio source before output and apply
    biquad filters. Most robust, highest engine maintenance.
  - **B. PulseAudio LADSPA sink:** load an equalizer sink and route the engine
    to it. No engine patch, but depends on an installed plugin and is fragile.
  - **C. PipeWire filter-chain:** write a filter-chain config and route the
    engine through a virtual sink. Modern, but requires a PipeWire restart and
    user-environment assumptions.
- **Decision rule:** prefer A if the audio pipeline supports a clean wrapper;
  otherwise B with a documented dependency; if neither is reliable, defer.
- **Acceptance (if shipped):** 10 bands (32 Hz–16 kHz), presets, enable/reset,
  live changes, persistence, no clipping or audible artefacts.
- **Tests:** biquad coefficient math, preset snapshots, API, manual audio
  checks.
- **Effort:** XL. **Depends on:** P3 (output routing).
- **PR slices:** spike/ADR; DSP or sink integration; UI; docs.

### M4. Playlist CRUD (spike-gated)

- **Goal:** create, rename, delete and reorder playlists inside Cadence.
- **Reality check:** the pinned engine exposes playlist append, remove and
  contains only. `spclient` has revision/contents/append/remove but no create,
  rename, delete or arbitrary move. This requires new engine protocol work and
  is the highest-risk item in the roadmap.
- **Scope:** spike first; if feasible, add engine endpoints and Cadence UI for
  create, rename, delete, add/remove and reorder (with keyboard alternatives to
  drag and drop). If not feasible, document the limitation and keep
  add/remove.
- **Acceptance (if shipped):** changes are visible in the official client;
  failures are atomic; no partial playlist state.
- **Tests:** engine handler tests against a fake spclient, UI tests.
- **Effort:** XL. **Depends on:** P4 (engine patch workflow).
- **PR slices:** spike/ADR; engine protocol; RPC; UI; docs.

### M5. Search pagination and Discover refresh

- **Goal:** remove the fixed 30-result ceiling and the once-only Discover.
- **Scope:** extend the engine search request with an offset if the Spotify
  search call supports it; otherwise raise the cap to a bounded maximum and
  paginate client-side. Add a Discover refresh action with a TTL and invalidate
  it when the library changes (save/follow/unfollow).
- **Acceptance:** "Load more" works per search tab; Discover refreshes without
  a restart; no duplicate results.
- **Tests:** search paging, dedupe, Discover invalidation.
- **Effort:** S–M. **Depends on:** P4 if the engine patch is needed.
- **PR slices:** engine patch (conditional); frontend paging/refresh.

---

## 8. Phase 0.6.0 — Release maturity

**Goal:** make releases supportable and self-updating.

**Exit criteria**

- The app can check for and install a new AppImage release.
- A clean Arch container can install and smoke-test the packaged app.
- Diagnostics can be exported and shared safely.
- Release artifacts are reproducible and signed.
- A real engine binary can be started in a contract test without credentials.

### R1. Updater

- **Goal:** stop asking users to hunt for releases.
- **Scope:** `internal/update` checks the GitHub Releases API, compares semver,
  selects the AppImage or tarball asset, downloads to a temporary path, verifies
  `SHA256SUMS`, and atomically replaces the AppImage. The tarball path shows
  instructions instead of replacing an install directory. UI in Settings >
  About with version, commit, check, download progress and release notes.
- **Acceptance:** an update is detected, downloaded, verified and applied on
  restart; a corrupted download is rejected; no telemetry is sent.
- **Tests:** version comparison, asset selection, checksum failure, HTTP
  errors, atomic replace with temp files.
- **Effort:** M. **Depends on:** H6, R4 (signed artifacts).
- **PR slices:** client; replace/restart logic; UI; docs.

### R2. Clean Arch smoke automation

- **Goal:** prove the packaged app works outside the developer machine.
- **Scope:** an Arch container job with GTK/WebKit dependencies and `xvfb-run`
  that runs `--version`, `--smoke-test --demo`, `--install-skin` and
  `--list-skins` on the built AppImage. Pairing and playback remain a manual
  checklist because they need a Spotify account.
- **Acceptance:** the job passes on a clean container and fails on a broken
  package; the manual pairing checklist is documented.
- **Tests:** CI job; docs.
- **Effort:** M. **Depends on:** H6.
- **PR slices:** CI job; docs/checklist.

### R3. Diagnostics export

- **Goal:** make support requests actionable.
- **Scope:** `--diagnose` prints JSON with version, commit, OS, engine discovery,
  audio backend availability (`pactl`, `parec`), MPRIS state, XDG paths, config
  schema and a scrubbed log tail. Add "Copy diagnostics" in Settings.
- **Acceptance:** no tokens, pairing secrets or full user paths beyond the
  documented XDG roots; the output is valid JSON.
- **Tests:** redaction test, JSON schema test.
- **Effort:** S–M. **Depends on:** H3.
- **PR slices:** command; UI; docs.

### R4. Reproducible and signed releases

- **Goal:** make artifacts verifiable.
- **Scope:** `-trimpath`, pinned toolchain, `SOURCE_DATE_EPOCH`, documented
  build environment, SBOM artifact, minisign/cosign signatures, published
  public key, and checksum verification instructions.
- **Acceptance:** two builds of the same commit produce equivalent artifacts
  (or a documented exception); signatures verify; the SBOM lists the engine
  and runtime dependencies.
- **Tests:** release workflow; verification script.
- **Effort:** M. **Depends on:** H1.
- **PR slices:** build flags/SBOM; signing; docs.

### R5. Engine contract test

- **Goal:** catch engine API drift before a release.
- **Scope:** start the real `go-librespot` binary in a temporary XDG directory
  without credentials, assert `/status` reports unpaired, exercise the pairing
  code path where the network allows it, and shut down cleanly. Tag it
  `integration` so the normal suite stays fast.
- **Acceptance:** the test passes with the pinned engine and fails clearly if
  the engine cannot start or the status contract changes.
- **Tests:** the contract test itself; optional nightly job.
- **Effort:** M. **Depends on:** H1.
- **PR slices:** harness; CI/manual job.

---

## 9. Dependency graph

```text
0.2.1 Hardening
  H1 CI gates ──────────────┬──> H6 smoke enablers ──> R2 clean Arch
  H2 artwork allowlist      │
  H3 logging ───────────────┼──> R3 diagnostics
  H4 a11y                   │
  H5 error envelope ────────┼──> 0.3.0 (P1, P3, P4, P5, P6)
  H6 smoke enablers ────────┘

0.3.0 Player completeness
  P1 full queue/jump ──> P2 queue edit (spike)
  P3 output selector ──> M3 EQ (spike)
  P4 saved shows (engine patch) ──> M4 playlist CRUD (spike), M5 search offset
  P5 desktop integration
  P6 cache controls

0.4.0 Winamp native
  W0 size decision ──> W1 .wsz importer ──> W3 viscolor
  W1 ──> W2 gallery

0.5.0 Music depth
  M1 lyrics
  M2 scrobbling
  M3 EQ (spike-gated)
  M4 playlist CRUD (spike-gated)
  M5 search/discover

0.6.0 Release maturity
  R1 updater ──> needs R4 signing
  R2 clean Arch
  R3 diagnostics
  R4 signed/reproducible
  R5 engine contract
```

---

## 10. Delivery and review rules

- **One work unit per commit.** Tests and docs travel with the behaviour they
  verify. Use Conventional Commits and no AI attribution.
- **One work unit per PR.** If a slice exceeds 400 authored changed lines,
  split it honestly. Never delete tests, docs, comments or blank lines to fit.
- **Default delivery strategy:** `ask-on-risk`. When a slice forecasts more
  than 400 lines, stop once and choose a chain strategy; record it in the
  feature tracker. Do not switch strategies mid-chain.
- **Feature trackers:** when implementation starts, create
  `odd/tasks/<release>.md` with task IDs, acceptance evidence and commit
  identities. The roadmap is the plan; the tracker is the execution record.
- **Engine patches:** every patch gets a descriptive filename, a focused test,
  an entry in `scripts/build-go-librespot.sh`, a line in `task audio:check` and
  a note in the engine documentation. Keep patches minimal and upstream them
  when possible.
- **No SDD artifacts** are created by this roadmap unless the user explicitly
  selects SDD. This is organic execution.
- **Native review:** when RDD is enabled, each work-unit commit is the review
  candidate against the previous reviewed boundary. Do not review the
  accumulated feature branch.

---

## 11. Verification matrix

| Layer | Per work unit | Per release |
|---|---|---|
| Go | `go test ./internal/...` focused | `go test -race ./...`, `go vet ./...` |
| Frontend | `npm run check`, focused Vitest | `npm run lint`, `format`, `i18n:check`, full Vitest, `build` |
| Engine | patch apply/revert, `task audio:check` | engine build + contract test |
| Packaging | — | `task package`, AppImage/tarball smoke, checksums |
| Manual | the item's scenario | Sway/Wayland, PulseAudio and PipeWire, tray host, notification daemon, two sinks |
| A11y | axe on the touched view | compact, Browse search, Settings |
| i18n | en/es keys for the change | guard 0, no raw backend errors |
| Docs | update the touched doc | README, CHANGELOG, installation, skin format |

---

## 12. Risks and mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Engine patches accumulate | Upgrades become expensive | Small, descriptively named patches, tests, upstream PRs, documented inventory |
| Spotify protocol changes | Playback or library breaks | Pinned engine, contract test, clear engine errors, update path |
| Queue remove/reorder is infeasible | P2 slips | Spike first; ship P1 independently; document the limitation |
| EQ needs an engine DSP patch | M3 slips | Spike A/B/C; prefer the least fragile option; defer if none is solid |
| Playlist CRUD needs new Spotify protocol work | M4 slips | Treat as spike-gated; keep add/remove as the committed baseline |
| `.wsz` importer is larger than expected | 0.4 slips | Strict subset (main window only), W0 decision first, golden fixture early |
| Tray/hotkeys differ on Wayland | Desktop integration feels broken | Document requirements, fall back gracefully, test on Sway and a tray host |
| Gallery hosting/moderation | Scope creep | Start with a curated static manifest in a separate repo; no server |
| Lyrics/scrobbling privacy | User trust | Opt-in, documented data flow, local cache, disable switch |
| 400-line budget forces many PRs | Review fatigue | Slice by work unit, one cohesive split, `size:exception` only with approval |
| Third-party skin licensing | Legal exposure | User-supplied imports only; never bundle copyrighted skins; document |

---

## 13. Out of scope / backlog

These are intentionally **not** committed to a release:

- Offline downloads and true offline playback (the current cache is a streaming
  cache).
- Remote Spotify Connect device transfer (the engine exposes one local device).
- Maki, AVS, executable/level-3 skins, cursors, or the classic EQ/playlist
  windows.
- Windows or macOS builds.
- Multi-account support.
- Discord Rich Presence.
- ListenBrainz (revisit after Last.fm).
- Native beat detection (requires isolated audio access the engine does not
  expose).
- Video podcasts.
- A hosted skin marketplace with accounts or payments.

---

## 14. Recommended first slice

If implementation starts tomorrow, use this order:

1. H1 CI gates and record the coverage baseline.
2. H5 typed error envelope and i18n completion.
3. H3 structured logging and rotation.
4. H2 artwork host allowlist.
5. H4 accessibility announcements and axe smoke tests.
6. H6 release smoke enablers.
7. P5a single instance — the smallest visible desktop win.
8. P1 full queue and jump-to-item.
9. P3 audio output selector.
10. P4 saved shows.
11. P5b–P5f tray, notifications, autostart, hotkeys and MPRIS default.
12. P6 cache controls.

That closes 0.2.1 and 0.3.0. Start the P2 and W0 spikes in parallel with the
last items so 0.4.0 is not blocked by a surprise.

---

## 15. Definition of done for 1.0

- 0.2.1, 0.3.0, 0.4.0 and 0.6.0 exit criteria are met.
- 0.5.0 ships lyrics, scrobbling and search/discover; EQ and playlist CRUD are
  either shipped or explicitly documented as deferred after their spikes.
- `task verify`, `go test -race`, frontend checks and packaging are green.
- A clean Arch container installs and smoke-tests the packaged app.
- The updater works against a signed release.
- Every packaged artifact is checksummed and signed.
- The documentation matches the shipped UI and engine patches.
- No credentials or telemetry leak; the frontend never receives tokens.
- The demo player still works without Spotify or network access.
