# Cadence — Technical Design Document

> **Status:** design baseline (2026-10-25); product architecture updated 2026-11-03<br>
> **Working name:** Cadence (final name and bundle ID pending)<br>
> **Goal:** a compact Arch Linux desktop Spotify player with installable, declarative, Winamp-inspired skins.<br>
> **Stack:** Go + Wails v3 + Svelte 5 + TypeScript + CSS + managed go-librespot engine.<br>
> **Platform:** Arch Linux x86_64 only in v1.<br>
> **UI and documentation language:** English.<br>
> **License:** MIT for Cadence code; CC BY-SA 4.0 for original skin assets; go-librespot is GPL-3.0 and runs as a separate process.

This document supersedes the original Spanish/Linux+Windows design. The skin system, window model and visual direction remain the v1 baseline. **The Spotify architecture changed on 2026-11-03:** Cadence is now a local player, not a Spotify Connect controller.

## Architecture update — 2026-11-03

The earlier design assumed the official Spotify client would remain the playback device and Cadence would control it through the Web API. That assumption was replaced:

- Audio is decoded and played by **go-librespot**, a managed sidecar process.
- Authentication uses Spotify **device pairing**, not Authorization Code + PKCE.
- The app browses the user's **playlists, saved albums, followed artists and Liked Songs** through go-librespot's local REST API.
- The official Spotify client is optional and is not required for playback.
- The frontend still never receives credentials. The engine keeps its own `state.json`.

Sections below that describe OAuth, the OS keyring or Spotify Connect control are retained as historical design context and are superseded by [`plan/nombre_plan.md`](plan/nombre_plan.md) and [`docs/spotify-setup.md`](docs/spotify-setup.md).

---

## 1. Product summary

Cadence is a desktop **Spotify player**. It pairs with a Spotify Premium account, plays audio locally through PulseAudio/PipeWire, and provides its own library, track selection and queue. Cadence does not require the official Spotify client and never exposes playback credentials to the frontend.

The product is a single compact window (420×220 by default) with an expandable queue, a full-window Library panel and a fully declarative skin system. The default skin is an original design inspired by the minimal, elegant, animated cPro Pure lineage from the 2010–2015 Winamp modern-skin era. It is not a copy and contains no third-party skin assets.

The project is for personal/local use in v1.

---

## 2. Scope

### 2.1 In scope for v1

- Spotify device pairing through `spotify.com/pair`.
- Local audio playback through a managed go-librespot sidecar and PulseAudio/PipeWire.
- Playback state: track, artists, album, album art, progress, duration, play/pause, seek, next, previous, volume, shuffle and repeat.
- In-app library: playlists, saved albums, followed artists and Liked Songs.
- Context and track selection, play-all and add-to-queue.
- Queue display from the local engine, including the currently playing item and upcoming items.
- Demo mode with a simulated provider, independent of Spotify.
- One window, compact layout, expandable queue and a full-window Library panel.
- Declarative skin system levels 1–2: JSON manifest/layout, CSS, local assets, themes, animations.
- Original default skin with four simulated visualizer modes and 4–6 themes.
- Skin installation from a directory or ZIP, validation, activation, uninstallation and safe-mode fallback.
- Settings, engine pairing state, diagnostics and logging with redaction.
- Arch Linux builds: AppImage and `.tar.gz`.
- GitHub Actions CI and reproducible release artifacts.

### 2.2 Out of scope for v1

- Official Web Playback SDK, Widevine/EME or DRM playback inside the webview.
- Embedding go-librespot as a linked library.
- Audio extraction, audio capture or audio analysis.
- Native `.wsz` or `.wal` loading.
- Maki or any executable skin code.
- Skin marketplace, online catalog or automatic updates.
- Visual skin editor.
- Level 3 executable components.
- Playlist editing, social features and full catalog search.
- Windows and macOS targets.
- Real beat detection or audio-reactive visualization.

### 2.3 Deferred to v2

- A read-only native `.wsz` importer for a documented subset, with no Maki execution.
- A read-only native `.wal` importer for a documented subset, with no Maki execution.
- Level 3 sandboxed, prebuilt modules. Skins will not ship Svelte source; modules will use a restricted, versioned API.
- Catalog search and richer library management.

---

## 3. Locked decisions

| Area | Decision |
|---|---|
| Product type | Local Spotify player with in-app library, queue and skins |
| Spotify account | Premium required; audio plays through the local engine |
| Audio engine | go-librespot managed as a separate process; loopback REST API |
| Spotify API | go-librespot local API plus its authenticated Spotify session; no Web Playback SDK |
| Authentication | Spotify device pairing (`spotify.com/pair`) |
| Credential storage | Engine-owned `state.json` under `$XDG_CONFIG_HOME/cadence/audio` |
| Frontend credential access | None. The frontend only receives token-free engine and player state |
| Window model | One window, frameless, custom titlebar, default 420×220, 460 px Library |
| Skins v1 | Levels 1–2: JSON manifest/layout + CSS + local assets; no JavaScript |
| Skins v2 | Level 3 only with sandboxed, prebuilt modules and a restricted API |
| Default skin | Original “Clarity”, inspired by cPro Pure; no copied assets |
| Visualizer | Four simulated modes; no real beat detection |
| Native Winamp skins | Reference only in v1; read-only importer may be evaluated in v2 |
| Copyrighted recreations | Personal only, under `skins-local/`, excluded from Git and releases |
| Platform | Arch Linux x86_64 only |
| Language | English UI and documentation |
| Packaging | AppImage and `.tar.gz`; no Windows, no AUR in v1 |
| License | MIT for Cadence code; CC BY-SA 4.0 for original skin assets; GPL-3.0 engine kept separate |
| CI | GitHub Actions, Arch Linux container |

### 3.1 Version pins

| Component | Version | Reason |
|---|---|---|
| Go | 1.26+ (development environment: 1.27.3) | Wails v3 requirement |
| Wails | `v3.0.0-beta.34` | Latest v3 beta considered; pin deliberately |
| Wails template | `svelte` shortname | The template already includes TypeScript |
| Node.js | 26.12.1 | Development environment |
| npm | 12.2.1 | Development environment |
| Svelte | 5.57.2 | Current stable |
| Vite | 8.3.5 | Current stable |
| `@sveltejs/vite-plugin-svelte` | 7.3.1 | Svelte 5 / Vite 8 compatibility |
| TypeScript | 6.0.3 | `svelte-check` and `typescript-eslint` compatibility |
| `svelte-check` | 4.7.7 | Current stable |
| Vitest | 5.0.5 | Frontend unit tests |
| `@testing-library/svelte` | 5.4.2 | Component tests |
| Playwright | 1.64.1 | Browser-level end-to-end tests |
| jsdom | 30.1.3 | DOM test environment |
| ESLint | 10.13.0 | Linting |
| `typescript-eslint` | 8.72.0 | TypeScript linting |
| `eslint-plugin-svelte` | 3.23.1 | Svelte linting |
| Prettier | 3.9.10 | Formatting |
| `prettier-plugin-svelte` | 4.1.1 | Svelte formatting |

TypeScript 7.0.2 is intentionally not used: `svelte-check` 4.7.7 accepts `^5 || ^6`, and `typescript-eslint` 8.72.0 requires `<6.1.0`.

### 3.2 Go dependencies

| Dependency | Version | Purpose |
|---|---|---|
| `github.com/zalando/go-keyring` | 0.2.8 | Refresh token storage via the OS keyring |
| `github.com/santhosh-tekuri/jsonschema/v6` | 6.0.3 | Manifest and layout validation |
| `github.com/adrg/xdg` | 0.5.3 | XDG config/data/cache paths |
| `github.com/tdewolff/parse/v2` | 2.8.20 | CSS parsing and validation |
| `modernc.org/sqlite` | 1.61.0 | Only if persistence later outgrows JSON |

---

## 4. Architecture

```text
┌─────────────────────────────────────────────────────────────┐
│                        Cadence (Wails)                      │
│                                                             │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                  Frontend (Svelte/TS)                 │  │
│  │                                                       │  │
│  │  App shell · Settings · Player store                  │  │
│  │  Skin runtime · Layout renderer · Theme tokens        │  │
│  │  Simulated visualizer · Core component registry       │  │
│  └──────────────────────────┬────────────────────────────┘  │
│                             │ typed Wails bindings/events    │
│  ┌──────────────────────────▼────────────────────────────┐  │
│  │                    Go services                        │  │
│  │                                                       │  │
│  │  PlayerService · SpotifyService · SkinService         │  │
│  │  SettingsService · KeyringService · Diagnostics       │  │
│  └──────────────────────────┬────────────────────────────┘  │
│                             │                               │
│  ┌──────────────────────────▼────────────────────────────┐  │
│  │ External adapters and persistence                     │  │
│  │ Spotify Web API · OS keyring · XDG files · logs        │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### 4.1 Architectural rule

**The core defines capabilities; skins define presentation.**

The core exposes stable operations and state. A skin decides how play/pause is drawn, where the queue appears and which visualizer mode is active. A skin never accesses Spotify credentials, the keyring, arbitrary files or the network directly.

### 4.2 Go packages

```text
internal/
├── app/                 # Wails lifecycle, window, CLI flags, safe mode
├── player/              # PlayerState, polling, optimistic updates, reconciliation
├── musicprovider/       # Provider interface
│   ├── provider.go
│   └── spotify/         # OAuth, API client, rate limiting, image proxy
├── skins/               # Manifest, layout, validation, installer, manager
├── settings/            # XDG config, preferences, active skin/theme
├── keyring/             # Refresh-token storage
└── diagnostics/         # Structured logs, redaction, error reporting
```

Suggested package responsibilities:

- **`app`:** Wails application setup, single-window lifecycle, frameless window options, `--safe-mode` and `--demo` flags.
- **`player`:** provider-independent state machine. Owns the canonical `PlayerState`, polling cadence, optimistic action handling, error states and event emission.
- **`musicprovider`:** small interface used by the player. The Spotify adapter is one implementation; the demo provider is another.
- **`skins`:** discovers skins, validates manifests/layouts/CSS, installs and uninstalls packages, enforces path and size limits, maintains last-known-good state and exposes skin metadata to the frontend.
- **`settings`:** persists user preferences, active skin/theme, window size and OAuth client ID. Uses XDG paths.
- **`keyring`:** stores and retrieves the Spotify refresh token. Never logs token values.
- **`diagnostics`:** structured logs with redaction, error classification and a diagnostics screen.

### 4.3 Frontend modules

```text
frontend/src/
├── app/                 # Shell, routing between player/settings, error boundary
├── components/          # Registered core components (album art, transport, queue…)
├── player/              # Player store, actions, Wails event subscription
├── skins/
│   ├── runtime/         # Shadow DOM host, skin lifecycle, safe mode
│   ├── layout/          # Declarative layout renderer and component registry
│   └── theme/           # Theme tokens, CSS variable application
├── visualizer/          # Simulated spectrum/pulse/wave/grid renderers
└── settings/            # Account, devices, skins, themes, diagnostics
```

- Svelte 5 runes for local state; a dedicated player store for canonical state.
- Skin rendering occurs inside a Shadow DOM host. Core component styles and skin styles are injected into the shadow root; app-level styles do not leak in.
- The layout renderer accepts only registered component names and typed props.
- The visualizer runs on a canvas and is driven by the player store plus a deterministic simulated beat signal.

### 4.4 Data flow

1. Go polls Spotify and emits a typed `PlayerState` event.
2. The frontend player store receives the event and updates reactive state.
3. The active skin reads state through the skin runtime and updates its rendered components.
4. A user action calls a typed Go binding (for example, `TogglePlayPause`).
5. The Go player service validates the action, applies an optimistic state change, calls Spotify and reconciles on the next poll or response.
6. Authentication and token refresh happen entirely in Go. The frontend only sees authenticated/unauthenticated state.

### 4.5 Failure boundaries

- Spotify unavailable: demo state or a clear offline/error state; the UI remains usable.
- No active device: show device picker and disable transport actions.
- Invalid skin: reject before activation; if an active skin fails at runtime, fall back to the built-in skin and show a diagnostic.
- Rate limited: back off, show retry information, do not hammer the API.
- Token revoked: clear session, show reconnect flow, keep the app usable.

---

## 5. Spotify integration

### 5.1 Product constraints

- Spotify Premium is required.
- The Spotify Developer app runs in Development Mode: the owner must have Premium and at most five users can be authorized. A shared public Client ID is not viable.
- Extended quota access is unrealistic for a personal controller and is not assumed.
- The app uses only official Web API endpoints that are available to the registered app.
- The app does not use removed or restricted catalog endpoints from the February 2026 API changes. v1 does not need new releases, categories, artist top tracks, batch fetch, other users’ playlists or playlist contents.
- The app does not use the Web Playback SDK. WebKitGTK 6.0 on Arch lacks EME (`navigator.requestMediaKeySystemAccess` is undefined even with encrypted-media enabled), so the SDK cannot run on the target platform.

### 5.2 Authentication

- Flow: OAuth 2.0 Authorization Code with PKCE (`S256`).
- Browser: system default browser, not an embedded WebView.
- Redirect URI: `http://127.0.0.1:<fixed-port>/callback`. `localhost` and generic HTTP redirects are not used. The exact URI must be registered in the Spotify Developer Dashboard.
- Port: a fixed, documented port (for example, `43821`) is used so the registered redirect URI is exact. If the port is occupied, authentication fails with a clear message and instructions.
- Client ID: non-secret, supplied via build-time environment variable or the onboarding settings screen. No client secret is used.
- PKCE verifier: generated per authorization attempt, held in memory, never logged.
- `state`: random, validated on callback.
- Refresh token: stored in the OS keyring.
- Access token: held in Go memory only; refreshed proactively before expiry.
- Scopes: `user-read-playback-state`, `user-modify-playback-state`, `user-read-currently-playing`.
- Logout: revoke/clear local tokens, remove keyring entry, return to unauthenticated state.

### 5.3 Endpoints used

| Operation | Endpoint |
|---|---|
| Current playback state | `GET /me/player` |
| Available devices | `GET /me/player/devices` |
| Transfer playback / start context | `PUT /me/player` |
| Play | `PUT /me/player/play` |
| Pause | `PUT /me/player/pause` |
| Next | `POST /me/player/next` |
| Previous | `POST /me/player/previous` |
| Seek | `PUT /me/player/seek` |
| Volume | `PUT /me/player/volume` |
| Shuffle | `PUT /me/player/shuffle` |
| Repeat | `PUT /me/player/repeat` |
| Queue | `GET /me/player/queue` |

### 5.4 Polling and state

- Playing: poll every 1–2 seconds.
- Paused or idle: poll every 5 seconds.
- Hidden/minimized: reduce polling; resume immediately on focus.
- Rate limiting: honor `429` and `Retry-After`; exponential backoff with jitter; never retry aggressively.
- Optimistic updates: play/pause, next/previous, seek, volume, shuffle and repeat update the UI immediately, then reconcile with the next poll.
- Source of truth: the Go player service. The frontend never calls Spotify directly.
- Progress: interpolate position locally between polls while playing; correct on each poll.
- Device state: track the active device, volume and restricted actions.

### 5.5 Queue

- `GET /me/player/queue` returns the currently playing item and upcoming items.
- v1 displays the queue and highlights the current item.
- Reordering the Spotify queue is not supported by the API and is not a v1 feature.
- Jumping to an arbitrary queue item is not guaranteed to preserve the current playback context; v1 treats it as display-only unless a safe API path is confirmed. A “play now” action may be added only if it does not break context semantics.
- Queue art thumbnails use the same Go image proxy as the main album art.

### 5.6 Album art and remote content

- Go fetches album art from Spotify image URLs and serves it to the frontend from a local app endpoint or as a cached data URL.
- The frontend and skins do not load arbitrary remote images. CSP keeps `img-src` restricted to the app origin, `data:` and `blob:`.
- The Go image cache is in-memory with a small LRU bound; no long-term listening history is stored.
- Skins can only reference local assets inside their own directory.

### 5.7 Error model

Explicit states for:

- Not authenticated.
- Authentication in progress.
- Premium required / account restriction.
- No active device.
- Device unavailable.
- Playback restricted.
- Rate limited.
- Token expired or revoked.
- Network offline.
- Spotify server error.
- Invalid response.

Each state has a user-facing message, a retry policy and a safe fallback. The skin runtime must render these states without crashing.

---

## 6. Skin system

### 6.1 Levels

**Level 1 — Visual themes**

- Colors, typography, borders, radii, shadows, backgrounds and spacing.
- Local icons, images and fonts.
- Validated CSS animations and transitions.
- Theme tokens exposed as CSS custom properties.

**Level 2 — Custom layouts**

- Reorder, hide and resize supported panels.
- Choose which registered components appear.
- Change the control arrangement.
- Define compact and expanded views.
- Multiple native windows are not supported in v1; the single-window model is locked.
- Responsive behavior is constrained by declared minimum sizes.

**Level 3 — Executable components (v2 only)**

- Sandboxed, prebuilt modules with a restricted, versioned API.
- No Svelte source, no arbitrary network/filesystem access and no OAuth token access.
- Capability declarations and resource limits.
- Not implemented in v1.

### 6.2 Skin package layout

```text
skins/
└── clarity/
    ├── manifest.json
    ├── layout.json
    ├── styles.css
    ├── themes/
    │   ├── graphite.css
    │   ├── ice.css
    │   └── violet.css
    └── assets/
        ├── album-frame.webp
        ├── noise.png
        └── fonts/
            └── Inter.woff2
```

- Installed skins live in the XDG data directory, e.g. `$XDG_DATA_HOME/cadence/skins/<id>/`.
- Development skins live in `skins-dev/`.
- Personal skins and copyrighted recreations live in `skins-local/`, which is gitignored and excluded from release artifacts.
- A skin must contain `manifest.json`, `layout.json` and `styles.css` at its root.

### 6.3 Manifest schema v1

```json
{
  "schemaVersion": 1,
  "id": "clarity",
  "name": "Clarity",
  "version": "1.0.0",
  "description": "Original minimal futuristic skin for Cadence.",
  "author": "Cadence",
  "license": "CC BY-SA 4.0",
  "entry": {
    "layout": "layout.json",
    "styles": "styles.css"
  },
  "capabilities": [
    "player.read",
    "player.control",
    "queue.read",
    "devices.read",
    "devices.control"
  ],
  "themes": ["graphite", "ice", "violet", "amber", "acid", "crimson"],
  "defaultTheme": "graphite",
  "visualizer": {
    "modes": ["spectrum", "radial", "oscilloscope", "grid"],
    "default": "spectrum",
    "intensity": 0.7
  },
  "window": {
    "width": 420,
    "height": 220,
    "minWidth": 380,
    "minHeight": 200,
    "queueHeight": 460,
    "resizable": true
  }
}
```

Rules:

- `schemaVersion` is an integer and is independent of the app version.
- `id` is a lowercase slug and must match the install directory.
- `capabilities` are declarations; v1 only grants the safe set required by the core.
- Unknown manifest fields are rejected in strict mode.
- No script, module, command or executable entry is allowed.
- Assets are resolved only inside the skin directory.

### 6.4 Layout schema v1

```json
{
  "schemaVersion": 1,
  "root": {
    "type": "stack",
    "children": [
      {
        "type": "component",
        "component": "visualizer",
        "props": { "mode": "spectrum", "layer": "background" }
      },
      {
        "type": "row",
        "children": [
          {
            "type": "component",
            "component": "album-art",
            "props": { "size": 96, "frame": true }
          },
          {
            "type": "column",
            "children": [
              { "type": "component", "component": "track-info" },
              { "type": "component", "component": "progress" },
              { "type": "component", "component": "transport-controls" }
            ]
          }
        ]
      },
      {
        "type": "component",
        "component": "titlebar",
        "props": { "position": "top" }
      }
    ]
  }
}
```

Registered component names for v1:

- `titlebar`
- `album-art`
- `track-info`
- `progress`
- `transport-controls`
- `volume`
- `shuffle`
- `repeat`
- `queue`
- `visualizer`
- `device-picker`
- `settings-button`

Rules:

- Only registered component names are accepted.
- Props are validated against per-component schemas.
- No raw HTML, inline event handlers or executable expressions.
- The layout tree supports `stack`, `row`, `column` and `grid` containers.
- Minimum sizes and overflow behavior are validated.

### 6.5 Theme model

Themes are CSS custom-property sets applied to the skin host. They may also define visualizer palettes and motion intensity.

Example:

```css
:root {
  --bg: #0b0d10;
  --panel: rgba(18, 22, 28, 0.82);
  --text: #e8edf4;
  --muted: #8a94a6;
  --accent: #4de3ff;
  --accent-2: #b26bff;
  --glow: 0 0 18px rgba(77, 227, 255, 0.35);
  --radius: 10px;
  --font-ui: "Inter", sans-serif;
  --font-mono: "JetBrains Mono", monospace;
}
```

The default skin ships six original palettes: **Graphite**, **Ice**, **Violet**, **Amber**, **Acid** and **Crimson**. Palette names and exact values are original and not copied from any Winamp skin.

### 6.6 Default skin: “Clarity”

**Reference:** cPro Pure (2012–2013) by Veroka. Pure is used only as a design reference for minimalism, elegance and animated beat visualizations. No assets, code, names or exact layouts are copied.

**Design goals:**

- Minimal, precise, futuristic; not busy chrome.
- Thin lines, restrained glow, generous negative space.
- Compact player that feels alive without becoming a dashboard.
- Clear hierarchy at 420×220.
- Works with keyboard and reduced-motion preferences.

**Window and layout:**

- Frameless window with a custom titlebar and draggable region.
- Default 420×220; queue expands to approximately 420×460.
- Album art on the left (96×96, subtle reflection/frame).
- Track title, artists and album in the center.
- Progress bar and time labels along the bottom edge.
- Transport controls on the right or bottom-right, depending on the theme.
- Visualizer as a low-contrast background layer, not an opaque panel.
- Queue slides down beneath the player; the main layout remains visible.
- Device picker and skin/settings access in the titlebar or overflow menu.

**Visualizer modes (simulated):**

1. **Spectrum** — thin mirrored bars behind the album art and lower edge.
2. **Radial pulse** — concentric rings around the play button, reacting to play state and a simulated beat.
3. **Oscilloscope** — a single luminous waveform across the progress area.
4. **Grid/particles** — a sparse dot grid or particle field that drifts at idle and pulses on simulated beats.

**Motion:**

- Play: a short pulse and accent bloom.
- Pause: slow breathing/desaturation; no constant high-energy motion.
- Track change: a horizontal light sweep and album-art crossfade.
- Hover: subtle glow/trace on interactive elements.
- Queue: slide/fade with a short, interruptible transition.
- Progress: smooth interpolation between polls.
- Reduced motion: disable pulses, particles and sweeps; keep state changes legible.

**Important limitation:** Cadence does not receive audio. The visualizer is a **simulation** driven by playback state, elapsed time, track changes and a deterministic pseudo-random signal. It is not beat detection or audio analysis. A future version could evaluate optional audio analysis, but that is outside v1.

**Assets and fonts:**

- Original SVG icons rendered by core components.
- Original CSS/SVG/WebP textures.
- OFL-licensed fonts bundled locally (for example, Inter for UI and JetBrains Mono for time/technical labels).
- No remote fonts, no copied Winamp assets, no Spotify trademarks used as skin branding.

### 6.7 Skin validation and security

**Manifest and layout**

- JSON Schema validation with strict unknown-field rejection.
- Maximum manifest size: 64 KB; maximum layout size: 256 KB.
- Maximum layout depth: 12; maximum node count: 500.
- Only registered component names and typed props.
- Reject unsupported schema versions and unsupported capabilities.

**Assets**

- Allowed types: PNG, JPEG, WebP, WOFF2. SVG is not accepted from third-party skins in v1; core icons are app-owned.
- Maximum single asset: 10 MB; maximum total skin: 50 MB uncompressed; maximum file count: 500.
- Reject absolute paths, `..`, symlinks, hard links, device files, duplicate names and case-collision paths.
- Canonicalize and verify every resolved path remains inside the skin directory.

**ZIP installation**

- Manifest must be at the archive root.
- Maximum ZIP size: 20 MB; maximum uncompressed size: 100 MB; maximum compression ratio: 100:1.
- Reject encrypted archives, nested archives and entries with unsupported methods.
- Validate into a staging directory before activation.
- Activate atomically; keep the previous skin as rollback.
- Uninstall only removes the managed skin directory.

**CSS**

- Parse and validate CSS before activation.
- Reject `@import`, remote `url()`, `javascript:`, `expression()`, `behavior`, `-moz-binding` and remote `@font-face` sources.
- Allow only local asset URLs inside the skin directory.
- Maximum CSS size: 1 MB.
- No HTML injection surface: layout is JSON, not HTML.

**Runtime isolation**

- Shadow DOM host for skin styles and components.
- CSP with scripts disabled for skin content; no `eval`, `new Function`, inline event handlers or script tags.
- Skins never receive OAuth tokens, keyring access, arbitrary filesystem access or network access.
- The core component registry is the only rendering surface.

**Safe mode**

- Built-in default skin is always available and cannot be uninstalled.
- If the active skin fails validation or runtime mount, fall back to the built-in skin.
- Persist last-known-good skin and theme.
- `--safe-mode` starts with the built-in skin and disables third-party skin loading.
- The settings screen exposes “disable third-party skins” and “reset skin state”.

### 6.8 Installation flow

1. User selects a skin directory or ZIP.
2. Go validates archive structure and size limits.
3. Go extracts to a staging directory.
4. Manifest, layout, CSS and assets are validated.
5. Skin metadata is shown for confirmation (name, version, author, license, capabilities).
6. Installer copies/renames into the XDG skin directory atomically.
7. Skin manager updates the catalog and emits an event.
8. Activation mounts the skin; on failure, the previous skin is restored.

v1 does not include a marketplace, remote downloads or automatic updates.

---

## 7. Repository layout

```text
cadence/
├── main.go
├── go.mod
├── Taskfile.yml
├── internal/
│   ├── app/
│   ├── player/
│   ├── musicprovider/
│   │   ├── provider.go
│   │   └── spotify/
│   ├── skins/
│   ├── settings/
│   ├── keyring/
│   └── diagnostics/
├── frontend/
│   ├── src/
│   │   ├── app/
│   │   ├── components/
│   │   ├── player/
│   │   ├── skins/
│   │   │   ├── runtime/
│   │   │   ├── layout/
│   │   │   └── theme/
│   │   ├── visualizer/
│   │   └── settings/
│   └── package.json
├── skins-dev/
│   └── clarity/
├── skins-local/          # gitignored; personal skins/recreations
├── docs/
│   ├── architecture.md
│   ├── skin-format.md
│   ├── skin-api.md
│   └── spotify.md
├── build/
└── .github/
    └── workflows/
        └── ci.yml
```

- `skins-dev/` holds development skins and may not ship in the binary.
- `skins-local/` is for personal use only, including any downloaded `.wsz`/`.wal`-derived recreations. It is excluded from Git and releases.
- Installed third-party skins live under the XDG data directory.
- The repository may keep its current directory name; the app name is independent.

---

## 8. Toolchain and bootstrap

### 8.1 Arch Linux prerequisites

- Go 1.26+.
- Node.js 26+ and npm.
- GTK4 and WebKitGTK 6.0 development packages.
- `libsoup3`, `base-devel`, `git`.
- Wails v3 CLI installed with the pinned version.
- Task runner (Task) for the project Taskfile.

Exact package names and Wails setup steps must be verified against the installed Wails v3 documentation and `wails3 doctor`; do not copy Wails v2 commands.

### 8.2 Bootstrap

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.34
wails3 setup
wails3 doctor
wails3 init -n cadence -t svelte
cd cadence
wails3 dev
```

Notes:

- The `svelte` template includes TypeScript; `svelte-ts` is not assumed to exist in this Wails version.
- Verify available templates with the CLI before initializing.
- `wails3 doctor` must pass before feature work.
- Pin Wails and do not auto-update beta versions without reviewing release notes.

### 8.3 Configuration paths

- Config: `$XDG_CONFIG_HOME/cadence/config.json` (fallback `~/.config/cadence/`).
- Data: `$XDG_DATA_HOME/cadence/` (fallback `~/.local/share/cadence/`).
- Cache: `$XDG_CACHE_HOME/cadence/` (fallback `~/.cache/cadence/`).
- Logs: under the cache or state directory, with rotation and redaction.

---

## 9. Development milestones

### Milestone 0 — Repository and toolchain

- Initialize Git and the Wails v3 Svelte project.
- Add the package structure, Taskfile, linting, formatting and CI skeleton.
- Rewrite project documentation in English for Arch Linux.
- Verify `wails3 doctor`, Go tests, frontend checks and a development build.

**Exit:** clean scaffold builds and runs; CI is green.

### Milestone 1 — Demo player and default skin

- Define `PlayerState` and the demo provider.
- Build the compact app shell and core components.
- Implement play/pause, next/previous, seek, volume, shuffle and repeat against the demo provider.
- Implement the Clarity skin with the 420×220 layout, queue expansion and four visualizer modes.
- Implement basic theme switching and reduced-motion behavior.

**Exit:** the app runs without Spotify; the demo player is usable; the default skin renders and animates; controls update state.

### Milestone 2 — Skin system levels 1–2

- Implement manifest/layout schemas and validation.
- Implement theme tokens and CSS validation.
- Implement directory and ZIP installation with staging, atomic activation and rollback.
- Implement skin manager UI, activation, uninstallation and safe mode.
- Add a second development skin with a deliberately different layout to prove the system.
- Write `docs/skin-format.md` and a skin authoring guide.

**Exit:** a user can install, activate and remove a skin without recompiling; invalid skins are rejected; a broken skin cannot prevent startup.

### Milestone 3 — Local Spotify playback and library

- Implement a managed go-librespot sidecar with loopback REST control.
- Pair through Spotify's device flow; keep engine credentials outside the frontend.
- Implement the local provider, playback controls, queue display and album-art proxy.
- Implement the in-app library: playlists, saved albums, artists and Liked Songs.
- Implement context playback, individual track selection and add-to-queue.
- Test with a real Premium account and local PulseAudio/PipeWire output on Arch.

**Exit:** Cadence plays Spotify audio locally without the official client; the library and queue work in-app; credentials never reach the frontend.

### Milestone 4 — Packaging and release

- Build AppImage and `.tar.gz` artifacts in an Arch Linux CI container.
- Bundle the pinned engine or document it as a required sidecar artifact.
- Add GitHub Actions release workflow, checksums and install instructions.
- Verify on a clean Arch environment.
- Finalize license files, README, screenshots and skin authoring docs.

**Exit:** a clean Arch user can install Cadence, build or install the engine, pair and play, and install a skin from the release artifacts.

### Milestone 5 — v2 evaluation (not v1)

- Evaluate a read-only `.wsz` importer subset without Maki.
- Evaluate a read-only `.wal` importer subset without Maki.
- Design sandboxed level 3 modules with a restricted API, permissions and resource limits.
- Do not start level 3 until levels 1–2 are stable and documented.

---

## 10. Testing and quality

### 10.1 Go tests

- Manifest/layout schema validation (valid and invalid cases).
- Path traversal, symlink, duplicate-name and ZIP-bomb rejection.
- Installer staging, atomic activation, rollback and uninstall.
- OAuth PKCE generation, state validation and callback handling.
- Spotify client behavior with `httptest`: success, 401, 403, 404, 429, 5xx, malformed JSON.
- Polling cadence, backoff and optimistic update reconciliation.
- Settings persistence and keyring abstraction.
- Race detector and `go vet` in CI.

### 10.2 Frontend tests

- Player store state transitions and action dispatch.
- Skin runtime mount/unmount and listener cleanup.
- Layout renderer with valid and invalid layouts.
- Theme application and CSS variable isolation.
- Visualizer simulation determinism and reduced-motion behavior.
- Error states and safe-mode fallback.
- Component tests with Vitest + Testing Library; jsdom for DOM tests.

### 10.3 End-to-end tests

- Playwright against a browser build with mocked Wails bindings and a mock provider.
- Flows: startup, demo playback, skin switch, skin install error, queue expansion, settings, safe mode.
- A real Wails GUI smoke test is manual in v1; CI focuses on build + unit/component tests to avoid headless WebKitGTK fragility.

### 10.4 CI

- GitHub Actions workflow in an Arch Linux container.
- Install Go, Node.js, GTK4/WebKitGTK 6.0 and build dependencies.
- Run `go test ./...`, `go vet ./...`, `npm ci`, `npm run check`, `npm run lint`, `npm run test` and `npm run build`.
- Build the Wails application and produce artifacts.
- Cache Go modules and npm dependencies.
- Fail on formatting or type errors.
- Release workflow builds AppImage and `.tar.gz` and uploads checksums.

---

## 11. Packaging and distribution

- Target: Arch Linux x86_64.
- Formats: AppImage and `.tar.gz` containing the binary, desktop file, icon, licenses and install notes.
- Build in an Arch Linux container for ABI consistency.
- Test the artifacts on a clean Arch installation or VM.
- GitHub Releases with version tags and SHA-256 checksums.
- No Windows or macOS artifacts.
- No AUR package in v1; it can be added later if the release process stabilizes.
- Versioning: SemVer for the app; `schemaVersion` for skins.
- License files: MIT for code; CC BY-SA 4.0 for original skin assets; third-party notices for fonts and dependencies.
- Trademark note: Spotify, Winamp and cPro are trademarks of their respective owners; Cadence is not affiliated with them.

---

## 12. Security and privacy

### 12.1 Credentials

- OAuth client secret: never used.
- Refresh token: OS keyring only.
- Access token: Go memory only; never written to disk or logs.
- Frontend: receives player state and commands only; no tokens, no keyring access, no Spotify API calls.
- Logs: redact authorization codes, tokens, PKCE verifiers and client IDs where appropriate.

### 12.2 Skin threat model

- Malicious skin ZIP: path traversal, symlinks, ZIP bombs, duplicate entries, oversized assets.
- Malicious CSS: remote requests, font/script injection, UI redress.
- Malicious layout: unknown components, oversized trees, invalid props.
- Runtime failure: broken skin must not prevent startup or block playback controls.
- Mitigations: strict schemas, allowlists, path canonicalization, size limits, CSS parsing, Shadow DOM, CSP, no scripts, safe mode.

### 12.3 Privacy

- No telemetry.
- No listening history persisted in v1.
- Queue and player state are in memory; only preferences, active skin/theme and window size are persisted.
- Album art is fetched by Go and cached in memory; no third-party requests from skin CSS.
- Diagnostics are local and opt-in for sharing.

---

## 13. Acceptance criteria for v1

1. Cadence starts on Arch Linux with Wails v3 and shows the default Clarity skin.
2. The demo provider works without Spotify: play/pause, next/previous, seek, volume, shuffle, repeat and queue display.
3. The window is compact (420×220), frameless, draggable and the queue expands smoothly.
4. The default skin has four simulated visualizer modes, six themes and reduced-motion support.
5. Spotify OAuth Authorization Code + PKCE works through the system browser with a loopback redirect.
6. The refresh token is stored in the OS keyring; no token appears in the frontend, logs or repository.
7. Cadence lists devices, transfers playback and controls the official Spotify client.
8. Playback state, progress, volume, shuffle, repeat, device and queue are displayed correctly.
9. Rate limits, no-active-device, offline and revoked-token states are handled without crashing.
10. A second skin can be installed from a directory or ZIP without recompiling.
11. Invalid, malicious or broken skins are rejected or safely rolled back.
12. The built-in skin cannot be removed, and `--safe-mode` starts without third-party skins.
13. `docs/skin-format.md` is sufficient to author a new level 1–2 skin without touching Go or Svelte.
14. CI passes and produces AppImage and `.tar.gz` artifacts.
15. A clean Arch installation can install the artifact, authenticate and control playback.

---

## 14. Open decisions (non-blocking)

- Final product name and bundle ID. Working name: **Cadence**; provisional bundle ID: `io.github.<user>.cadence`.
- Final default palette values and font pairing; the implementation may adjust the six proposed themes.
- Exact fixed OAuth callback port; it must be documented and registered exactly.
- Whether v1 queue includes a safe “play now” action; display-only is the default unless a context-preserving API path is confirmed.
- Whether a tray icon or global shortcuts are added after v1; neither is required for the first milestone.
- Final repository hosting details if not GitHub.

---

## 15. References

### Project references

- Wails v3: https://v3.wails.io/
- Wails v3 API: https://v3.wails.io/reference/overview/
- Wails v3 FAQ: https://v3.wails.io/faq/
- Svelte: https://svelte.dev/
- TypeScript: https://www.typescriptlang.org/docs/
- Spotify Web API: https://developer.spotify.com/documentation/web-api
- Spotify Developer Policy: https://developer.spotify.com/policy
- Spotify Authorization Code + PKCE: https://developer.spotify.com/documentation/web-api/tutorials/code-pkce-flow
- Spotify Player endpoints: https://developer.spotify.com/documentation/web-api/reference/#/operations/get-information-about-the-users-current-playback
- cPro Pure (visual reference only): https://www.deviantart.com/veroka/art/cPro-Pure-339309269
- Winamp Skin Museum (reference): https://skins.webamp.org/

### Design reference note

cPro Pure is a 2012–2013 Winamp cPro skin by Veroka with four beatvis animations. It is referenced for mood, restraint and the idea of multiple animated visualization modes. Cadence does not load cPro skins, execute Maki, copy assets or reproduce the original layout. The default Clarity skin is an original design.
