# Cadence v1 — Implementation Plan

> **Status:** M2 complete; M3 next<br>
> **Design source of truth:** [`../documentacion-cliente-musical-skins.md`](../documentacion-cliente-musical-skins.md)  
> **Working name:** Cadence  
> **Platform:** Arch Linux x86_64  
> **UI and documentation language:** English  
> **Last updated:** 2026-11-03

---

## 1. Goal

Build a compact Arch Linux desktop controller for Spotify Connect with a single window, an expandable queue and installable declarative skins. Cadence never plays or processes audio; Spotify audio remains in the official Spotify client.

The first usable milestone (M1) delivers a demo player plus the original default skin, before real Spotify integration.

---

## 2. Locked decisions

| Area | Decision |
|---|---|
| Product | Spotify Connect controller, not a player |
| Auth | Authorization Code + PKCE, system browser, loopback `127.0.0.1`, no client secret |
| Tokens | Refresh token in OS keyring; access token in Go memory; frontend never receives tokens |
| Window | One frameless window, 420×220 default, expandable queue |
| Skins v1 | Levels 1–2: JSON manifest/layout + CSS + local assets; no JavaScript |
| Default skin | Original “Clarity”, inspired by cPro Pure; no copied assets |
| Visualizer | Four simulated modes; no real beat detection |
| Native Winamp skins | Reference only in v1; read-only importer may be evaluated in v2 |
| Copyrighted recreations | Personal only, under `skins-local/`, excluded from Git and releases |
| Platform | Arch Linux x86_64 |
| Packaging | AppImage and `.tar.gz`; GitHub Actions CI |
| License | MIT for code; CC BY-SA 4.0 for original skin assets |

Pinned stack: Go 1.26+ (environment 1.27.3), Wails `v3.0.0-beta.34`, Svelte 5.57.2, Vite 8.3.5, TypeScript 6.0.3, Node 26.12.1.

---

## 3. Pre-development status

| Item | Status |
|---|---|
| Product scope and architecture | Done |
| Default skin direction | Done |
| Skin format scope | Done |
| Native `.wsz`/`.wal` decision | Done |
| Design document updated | Done |
| Plan saved in `plan/nombre_plan.md` | Done |
| Git repository initialized | Done |
| Wails v3 beta.34 CLI installed | Done |
| Task 3.54.0 installed | Done |
| `wails3 doctor` passes | Done |
| Spotify Developer app created | User action pending |
| Fixed OAuth callback port registered | User action pending |
| Final app name and bundle ID | Non-blocking; working name Cadence |
| Wails project scaffold | Done (M0) |
| Frontend toolchain pinned and linted | Done (M0) |
| CI skeleton | Done (M0) |

### Toolchain note

The Go-installed binaries are in `$(go env GOPATH)/bin` (`/home/ivan/go/bin`), which is not currently in `PATH`. Add this to the shell configuration:

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

`wails3 doctor` reports: “Your system is ready for Wails development!”

---

## 4. Milestones

### M0 — Repository and Wails scaffold

**Goal:** a clean Wails v3 + Svelte project that builds and runs, with the existing docs preserved.

**Status: complete.** The scaffold is pinned to Wails `v3.0.0-beta.34`, the demo service and time event were removed, and the Linux-only Taskfile now builds a frameless 420×220 shell.

Tasks:

- [x] Scaffold Wails v3 with the `svelte` template in a temporary directory.
- [x] Merge the generated `main.go`, `go.mod`, `frontend/`, `build/` and Wails configuration into the repository root without overwriting `plan/`, `docs/` or the design document.
- [x] Add the project `Taskfile.yml` and verify `wails3 dev`.
- [x] Add `.gitignore`, `.editorconfig`, README and license files.
- [x] Add the CI skeleton.
- [x] Record the exact scaffold version and template changes.

Exit criteria:

- [x] `wails3 dev` opens the Cadence window.
- [x] Filtered Go tests and frontend checks pass on the scaffold.
- [x] The repository contains no generated secrets or local-only files.
- [x] The docs and plan are still intact.

### M1 — Demo player and Clarity skin

**Goal:** a usable simulated player and the first original skin, with no Spotify dependency.

**Status: complete.** The demo provider, player store, Clarity skin and window services are implemented. The queue expands and collapses through a native GTK4 resize helper because Wails v3 beta.34 maps `SetSize` to `gtk_window_set_default_size`, which does not resize a mapped window on GTK4.

Tasks:

- [x] Define `PlayerState` and the provider interface.
- [x] Implement the demo provider with a small fake queue and deterministic track metadata.
- [x] Build the frameless 420×220 shell and custom titlebar.
- [x] Implement core components: album art, track info, progress, transport, volume, shuffle, repeat, queue, device picker placeholder and settings button.
- [x] Implement the Clarity skin with six themes and four visualizer modes.
- [x] Implement play/pause, next/previous, seek, volume, shuffle and repeat against the demo provider.
- [x] Implement queue expansion and reduced-motion behavior.
- [x] Add keyboard focus states and basic accessibility.

Exit criteria:

- [x] The app runs without Spotify.
- [x] The demo player responds to every control.
- [x] The default skin renders, animates and switches themes.
- [x] The queue expands and collapses smoothly.
- [x] No token, OAuth or Spotify code is required to run the demo.

### M2 — Skin system levels 1–2

**Goal:** install, validate, activate and remove declarative skins without recompiling.

**Status: complete.** Manifest and layout schemas, semantic validation, CSS parsing, local asset checks, directory and ZIP installation, staging with rollback, last-known-good fallback, safe mode, a Shadow DOM runtime, a skin library and the Afterglow development skin are implemented. The built-in Clarity skin remains native and always available.

Tasks:

- [x] Define JSON Schemas for `manifest.json` and `layout.json`.
- [x] Implement manifest/layout/CSS validation in Go.
- [x] Implement the Shadow DOM skin runtime and component registry.
- [x] Implement theme tokens and CSS variable application.
- [x] Implement directory and ZIP installation with staging, atomic activation and rollback.
- [x] Enforce path, size, asset-type and ZIP limits.
- [x] Implement safe mode, last-known-good skin and fallback to the built-in skin.
- [x] Add a second development skin with a deliberately different layout.
- [x] Write `docs/skin-format.md` and a skin authoring guide.

Exit criteria:

- [x] A second skin can be installed and activated without recompiling.
- [x] Invalid or malicious skins are rejected before activation.
- [x] A broken active skin cannot prevent startup.
- [x] The built-in Clarity skin is always available.
- [x] The skin format documentation is sufficient to create a new level 1–2 skin.

### M3 — Spotify Connect integration

**Goal:** control the official Spotify client with a real Premium account.

Tasks:

- Create and document the Spotify Developer app setup.
- Implement OAuth Authorization Code + PKCE with a fixed loopback callback.
- Store the refresh token in the OS keyring; keep access tokens in Go memory.
- Implement the Spotify provider, rate limiting, polling and state reconciliation.
- Implement device listing and playback transfer.
- Implement play/pause, next/previous, seek, volume, shuffle and repeat.
- Implement queue display and album-art caching/proxying.
- Implement error states: no device, Premium restriction, offline, rate limit, revoked token.
- Ensure the frontend never receives tokens.
- Test with the official Spotify client on Arch Linux.

Exit criteria:

- Cadence authenticates through the system browser.
- Cadence controls the official Spotify client.
- Device transfer and queue display work.
- Tokens are not present in frontend state, logs or the repository.
- Rate limits and error states are handled without crashing.
- Demo mode remains available without Spotify.

### M4 — Packaging and release

**Goal:** reproducible Arch Linux artifacts.

Tasks:

- Build AppImage and `.tar.gz` artifacts in an Arch Linux CI container.
- Add GitHub Actions test/build/release workflows.
- Generate SHA-256 checksums.
- Write installation and first-run documentation.
- Test on a clean Arch installation or VM.
- Finalize licenses, README, screenshots and skin authoring docs.

Exit criteria:

- A clean Arch user can install the artifact, authenticate and control playback.
- Skins can be installed from the release build.
- CI produces reproducible artifacts and checksums.
- No Windows or macOS artifacts are produced.

### M5 — v2 evaluation only

- Evaluate a read-only `.wsz` importer for a documented subset without Maki.
- Evaluate a read-only `.wal` importer for a documented subset without Maki.
- Design sandboxed level 3 modules with a restricted API, permissions and resource limits.
- Do not start level 3 until levels 1–2 are stable and documented.

---

## 5. Recommended work order

1. **M0:** scaffold Wails in a temporary directory and merge it into the repo.
2. **M1:** build the demo provider and the Clarity skin.
3. **M2:** lock the skin format and the installer before adding Spotify complexity.
4. **M3:** add OAuth and Spotify Connect only after the UI and skins are stable.
5. **M4:** package and release.
6. **M5:** evaluate v2 features only after v1 is usable.

Do not start with OAuth, native Winamp importers or level 3 modules.

---

## 6. Test gates

### Gate A — after M0

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

**Status: passed.** `task check`, `task test` and `wails3 build` pass locally. The Go suite covers schemas, CSS, traversal, symlinks, ZIP limits, staging, activation, rollback, uninstall, safe mode, last-known-good fallback and a second skin. The frontend suite covers Shadow DOM mount/theme/queue updates and invalid-layout fallback.

- [x] manifest/layout schema tests
- [x] path traversal, symlink and ZIP-bomb rejection
- [x] installer staging, activation, rollback and uninstall tests
- [x] safe-mode fallback test
- [x] listener cleanup when switching skins
- [x] second-skin installation test

### Gate D — after M3

- OAuth PKCE and state tests
- token redaction test
- Spotify client tests with `httptest` for 200/401/403/404/429/5xx
- polling and backoff tests
- no-active-device, offline and revoked-token UI tests
- manual test against the official Spotify client

### Gate E — after M4

- clean Arch install test
- AppImage and tarball smoke test
- checksum verification
- skin installation from the packaged build
- no secrets in the repository or artifacts

---

## 7. Risks and mitigations

| Risk | Mitigation |
|---|---|
| Spotify Development Mode limits | Personal use only; document the five-user limit and Premium requirement |
| OAuth redirect registration mismatch | Use one fixed loopback port; register the exact URI; document it |
| Wails v3 beta API churn | Pin `v3.0.0-beta.34`; review release notes before upgrading |
| WebKitGTK limitations | Do not use Web Playback SDK or EME; test on Arch WebKitGTK 6.0 |
| Frameless window differences on Wayland | Test on Sway; provide a fallback titlebar/drag region if needed |
| Keyring availability | Detect Secret Service availability; show a clear error if unavailable |
| Malicious skin packages | Strict schemas, path canonicalization, ZIP limits, CSP, safe mode |
| Skin CSS exfiltration | No remote URLs, no scripts, local assets only |
| Queue API limitations | Display-only queue in v1 unless a context-preserving action is confirmed |
| Album art remote content | Proxy/cache art through Go; keep CSP restricted |
| Real beat detection impossible without audio | Simulated visualizer only; document the limitation |

---

## 8. Definition of done for v1

- [x] Cadence starts on Arch Linux with Wails v3.
- [x] The demo player works without Spotify.
- [x] The Clarity skin is the default and looks correct at 420×220.
- [x] The queue expands and collapses.
- [x] Four simulated visualizer modes and six themes work.
- [ ] Spotify OAuth PKCE works through the system browser.
- [ ] The refresh token is stored in the OS keyring.
- [ ] No token appears in the frontend, logs or repository.
- [ ] Device listing and transfer work.
- [ ] Play/pause, next/previous, seek, volume, shuffle, repeat and queue work.
- [ ] Rate-limit, offline, no-device and revoked-token states are handled.
- [x] A second skin can be installed, activated and removed without recompiling.
- [x] Invalid or malicious skins are rejected or safely rolled back.
- [x] `--safe-mode` starts without third-party skins.
- [x] Skin authoring documentation is sufficient for level 1–2 skins.
- [ ] CI passes and produces AppImage and `.tar.gz` artifacts.
- [ ] A clean Arch installation can install, authenticate and control playback.

---

## 9. Immediate next actions

1. Create the Spotify Developer app and register `http://127.0.0.1:43821/callback` (see `docs/spotify-setup.md`) before starting M3.
2. Implement OAuth Authorization Code + PKCE, keyring storage and the Spotify provider.
3. Add the Spotify settings/onboarding surface and error states.

Do not begin with native Winamp importers or level 3 modules. M1 and M2 are complete; M3 requires the user's Spotify Client ID.
