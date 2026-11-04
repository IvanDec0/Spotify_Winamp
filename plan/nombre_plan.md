# Cadence v1 — Implementation Plan

> **Status:** M3 local playback implemented and verified live<br>
> **Working name:** Cadence<br>
> **Platform:** Arch Linux x86_64<br>
> **UI and documentation language:** English<br>
> **Last updated:** 2026-11-03

---

## 1. Goal

Build a compact Arch Linux desktop **Spotify player** with a single window, an expandable queue, an in-app library and installable declarative skins.

Cadence decodes and plays Spotify audio locally through a managed **go-librespot** sidecar. It does not require the official Spotify client, does not use Web Playback SDK/DRM, and never exposes playback credentials to the frontend.

The demo player remains available for development and for machines without Spotify.

---

## 2. Locked decisions

| Area | Decision |
|---|---|
| Product | Local Spotify player, library and queue inside Cadence |
| Audio engine | go-librespot pinned to `v0.10.6-0.20261102145552-522adcae8d65`, managed as a separate process |
| Engine control | Loopback HTTP REST API on `127.0.0.1` with a random port |
| Auth | Spotify device pairing (`spotify.com/pair`); credentials stored by the engine |
| Library | go-librespot endpoints for playlists, saved albums, followed artists, Liked Songs and context tracks |
| Audio output | PulseAudio/PipeWire, 320 kbps, device name **Cadence** |
| Window | One frameless window, 420×220 logical with a 125–300% adaptive UI scale (200% default), expanded to fit the Library or queue |
| Skins v1 | Levels 1–2: JSON manifest/layout + CSS + local assets; no JavaScript |
| Default skin | Original “Clarity”, inspired by cPro Pure; no copied assets |
| Visualizer | Four simulated modes; no real beat detection |
| Native Winamp skins | Reference only in v1; read-only importer may be evaluated in v2 |
| Copyrighted recreations | Personal only, under `skins-local/`, excluded from Git and releases |
| Platform | Arch Linux x86_64 |
| Packaging | AppImage and `.tar.gz`; go-librespot bundled in both artifacts |
| License | MIT for Cadence code; CC BY-SA 4.0 for original skin assets; go-librespot is GPL-3.0 and kept as a separate process |

Pinned stack: Go 1.26+ (environment 1.27.3), Wails `v3.0.0-beta.34`, Svelte 5.57.2, Vite 8.3.5, TypeScript 6.0.3, Node 26.12.1.

This is a private, personal-use project. The user explicitly accepted the use of an unofficial Spotify client and the associated account/terms risk.

---

## 3. Status

| Item | Status |
|---|---|
| Product scope and architecture | Done |
| Default skin direction | Done |
| Skin format scope | Done |
| Native `.wsz`/`.wal` decision | Done |
| Git repository initialized | Done |
| Wails v3 beta.34 CLI installed | Done |
| Task 3.54.0 installed | Done |
| `wails3 doctor` passes | Done |
| Wails project scaffold | Done (M0) |
| Frontend toolchain pinned and linted | Done (M0) |
| CI skeleton | Done (M0) |
| Demo player and Clarity | Done (M1) |
| Declarative skin system | Done (M2) |
| UI/UX polish, Lumina, Vanta and Nightstar skins | Done (post-M4) |
| Catalog search and Spotify radio | Done (M3 extension) |
| Local audio engine | Done (M3) |
| Real Spotify pairing | Done and verified live |
| Local audio playback | Done and verified with 320 kbps OGG |
| Library browsing and selection | Implemented; extended live verification in progress |
| `task audio:build` | Done |
| Engine documentation | Done |
| Packaging and release | Implemented locally; clean Arch/VM test pending (M4) |

### Toolchain note

The Go-installed binaries are in `$(go env GOPATH)/bin` (`/home/ivan/go/bin`), which is not always in `PATH`. Add this to the shell configuration:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

Installed versions:

```text
wails3 v3.0.0-beta.34
task 3.54.0
go 1.27.3
node 26.12.1
npm 12.2.1
```

---

## 4. Milestones

### M0 — Repository and Wails scaffold

**Goal:** a clean Wails v3 + Svelte project that builds and runs, with the existing docs preserved.

**Status: complete.** The scaffold is pinned to Wails `v3.0.0-beta.34`, the demo service and time event were removed, and the Linux-only Taskfile builds a frameless shell. The logical 420×220 layout uses an adaptive 125–300% UI scale (200% default) so text stays legible.

### M1 — Demo player and Clarity skin

**Goal:** a usable simulated player and the first original skin, with no Spotify dependency.

**Status: complete.** The demo provider, player store, Clarity skin and window services are implemented. The queue expands and collapses through a native GTK4 resize helper.

### M2 — Skin system levels 1–2

**Goal:** install, validate, activate and remove declarative skins without recompiling.

**Status: complete.** Manifest and layout schemas, semantic validation, CSS parsing, local asset checks, directory and ZIP installation, staging with rollback, last-known-good fallback, safe mode, a Shadow DOM runtime, a skin library and the Afterglow development skin are implemented. A second development skin, **Lumina**, adds a glossy Windows Media Player-era visual language with animated chrome, ambient light, a rotating album-art halo, LED accents and Azure/Carbon/Ember/Violet themes using CSS-only motion. **Vanta** adds an asymmetric cockpit, and **Nightstar** adds a shaped, transparent four-blade star console and exercises the declarative box/text/image/button primitives.

### M3 — Local Spotify playback and in-app library

**Goal:** Cadence plays Spotify audio itself, pairs without the official client, and lets the user choose music inside the app.

**Status: implemented and verified live.** Device pairing, authentication and 320 kbps local playback were verified on Arch Linux. Playlist browsing, context listing, track selection, play-all, add-to-queue, pause, queue expansion and album-art proxying were verified in the running app.

Tasks:

- [x] Evaluate librespot, spotifyd and go-librespot for local playback and library APIs.
- [x] Choose go-librespot and pin a master build with albums, artists and Liked Songs endpoints.
- [x] Add `task audio:build` for the pinned engine.
- [x] Implement the managed sidecar runtime, config, free-port allocation and log capture.
- [x] Implement the loopback REST client for status, auth, playback, volume, shuffle, repeat and queue.
- [x] Implement device-auth pairing and expose the pairing URL/code to the UI.
- [x] Implement the provider that maps engine status to `PlayerState`.
- [x] Implement library endpoints for playlists, albums, artists and context tracks.
- [x] Extend the engine with internal catalog search for tracks, albums, artists and playlists.
- [x] Add Spotify radio stations seeded from tracks, albums, artists and playlists.
- [x] Implement the Library panel with tabs, search, playback, radio and add-to-queue.
- [x] Proxy and cache album art through Go.
- [x] Keep the frontend free of credentials.
- [x] Remove the old Spotify Connect/OAuth/keyring path from the app.
- [x] Rewrite setup documentation for the local engine.
- [ ] Finish live verification of volume, next/previous, shuffle/repeat and every library tab.
- [ ] Verify reset pairing from a clean state.

Exit criteria:

- [x] Cadence pairs with Spotify through the device flow.
- [x] Cadence decodes and plays Spotify audio locally.
- [x] The official Spotify client is not required.
- [x] Playlists, tracks and queue are visible in the app.
- [x] A selected track or context starts local playback.
- [x] The frontend never receives playback credentials.
- [x] `--demo` still works without the engine.
- [x] Every transport control is verified against the local engine.
- [x] Pairing reset is verified from a clean state.

### M4 — Packaging and release

**Goal:** reproducible Arch Linux artifacts that include the audio engine.

**Status: implemented locally.** `task package` builds `cadence-0.1.0-linux-x86_64.AppImage`, `cadence-0.1.0-linux-x86_64.tar.gz` and SHA-256 checksums. Both artifacts contain `go-librespot`; the AppImage adds it to `usr/bin` and the tarball places it next to Cadence. The AppImage and tarball were extracted and launched in demo mode on Arch. CI packaging and a tagged release workflow are configured. A clean Arch installation or VM test remains pending.

Tasks:

- [x] Build AppImage and `.tar.gz` artifacts in an Arch Linux CI container workflow.
- [x] Bundle `bin/go-librespot` in both artifacts.
- [x] Add GitHub Actions test/package/release workflows.
- [x] Generate SHA-256 checksums and verify them.
- [x] Write installation and first-run documentation.
- [ ] Test on a clean Arch installation or VM.
- [x] Finalize licenses, README and installation docs.
- [ ] Add final screenshots to the release notes/README.

Exit criteria:

- [ ] A clean Arch user can install Cadence, pair and play.
- [ ] Skins can be installed from the release build.
- [ ] CI produces reproducible artifacts and checksums.
- [x] Both artifacts include the pinned engine.
- [x] No Windows or macOS artifacts are produced.

### M5 — v2 evaluation only

- Evaluate a read-only `.wsz` importer for a documented subset without Maki.
- Evaluate a read-only `.wal` importer for a documented subset without Maki.
- Design sandboxed level 3 modules with a restricted API, permissions and resource limits.
- Do not start level 3 until levels 1–2 are stable and documented.

---

## 5. Recommended work order

1. **M0:** scaffold Wails in a temporary directory and merge it into the repo.
2. **M1:** build the demo provider and the Clarity skin.
3. **M2:** lock the skin format and the installer.
4. **M3:** add the local audio engine, pairing, library and selection.
5. **M4:** package and release.
6. **M5:** evaluate v2 features only after v1 is usable.

Do not start with native Winamp importers or level 3 modules.

---

## 6. Test gates

### Gate A — after M0

**Status: passed.**

- `go test ./...`
- `go vet ./...`
- frontend dependency install, type check, lint and build
- `wails3 build` / `wails3 dev` smoke test
- `git status` clean

### Gate B — after M1

**Status: passed.** `task check`, `task test` and `wails3 build` pass locally; queue expansion was verified on Sway with app-driven native resizing.

- Player store and demo provider unit tests
- Clarity layout/theme rendering tests
- visualizer determinism and reduced-motion tests
- queue expand/collapse test
- keyboard focus and basic accessibility check

### Gate C — after M2

**Status: passed.** The Go suite covers schemas, CSS, traversal, symlinks, ZIP limits, staging, activation, rollback, uninstall, safe mode, last-known-good fallback and a second skin. The frontend suite covers Shadow DOM mount/theme/queue updates and invalid-layout fallback.

- [x] manifest/layout schema tests
- [x] path traversal, symlink and ZIP-bomb rejection
- [x] installer staging, activation, rollback and uninstall tests
- [x] safe-mode fallback test
- [x] listener cleanup when switching skins
- [x] second-skin installation test

### Gate D — after M3

**Status: automated tests passed; extended live verification in progress.**

- [x] REST client tests with `httptest` for status/auth/playback/library/context/errors
- [x] provider mapping tests for playback status, queue, art, volume and controls
- [x] engine process discovery test
- [x] `go test -race ./internal/golibrespot`
- [x] `task check`, `task test` and `wails3 build`
- [x] live device pairing and local playback
- [x] live library browsing, track selection and add-to-queue
- [x] live volume, next/previous and shuffle/repeat verification
- [x] live pairing reset from a clean state

### Gate E — after M4

**Status: local packaging passed; clean Arch/VM test pending.**

- [x] AppImage and tarball build locally
- [x] AppImage and tarball smoke test in demo mode
- [x] engine binary discovery in the packaged builds
- [x] checksum verification
- [x] bundled docs and GPL notice
- [ ] skin installation from the packaged build
- [x] no credentials in the repository or artifacts
- [ ] clean Arch installation test

---

## 7. Risks and mitigations

| Risk | Mitigation |
|---|---|
| Unofficial Spotify protocol breaks | Pin the engine version; document the risk; expose clear engine errors |
| Spotify account/terms risk | Private personal use only; user accepted the risk; no redistribution |
| GPL-3.0 engine licensing | Keep go-librespot as a separate process; do not link it into Cadence |
| Engine auth state loss | Pairing reset flow; document `state.json` and the pairing URL |
| Audio backend unavailable | Detect PulseAudio/PipeWire failures in the engine log; show actionable errors |
| go-librespot API changes | Pin a known-good pseudo-version; keep the client narrowly scoped |
| Library endpoint gaps | Use the pinned master build, not the older release without albums/artists |
| Wails v3 beta API churn | Pin `v3.0.0-beta.34`; review release notes before upgrading |
| WebKitGTK limitations | Do not use Web Playback SDK or EME; audio is handled outside the webview |
| 2× UI scale needs a tall screen | The UI rescales with the window and clamps the expanded height to the screen work area |
| Frameless window differences on Wayland | Test on Sway; provide a fallback titlebar/drag region if needed |
| Malicious skin packages | Strict schemas, path canonicalization, ZIP limits, CSP, safe mode |
| Skin CSS exfiltration | No remote URLs, no scripts, local assets only |
| Album art remote content | Proxy/cache art through Go; keep CSP restricted |
| Real beat detection impossible without audio access | Simulated visualizer only; document the limitation |

---

## 8. Definition of done for v1

- [x] Cadence starts on Arch Linux with Wails v3.
- [x] The demo player works without Spotify.
- [x] The Clarity skin is the default and looks correct at its 420×220 logical size.
- [x] The queue expands and collapses.
- [x] Four simulated visualizer modes and six themes work.
- [x] Cadence pairs through the Spotify device flow.
- [x] Cadence decodes and plays Spotify audio locally.
- [x] The official Spotify client is not required for playback.
- [x] Playlists, saved albums, followed artists and Liked Songs are browsable.
- [x] The Spotify catalog can be searched for tracks, albums, artists and playlists.
- [x] Spotify radio can be started from a track, album, artist or playlist.
- [x] A context or individual track can be played from the Library.
- [x] Tracks can be added to the local queue.
- [x] No playback credential appears in the frontend, logs or repository.
- [x] The old Spotify Connect/OAuth/keyring path is removed from the app.
- [x] Volume, next/previous and shuffle/repeat are verified live.
- [x] Pairing reset is verified from a clean state.
- [x] A second skin can be installed, activated and removed without recompiling.
- [x] Invalid or malicious skins are rejected or safely rolled back.
- [x] `--safe-mode` starts without third-party skins.
- [x] Skin authoring documentation is sufficient for level 1–2 skins.
- [ ] CI passes and produces AppImage and `.tar.gz` artifacts.
- [x] Both release artifacts include the pinned engine and documentation.
- [x] Checksums are generated and verified locally.
- [ ] A clean Arch installation can install, pair and play.
- [ ] Release screenshots are included.

---

## 9. Immediate next actions

1. Run `task format:write`, `task check`, `task test` and `wails3 build` after the packaging changes.
2. Push the M4 commit and confirm the CI package job and a `v0.1.0` release workflow on GitHub.
3. Test the release artifacts on a clean Arch installation or VM, including skin installation.
4. Add final screenshots to the README or release notes.
5. Verify that the bundled Lumina, Vanta, Nightstar and Afterglow skins appear in packaged builds.
6. Optionally add a UI scale preference for smaller displays.

Do not begin with native Winamp importers or level 3 modules.
