# ADR 0004 — Dual audio backend

- **Status:** Accepted; amended for default Soloist, automatic install and guided
  key onboarding
- **Date:** 2026-11-06
- **Implemented:** Phases 1–5 of the original ODD feature, plus Phases A–B of
  [`../../odd/tasks/soloist-onboarding.md`](../../odd/tasks/soloist-onboarding.md)
- **Source:** [`../../odd/tasks/soloist-audio-backend.md`](../../odd/tasks/soloist-audio-backend.md)
- **Companion:** [`../architecture.md`](../architecture.md), [`../spotify-setup.md`](../spotify-setup.md)

## Context

Cadence's pinned go-librespot fails on some Premium accounts while retrieving
audio keys:

```text
failed retrieving audio key: failed retrieving aes key with code 1
skipping track: Spotify refused the audio key (code 1)
```

`code 1` is a server-side refusal of the account's legacy AES key request. It is
not fixed by another bitrate, a re-login, a newer engine or a different network;
the same account and machine play correctly through Spotify's official player,
Soloist. Soloist, however, exposes no library or catalog API: it can play URIs,
but it cannot browse, search or resolve contexts.

The public Spotify Web API and the Web Playback SDK are not a replacement either:
they require their own OAuth application, do not cover the local library flows
Cadence relies on, and still do not solve DRM playback on the affected accounts.

## Decision

Use Soloist as the default audio backend and keep go-librespot as the catalog
engine and an explicit fallback:

```text
main.go
  settings.Store ── playback.backend ──┐
                                       ├─ playback.Provider ── player.Service
  engine.Supervisor (library/auth) ────┘
  soloist.Supervisor (audio) ── soloist.Provider
  catalog.Service ── catalog.Playback ── active provider
  wailsapi/audio.Service ── merged AudioState + Soloist actions
             └─ Soloist maintenance loop (install, update, prune, start)
```

- **Hybrid ownership.** go-librespot keeps library, search, context resolution,
  queue metadata and catalog authentication in both backend modes. Soloist owns
  audio playback only.
- **Soloist by default.** `settings.Default()` and
  `NormalizePlaybackBackend` resolve empty and unknown values to `soloist`.
  Configs without a `playback` section move to Soloist; an explicit `librespot`
  is preserved as the escape hatch for accounts that work with go-librespot.
- **Startup selection.** `playback.backend` is persisted in `config.json` and
  read once at startup. There is no runtime provider hot-swap; switching requires
  an app restart.
- **Backend-owned setup.** The audio adapter runs a maintenance loop at startup
  and every six hours while `soloist` is selected: install when missing, fetch a
  new build when the installed one is at least 89 days old or within 24 hours of
  expiry, prune old builds, and start Soloist when a key exists. Without a key it
  stays stopped so onboarding can ask for one. The loop runs outside the window,
  never blocks startup, and is cancelled on shutdown. In `librespot` mode it does
  not run, so automatic download and updates stop; manual controls remain.
- **Downloaded, never bundled.** Soloist is not part of the AppImage, tarball,
  checksums or repository. The ~10 MB build is downloaded from Spotify's official
  hosts at runtime, validated with `soloist --version`, and installed atomically
  under a build-numbered directory. Only Spotify hosts are accepted.
- **Update before expiry, keep two builds.** A successful update installs next to
  the running build and restarts Soloist when it was running, so the new build is
  active before day 90. Pruning keeps the newest build and the one currently
  running, and removes interrupted download/staging directories older than 24
  hours. Network failures retry silently; a failed first download surfaces a
  localized retry in the setup dialog and the Settings card.
- **Guided one-time API key.** Spotify requires an authenticated dashboard
  session, accepting the terms and clicking **Generate API key**; there is no
  API to create the key and the dashboard is not automatable. Cadence opens the
  official dashboard in the system browser, accepts one paste, validates it,
  stores it in a `0600` file outside `config.json`, and starts Soloist. The
  prompt is shown once per install: **Later** stores a dismissal in the webview
  `localStorage` key `cadence.soloist.prompt.v1`, and a Browse banner plus the
  Settings card remain the way back.
- **One provider contract.** `playback.Provider` is unchanged. The Soloist
  provider also implements the four catalog playback methods, and `catalog`
  declares the `catalog.Playback` interface it consumes, so audio can be routed
  without a package cycle and without touching the provider contract.
- **Clear device names.** In Soloist mode the catalog device is `Cadence
  Library` and the playback device is `Cadence`, so the two Spotify Connect
  devices are distinguishable.
- **Quiet library engine.** In Soloist mode go-librespot MPRIS and its audio
  cache are disabled; Soloist manages its own cache and has no MPRIS surface.
- **Key outside JSON.** The Soloist API key is passed to Soloist through argv
  because the CLI requires it, and is never logged or returned to the frontend.

## Consequences

- Changing the backend is a restart, not an instant switch. The settings UI must
  show a restart notice when the saved value differs from the running one.
- Soloist mode exposes two Connect devices (`Cadence` and `Cadence Library`); the
  user must select `Cadence` for playback.
- Spotify radio stays unavailable in Soloist mode because go-librespot resolves
  radio through a private session that it does not expose. The UI hides it.
- `skipToURI` plays a standalone track in Soloist mode: the Soloist `play`
  command accepts a URI but no context offset.
- go-librespot-only controls (bitrate, FLAC, normalisation, crossfade, cache,
  MPRIS, device group) are disabled in Soloist mode; Soloist uses its defaults.
- Process supervision stays Linux-first: `Setpgid`, `Pdeathsig` and Unix
  signalling match the rest of Cadence. The installer still maps macOS and
  Windows archives, but those platforms are not advertised yet.
- Soloist builds expire 90 days after their build timestamp. Cadence updates them
  in the background before expiry and restarts Soloist after the switch; the
  Settings card still shows the remaining days, a manual **Update Soloist**
  action, and the expiry block.
- An expired build blocks playback and offers the update; it cannot be revived.
- The one-time key is the only manual step. It cannot be generated or fetched by
  Cadence, and a rejected key must be replaced in the dashboard.
- The frontend consumes one merged `audio:state` event with additive
  `playbackBackend` and `soloist` fields; no new event name is introduced.
- Automatic setup only runs in Soloist mode. A user who selects `librespot` gets
  the old manual behavior and no background downloads.

## Alternatives rejected

- **Replace go-librespot with Soloist.** Soloist cannot browse, search, resolve
  contexts or maintain the library, so Cadence would lose its catalog.
- **Spotify Web API / Web Playback SDK.** Requires an additional OAuth app,
  duplicates catalog work and does not solve the affected-account playback.
- **Fork go-librespot to fix `code 1`.** The refusal is server-side; a local fork
  cannot change Spotify's key policy and would add long-term maintenance.
- **Runtime backend hot-swap.** The provider, catalog router, process lifecycle
  and generated engine config all depend on the backend. A restart is
  predictable; hot-swapping adds races for little user benefit.
- **Headless browser, embedded Chromium or dashboard automation.** Spotify
  requires an authenticated dashboard session, accepting the terms and clicking
  **Generate API key**; there is no public API to create developer keys. A
  headless browser would add hundreds of MB, need the user's Spotify password or
  session, fight CAPTCHA/2FA and break on dashboard changes. The one-click
  handoff plus a paste is the lightest safe flow.
- **Bundle or redistribute Soloist.** It would grow the artifact and is not
  allowed by Spotify's terms. The runtime download keeps the release light and
  the checksums free of the archive.
- **Block the window until Soloist is ready.** The download needs no secret and
  the app must stay usable without a network or key. Setup is a background pass
  plus a prompt that can be dismissed immediately.

## Follow-up

- Phase 0: confirm on the failing account that Soloist plays, capture the real
  URI matrix and the exact start-failure strings, and record the evidence in the
  ODD feature document.
- Manual E2E on the failing machine: fresh config → Soloist default →
  auto-download → guided key → pairing → playback; then an old build (89 days)
  to verify the background update, the restart and the prune.
- Documentation lives in [`../spotify-setup.md`](../spotify-setup.md),
  [`../installation.md`](../installation.md) and
  [`../architecture.md`](../architecture.md).
