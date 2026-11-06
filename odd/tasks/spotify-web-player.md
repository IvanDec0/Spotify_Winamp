# Spotify web playback runtime (embedded Chromium + Widevine)

## Objective

Add a `web` playback backend that plays Spotify through the official Web
Playback SDK inside a bundled Chromium runtime with Widevine, so Cadence can
play without Spotify desktop/mobile and without the Soloist app pairing. Since
Phase 4 it is the default backend; librespot and Soloist remain opt-in. The
runtime is lazy, isolated and removable: the base artifact stays light and the
existing go-librespot/Soloist backends are untouched.

## Why this path

- Soloist's initial account session is an undocumented proprietary handshake.
  The API key and librespot credentials are not enough; the official app is
  required (confirmed against the real binary and the ZeroConf reference
  implementation).
- The Spotify Web Playback SDK is an official browser client. It creates a
  Spotify Connect device in Chromium and can be controlled through the Web API.
  No Spotify desktop/mobile is involved.
- The SDK needs EME/Widevine and AAC playback. Wails uses WebKitGTK on Linux,
  which does not provide Widevine. A separate Chromium runtime is required.
- Official CEF builds do not include proprietary codecs (AAC/H.264) or the
  Widevine CDM. A custom CEF build needs `proprietary_codecs=true`,
  `ffmpeg_branding=Chrome`, `enable_widevine=true`, plus the CDM and patent
  licensing. Castlabs Electron for Content Security is a prebuilt Chromium
  runtime with Widevine and proprietary codecs; it is the pragmatic v1.

## Hard truths

- This backend adds a Chromium runtime (measured: 118 MB zip, 283 MB on disk,
  381–466 MB PSS). It cannot be "light" in the same sense as the Go binary.
  The plan keeps it out of the base artifact and downloads it on demand.
- True window embedding is X11-only in practice. Wails v3 beta does not expose
  a native window handle; a separate frameless helper window is the robust v1.
  X11 child embedding can be added later with a native handle/tool.
- Linux Widevine has no VMP/persistent licenses. Streaming works; offline
  licenses are out of scope.
- The Spotify Web Playback SDK is non-commercial without Spotify's approval.
  Cadence is a personal project; keep it non-commercial.
- The helper is a separate component, not linked into Cadence. That is what
  makes removal one `rm -rf` plus deleting one package and one setting.

## Architecture

```text
main.go
  settings.playback.backend = "web" (default; librespot/soloist opt-in)
  internal/webplayer
    Provider        playback.Provider + playback.Streamer -> player.Service
    WebAPI          Spotify Web API playback control
    OAuth           PKCE, token store, refresh
    Runtime         versioned helper download/start/stop/health
    Control         loopback WebSocket to the helper
  helper (Chromium runtime, separate executable)
    loads http://127.0.0.1:<port>/player
    Spotify Web Playback SDK + Widevine
    reports device_id / ready / errors over the control socket
  local HTTP server (Go)
    /player         minimal SDK page
    /callback       OAuth PKCE redirect
    /token          short-lived access token for the SDK (loopback, nonce)
```

- Go owns playback policy: it uses the Web API (`GET /me/player/devices`,
  `PUT /me/player` with `device_ids`, `PUT /me/player/play`, pause, seek,
  volume, shuffle, repeat, queue). The helper only renders the SDK and audio.
- The SDK `device_id` is ephemeral (verified: it changes on every run). The
  provider takes it from the helper's `ready` event and re-transfers after any
  reconnect; it is never persisted.
- Player commands accept any 2xx as success. Spotify returns `204` or `200`
  with a short receipt body depending on the endpoint.
- The helper assets (`main.js`, `preload.js`, `player.html`) are embedded in
  the Go binary with `go:embed` and materialized into
  `<runtime>/resources/app/`. The only downloaded component is the castlabs
  Electron zip, verified by SHA-256.
- The SDK page is minimal HTML/JS, no framework, no external navigation. It
  calls the local `/token` endpoint for the access token and refreshes it.
- OAuth PKCE runs through the system browser. The refresh token is stored in
  `$XDG_CONFIG_HOME/cadence/web-player/tokens.json` (`0600`), never in
  `config.json`, never in the frontend, never in logs.
- One `player:state` event from the active provider; no new event names.

### Runtime choice

| Option | Widevine/AAC | Size | Build/legal cost | Verdict |
|---|---|---|---|---|
| Castlabs Electron for Content Security | prebuilt | 118 MB zip / 283 MB | MIT wrapper; CDM installed by component updater; Linux partial VMP | **Recommended v1** |
| Custom CEF + system Widevine | custom build | ~120–150 MB + CDM | Chromium build (hours/disk) + codec/CDM licensing | Optimized v2 |
| System Chrome/Firefox + SDK | system | 0 MB | none | **P0 validation / light fallback**, not embedded |
| CEF OSR into Wails | risky | + runtime | EME/GPU/perf issues | Not recommended |

The plan implements the same Go contract for all three, so the runtime choice
is an implementation detail behind `Runtime`.

## Scope

In:

- `settings.BackendWeb = "web"` as the default backend; librespot and Soloist
  stay selectable.
- `internal/webplayer`: provider, Web API client, OAuth PKCE, token store,
  runtime manager, control protocol, state mapping.
- `webplayer/helper`: castlabs Electron minimal shell (v1), embedded in
  `internal/webplayer/helper`.
- Local HTTP server: player page, OAuth callback, token endpoint.
- Runtime component: versioned download, SHA-256, size cap, atomic install,
  no partial runtime; lazy start/stop.
- Settings/UI: backend selector, login/logout, status, runtime install/update,
  error states; keep librespot/Soloist options.
- Tests and docs; removal guide.

Out:

- Bundling the runtime in the AppImage/tarball or git.
- Spotify desktop/mobile, Soloist auto-pair, PlayPlay, librespot forks.
- Commercial use, content synchronization, broadcasts, downloads.
- Persistent Widevine licenses / offline playback.
- True Wayland window embedding in v1.

## Lifecycle and performance

- **Lazy start.** The helper starts only when the `web` backend is selected and
  the user logs in or presses play. It stops when switching away or on app
  exit. No runtime process at idle in other backends.
- **One instance.** One helper, one browser process, one renderer. No multiple
  windows, no devtools, no extensions, no Node integration in the page.
- **Background audio.** Chromium flags to keep the hidden/minimized helper
  playing: `--disable-background-timer-throttling`,
  `--disable-renderer-backgrounding`,
  `--disable-backgrounding-occluded-windows`.
- **Cache bounds.** `--disk-cache-size=64MB`, `--media-cache-size=32MB`, profile
  under `$XDG_CACHE_HOME/cadence/web-player`; delete on logout/uninstall.
- **Security.** `--no-first-run --no-default-browser-check --disable-sync
  --disable-extensions --disable-component-update` (CDM update handled
  explicitly), loopback-only control port with a random nonce, navigation
  lockdown to the local page and `sdk.scdn.co`, no remote debugging exposure.
- **Budgets.** Measured in P0: 0% CPU idle, 5.6–11.6% CPU playing,
  381–466 MB PSS, 2.05–2.46 s warm start. Target: keep CPU <15%, warm start
  <3 s, and cut PSS toward 400 MB in P4 (the 350 MB goal was optimistic).
- **Stop on expire/log out.** Runtime manifest and tokens are validated before
  start; a failed login shows an actionable state, not a crash.

## Embedding strategy

- **V1: separate frameless helper window** managed by the Go runtime. It is
  positioned near the Cadence window, hidden/minimized after playback starts,
  and never steals focus. This works on X11 and Wayland.
- **V2: X11 child embedding** using the Wails X11 window ID and CEF
  `SetAsChild`/Electron `SetParentWindow` equivalent. Requires a native handle
  (Wails patch or X11/XID lookup) and is X11/XWayland-only. Native Wayland keeps
  the separate window.
- **No CEF OSR in v1.** Offscreen rendering plus Widevine/EME has performance
  and compatibility risk; a real window is the safe path.

## Tasks

### Phase 0 — Spike (decision gate)

- [x] P0.1 Validate the Web Playback SDK in system Firefox/Chrome (same
      account): OAuth PKCE, device in `GET /me/player/devices`, transfer,
      play/pause/seek/volume/queue, audio output, visualizer capture.
      Superseded: the full matrix was validated directly in the ECS helper.
- [x] P0.2 Build/run a minimal castlabs Electron shell with the same page;
      confirm EME/Widevine, AAC playback, device ID and Web API control.
- [x] P0.3 Measure runtime size, RSS, CPU idle/playing, warm start on X11 and
      Wayland.
- [x] P0.4 Gate: if Widevine/SDK playback fails in the helper, stop and keep
      the one-time Spotify app pairing path. Do not build the rest.
      Result: **passed**; see the measurements below.

## Phase 0 results (2026-11-06) — gate passed

Validated on the target machine (Wayland session with XWayland `DISPLAY=:1`,
PipeWire/PulseAudio, no system Widevine) with castlabs Electron for Content
Security `v44.9.3+wvcus` and a dedicated Spotify developer app (Authorization
Code + PKCE, `http://127.0.0.1:8888/callback`).

| Check | Result |
|---|---|
| Widevine CDM | `components.whenReady()` installed `4.10.3112.0` |
| Codec support | `canPlayType('audio/mp4; codecs="mp4a.40.2"')` = `probably` |
| Widevine + AAC | `requestMediaKeySystemAccess` = `com.widevine.alpha`, `SW_SECURE_CRYPTO` |
| SDK device | `ready` with a fresh `device_id` |
| Web API | `/me` 200, `product=premium`, `country=AR`; `/me/player/devices` 200 |
| Transfer | `PUT /me/player` 204, device active |
| Playback | `PUT /me/player/play` 204; `is_playing=true`; progress advances |
| Audio | PipeWire sink-input `application.name=webplayer-spike`; monitor RMS ~2500–3400, peak ~15k |
| Visualizer | existing `parec --device=@DEFAULT_MONITOR@` captures the same signal |
| Hidden window | audio and API progress continue after `win.hide()` |
| Warm start | 2.05–2.46 s from spawn to `ready` (X11 and Wayland) |
| Idle | 0.0% CPU, 430 MB PSS |
| Playing | 5.6–11.6% CPU, 381–466 MB PSS |
| Runtime size | 118 MB zip, 283 MB installed; SHA-256 `80cb5218…359fad` matches upstream |
| Wayland | works; the Vulkan warning is non-fatal |
| X11/XWayland | works |

Design facts confirmed by the spike:

- **The `device_id` is ephemeral.** Five runs produced five different IDs. The
  provider must take the ID from the helper's `ready` event every time and
  re-transfer after a reconnect; never persist or assume it.
- **Player commands can return 200 with a receipt body.** `pause`, `seek`,
  `shuffle`, `repeat`, `queue` and `next` returned `HTTP 200` with a
  27-character string instead of `204`. Treat any 2xx as success and never
  parse the body.
- **The visualizer needs no change.** It already captures
  `@DEFAULT_MONITOR@`, so the helper only has to play to the default sink.
- **`--disable-gpu` saves ~55 MB PSS but raises CPU** (381 MB / 11.6% vs
  436–466 MB / 5.6%). Keep GPU enabled by default; revisit in P4.
- **The helper can be embedded.** Ship `main.js`, `preload.js` and
  `player.html` inside the Go binary (`go:embed`) and materialize them into
  `<runtime>/resources/app/`. The only download is the castlabs zip; no Node,
  no asar, no separate hosting.
- **OAuth app.** A dedicated developer app is mandatory: the shared
  librespot/keymaster client ID returned a persistent `429 API rate limit
  exceeded`. The PKCE Client ID is public and can be a configurable default.

### Phase 1 — Runtime component and helper

- [x] P1.1 `internal/webplayer/helper`: embedded castlabs Electron app (main,
      preload, player page) materialized under `<root>/helper/` and passed to
      Electron as the app path; no Node in the renderer; sandbox/context
      isolation; loads the local player URL from argv/env.
- [x] P1.2 Local HTTP server in Go: `/player`, `/player.js`, `/callback`,
      `/token` with a one-time nonce; bind `127.0.0.1` only.
- [x] P1.3 Runtime manager: manifest (version, platform, URL, SHA-256, size),
      versioned install under `runtime/<version>/`, atomic rename, size caps,
      verify before exec, prune old versions.
- [x] P1.4 Control protocol: loopback WebSocket between Go and helper; messages
      `hello`, `ready{deviceId}`, `error{code,message}`, `show`, `hide`,
      `shutdown`; per-run nonce.
- [x] P1.5 Process lifecycle: Pdeathsig/process group, log redaction (no
      tokens), stop on switch/exit, no orphan.

## Phase 1 results (2026-11-06) — implemented

Verified with `go test ./...`, `go vet ./...`, `task verify` and a
build-tagged real-runtime smoke:
`go test -tags webplayer_smoke -run TestSmokeRealRuntime -v ./internal/webplayer`.

The smoke downloaded the pinned castlabs zip through the real GitHub path,
verified SHA-256, extracted and installed it, materialized the embedded helper,
launched Electron without `--no-sandbox`, and received `authentication_error`
over the control socket with a deliberately invalid token. That event proves
the page, the CSP, the SDK iframe and the preload bridge all work; it also
proves the helper stays alive after the SDK error (20 s window).

Implementation notes:

- The helper app lives under `<root>/helper/` and is passed as Electron's app
  path, instead of being written into `<runtime>/resources/app/`. The versioned
  runtime directory stays content-addressed by version and checksum, and a
  helper update does not invalidate it.
- The page glue is served as `/player.js` from the same origin. The CSP keeps
  `script-src 'self' https://sdk.scdn.co` and adds
  `frame-src https://sdk.scdn.co` for the SDK's
  `https://sdk.scdn.co/embedded/index.html` iframe; without it the SDK cannot
  authenticate or play.
- `--disable-component-update` is deliberately absent: the ECS component
  updater is what installs the Widevine CDM. `--no-sandbox` is also absent;
  unprivileged user namespaces are enabled on the target and the smoke proves
  the sandboxed runtime reaches the SDK.
- Stop sends SIGTERM to the process group and then SIGKILL. Chromium logs a
  benign `GPU process isn't usable` burst during that teardown; it does not
  happen while the helper is running. Phase 2 should prefer a `shutdown`
  control message before SIGTERM for a clean exit.

### Phase 2 — Go provider and Web API

- [x] P2.1 `settings.BackendWeb`; web becomes the default in Phase 4, while
      librespot and Soloist remain selectable.
- [x] P2.2 OAuth PKCE: authorize URL, callback exchange, refresh, token store
      `0600`, logout/clear; scopes `streaming`, `user-read-playback-state`,
      `user-modify-playback-state`, `user-read-email`, `user-read-private`.
- [x] P2.3 Web API client: devices, transfer, play/pause/next/previous/seek/
      volume/shuffle/repeat/queue; typed errors and retry/backoff on 429/5xx.
- [x] P2.4 `webplayer.Provider`: `Provider` + `Streamer` + `catalog.Playback`;
      state/queue mapping; device list from Web API; `CanSeek`/`CanControl`;
      `Stream` polls `/me/player` (~1s) and emits on change; the helper starts
      lazily on Play/PlayContext/Transfer, never in `Stream`/`State`.
- [x] P2.5 `main.go`: construct the web provider only when the backend is
      `web`; no runtime start at boot; player service unchanged.
- [x] P2.6 Tests: fake Web API, fake control socket, provider mapping, token
      store, runtime manifest/checksum; no real account.

## Phase 2 results (2026-11-06) — implemented and played

Real-account E2E passed:
`CADENCE_WEBPLAYER_E2E=1 go test -tags webplayer_e2e -run TestE2EPlayback -v ./internal/webplayer`.

The test used the stored session, started the real helper, played
`Never Gonna Give You Up - 2022 Remaster` on the `Cadence Web` device, and
captured the system monitor with **peak 15005**. The app was also launched with
`playback.backend = "web"` and the helper did **not** start at boot; the stream
polled the Web API and the app stayed healthy. It is now the default backend.

Implementation notes:

- The OAuth callback and the player page share a fixed `127.0.0.1:8888` server,
  because the dashboard entry is `http://127.0.0.1:8888/callback`. Spotify
  supports dynamic ports only when the entry omits the port, which the
  dashboard rejects in practice.
- The token store is `$XDG_CONFIG_HOME/cadence/webplayer/tokens.json` (`0600`,
  atomic). Refresh tokens rotate; `invalid_grant` clears the store and maps to
  `ErrLoginRequired`. No token reaches the frontend, logs or errors.
- The helper is lazy: `Play`, `PlayContext` and `Transfer("")` call
  `EnsureStarted`; `Stream`/`State` only poll the Web API. Installing the
  runtime is an explicit action (`InstallRuntime`): starting the helper never
  triggers a multi-minute download behind a 15 s catalog action.
- The provider reports `webplayer_login_required`, `webplayer_runtime_missing`,
  `webplayer_rate_limited`, `webplayer_premium_required`,
  `webplayer_device_missing` and `webplayer_api_error` as token-free
  `PlayerError`s. Commands accept HTTP 200/204 as success.
- The device id is read from the control `ready` event; it is never persisted.
- CLI helpers until Phase 3: `cadence -web-login`, `-web-logout` and
  `-web-install`. They are also useful for support and diagnostics.
- `main.go` keeps the go-librespot sidecar as the library session in web mode
  (`Cadence Library`, cache/MPRIS off) and only selects the web provider when
  the backend is `web`.

### Phase 3 — UX and settings

- [x] P3.1 Settings: backend `web`, "Connect Spotify" (browser OAuth), account
      label, runtime status/version, install/update, start/stop, logout.
- [x] P3.2 Onboarding: offer `web` as the "no app" option after Soloist is
      unavailable; explain the one-time browser login and the runtime download.
- [x] P3.3 Errors: runtime missing, OAuth denied/expired, device not found,
      Premium required, Widevine/CDM missing, Web API rate limit; localized.
- [x] P3.4 Player UI: device/transfer, queue, volume, seek; hide controls the
      SDK does not expose; visualizer unchanged via system capture.

## Phase 3 results (2026-11-06) — implemented

- Settings has the `web` backend option, a headless web-player section with
  account, Client ID, runtime status/version, install/update, start/stop and
  disconnect, plus localized `webplayer_*` errors.
- The Soloist onboarding dialog offers “Use the web player instead” and
  explains the restart/connect flow.
- The player UI keeps devices/transfer, queue, volume, seek and the visualizer;
  the unsupported radio action is hidden in web mode.
- `task verify` passes (301 frontend tests, all Go packages, audio check).

## Phase 3.5 — Spotify Developer automation (added)

The dashboard automation is a first-class feature, not a manual setup step:

- A separate embedded helper (`internal/developer`) drives the Spotify
  Developer dashboard in the castlabs runtime. It is **headless** when the
  persistent profile already has a session; only a real credential/login page
  shows a window, and it hides again as soon as the login completes.
- All terms are accepted automatically: the app-creation Developer Terms box,
  the Soloist `v1/consent` POST, and the Spotify OAuth consent button. The
  Cadence action is the user's explicit consent.
- The helper finds the **Cadence** developer app or creates it (name, redirect
  URI `http://127.0.0.1:8888/callback`, Web API + Web Playback SDK), then stores
  its public Client ID at `$XDG_CONFIG_HOME/cadence/webplayer/client-id`
  (`0600`). The active app is
  `8914d701933445d2800395f85d115fed`.
- The Soloist API key flow reads/generates/rotates the key through the
  dashboard wristband API and saves it through `soloist.SaveAPIKey` (`0600`).
  The previously exposed key was rotated; the old key is invalid.
- The web player OAuth uses the same embedded browser: it opens the authorize
  URL in the persistent profile, auto-clicks the localized consent
  (`Aceptar`/`Agree`), and the loopback callback completes the PKCE exchange.
  No system browser is opened.
- The persistent dashboard profile lives at
  `$XDG_CONFIG_HOME/cadence/developer/profile` (`0700`). It is a credential
  store; deleting it forces a visible login on the next automation.

Validation with the real account:

- Headless OAuth completed with the Cadence app and stored a token whose
  `clientId` is `8914d701933445d2800395f85d115fed`.
- Playback E2E played `Never Gonna Give You Up - 2022 Remaster` on
  `Cadence Web` with audio peak **15005** using the new app/token.
- The app was launched with `web` as the default backend: no helper at boot,
  the stream polled the Web API, and the app stayed healthy.
- `task verify` passes.

Note: an earlier probe created `7fe450922d8b4d31ac8bda4a1767d903` under an
unclear party/session. The verified active app and Client ID are `8914…`; if a
second empty “Cadence” app appears in the dashboard, delete the `7fe450…` one.

### Phase 4 — Embedding and performance

- [ ] P4.1 Frameless helper positioning; show/hide with the app; focus rules.
- [ ] P4.2 Optional X11 child embedding behind a flag; Wayland fallback.
- [ ] P4.3 Tune Chromium flags/cache/memory; measure budgets; diagnostics.
- [ ] P4.4 Runtime update/rollback: keep one previous version, verify, prune.

### Phase 5 — Docs, removal and verification

- [ ] P5.1 ADR `docs/adr/0005-web-playback-runtime.md`.
- [ ] P5.2 Docs: setup, OAuth app/redirect URI, runtime paths, licensing,
      limits, removal.
- [ ] P5.3 Removal guide: delete `internal/webplayer`, the runtime component,
      the `web` setting/UI; `task verify` must pass after.
- [ ] P5.4 `task verify` plus a separate `task webplayer:smoke` (not in CI by
      default) and manual E2E on X11/Wayland.

## File map

New:

- `internal/webplayer/doc.go`, `errors.go`, `manifest.go`, `manifest.json`,
  `extract.go`, `runtime.go`, `process.go`, `log.go`, `control.go`, `server.go`,
  `tokens.go`, `oauth.go`, `webapi.go`, `provider.go`, `service.go`,
  `clientid.go`, `smoke_test.go`, `e2e_test.go`, `*_test.go`
- `internal/webplayer/helper/` (`embed.go`, `assets/page.html`,
  `assets/page.js`, `assets/app/{package.json,main.js,preload.js}`)
- `internal/webplayer/runtime/manifest.json` (pinned castlabs version/checksum)
- `internal/developer/doc.go`, `errors.go`, `service.go`, `*_test.go`
- `internal/developer/helper/` (`embed.go`, `assets/app/{package.json,main.js}`)
- `internal/wailsapi/audio/developer.go`, `web.go`
- `docs/adr/0005-web-playback-runtime.md`
- `odd/tasks/spotify-web-player.md` (this plan)

Modified:

- `internal/settings/settings.go` + tests (`BackendWeb`, normalization)
- `internal/wailsapi/audio/service.go`, `types.go` + tests (backend, login,
  runtime actions)
- `main.go` (provider selection, no boot start)
- `frontend` settings/onboarding/i18n/tests
- `docs/*`

Not changed:

- `playback.Provider`, `catalog.Playback`, `internal/soloist`,
  `internal/engine`, go-librespot patches, packaging base.

## Acceptance criteria

- `web` is the default; librespot and Soloist remain selectable and the base
  artifact has no Chromium runtime (it is downloaded on demand).
- The runtime is downloaded lazily, checksum-verified, versioned and pruned;
  no partial installs.
- OAuth PKCE works through the embedded Chromium profile (headless after the
  initial login), and the refresh token is `0600` and never reaches the
  frontend/logs.
- The SDK device appears in `GET /me/player/devices`; transfer/play/pause/seek/
  volume/queue work through the Web API; audio plays and the visualizer works.
- No Spotify desktop/mobile is required.
- The runtime stops when switching backends and on app exit; no orphan.
- X11 and Wayland: separate helper window works; X11 embedding is optional and
  flagged.
- Removal leaves the app building and `task verify` passing.
- Budgets: 0% CPU idle, <15% CPU playing, <3 s warm start (measured in P0:
  0%, 5.6–11.6%, 2.05–2.46 s); PSS recorded honestly (381–466 MB) and tuned
  in P4.

## Risks

- **Widevine/codecs licensing.** Custom CEF requires patent/CDM licenses; use
  castlabs Electron or the system CDM to avoid bundling. Do not redistribute
  the CDM.
- **Spotify SDK terms.** Non-commercial only; no synchronization/broadcast;
  keep content original.
- **Runtime size.** Inherent; mitigate with lazy download and no base bundle.
- **Wayland embedding.** Not supported natively; separate window in v1.
- **Wails native handle.** Not exposed in beta; X11 embedding needs a patch or
  XID lookup.
- **Background throttling.** Verified with `win.hide()`: audio and API progress
  continue. Keep the documented flags anyway and re-test after runtime updates.
- **CDM/Chromium compatibility.** Pin the runtime and CDM; test after updates.
- **OAuth app setup.** The user needs a Spotify developer app with the local
  redirect URI; document it once.
- **Two Connect devices.** The SDK device and go-librespot/Soloist devices
  coexist; name the SDK device clearly (`Cadence Web`) and default to it only
  while the web backend is active.
- **Security.** Loopback control/token endpoints with nonce, no remote
  debugging, strict navigation allowlist, token redaction.

## Decision

**Recommended v1: castlabs Electron for Content Security** as the helper,
because it is the fastest legal path to Widevine + AAC playback and keeps the
Go app free of CEF/CGO. The same Go provider works with:
- system Chrome/Firefox for P0 and as a zero-bundle fallback;
- a custom CEF build later if the smaller runtime justifies the build and
  licensing work.

**Do not start with a custom CEF build.** It adds hours of Chromium build,
codec/CDM licensing and a new CI pipeline before the SDK is even proven.

## Next step

Phases 3 and 3.5 are complete: the web backend is the default, the developer
dashboard automation is headless, the Cadence app and Soloist key are
configured, and the real OAuth/playback E2E passed. Next is Phase 4 (embedding
and performance budgets) and Phase 5 (ADR, setup/removal docs, the
`webplayer:smoke` task and a final X11/Wayland pass).
