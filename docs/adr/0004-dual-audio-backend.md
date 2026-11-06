# ADR 0004 — Dual audio backend

- **Status:** Accepted
- **Date:** 2026-11-06
- **Phase:** 3 (implemented)
- **Source:** [`../../odd/tasks/soloist-audio-backend.md`](../../odd/tasks/soloist-audio-backend.md)
- **Companion:** [`../architecture.md`](../architecture.md)

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

Keep go-librespot and add Soloist as an opt-in audio backend:

```text
main.go
  settings.Store ── playback.backend ──┐
                                       ├─ playback.Provider ── player.Service
  engine.Supervisor (library/auth) ────┘
  soloist.Supervisor (audio) ── soloist.Provider
  catalog.Service ── catalog.Playback ── active provider
  wailsapi/audio.Service ── merged AudioState + Soloist actions
```

- **Hybrid ownership.** go-librespot keeps library, search, context resolution,
  queue metadata and catalog authentication. Soloist owns audio playback only.
- **Startup selection.** `playback.backend` is persisted in `config.json`
  (`librespot` by default, `soloist` opt-in) and read once at startup. There is
  no runtime provider hot-swap; switching requires an app restart.
- **One provider contract.** `playback.Provider` is unchanged. The Soloist
  provider also implements the four catalog playback methods, and `catalog`
  declares the `catalog.Playback` interface it consumes, so audio can be routed
  without a package cycle and without touching the provider contract.
- **Clear device names.** In Soloist mode the catalog device is `Cadence
  Library` and the playback device is `Cadence`, so the two Spotify Connect
  devices are distinguishable.
- **Quiet library engine.** In Soloist mode go-librespot MPRIS and its audio
  cache are disabled; Soloist manages its own cache and has no MPRIS surface.
- **Key outside JSON.** The Soloist API key lives in a `0600` file outside
  `config.json`, is passed to Soloist through argv because the CLI requires it,
  and is never logged or returned to the frontend.
- **Spotify-only downloads.** Soloist is never bundled or redistributed.
  Versioned builds are downloaded from Spotify's official hosts, validated with
  `--version`, and installed atomically.

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
- Soloist builds expire 90 days after their build timestamp. Updates are manual:
  warn at 14 days, block at expiry, and link to Spotify's download page.
- The frontend consumes one merged `audio:state` event with additive
  `playbackBackend` and `soloist` fields; no new event name is introduced.

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

## Follow-up

- Phase 5: regenerate Wails bindings and add the backend selector, Soloist card,
  status/expiry warnings and radio gating in the frontend.
- Phase 6: document the Soloist setup, paths, devices, limits and expiry in
  `docs/spotify-setup.md`, `docs/installation.md` and `docs/architecture.md`.
- Phase 0/Phase 7: record the real-account spike and the manual E2E evidence in
  the ODD feature document.
