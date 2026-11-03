# Cadence

> **Working title.** A compact Arch Linux Spotify player with installable, declarative, Winamp-inspired skins.

## Status

**Local playback implemented.** Cadence is now a real Spotify player: it runs **go-librespot** as a managed local audio engine, pairs through Spotify's device flow, browses the user's playlists, saved albums, followed artists and Liked Songs, and plays audio through PulseAudio/PipeWire. The official Spotify client is not required. The declarative skin system and the demo player remain available.

- Implementation plan: [`plan/nombre_plan.md`](plan/nombre_plan.md)
- Development workflow: [`docs/development.md`](docs/development.md)
- Skin format reference: [`docs/skin-format.md`](docs/skin-format.md)
- Skin authoring guide: [`docs/skin-authoring.md`](docs/skin-authoring.md)
- Spotify engine setup: [`docs/spotify-setup.md`](docs/spotify-setup.md)
- Original design baseline: [`documentacion-cliente-musical-skins.md`](documentacion-cliente-musical-skins.md)

## What it is

Cadence is a desktop **Spotify player** with a compact single window, an expandable queue and installable skins.

- **Audio is local.** A bundled or externally installed `go-librespot` process decodes and plays Spotify audio through PulseAudio or PipeWire.
- **No Spotify Desktop dependency.** Cadence appears to Spotify as a device named **Cadence** and is paired once through `spotify.com/pair`.
- **The library lives inside Cadence.** Playlists, saved albums, followed artists and Liked Songs are browsed and started from the Library panel.
- **Skins are declarative.** The built-in Clarity skin is native; third-party skins are JSON layouts, CSS and local assets with no JavaScript.
- **The demo player remains available.** Run `cadence --demo` to use the app without Spotify or network access.

Spotify playback requires a **Premium** account. The app uses the user's own account with an unofficial client engine; see the disclaimer below. The frontend never receives playback credentials.

The app uses a single compact window (420×220 by default) with an expandable queue. The default skin, **Clarity**, is an original design inspired by the minimal, elegant, animated cPro Pure lineage from the 2010–2015 Winamp modern-skin era. It contains no copied skin assets.

## What it is not

- Not the official Spotify client and not affiliated with Spotify.
- Not a Spotify Connect controller; it does not need the official desktop client to play audio.
- Not a Web Playback SDK or DRM integration. WebKitGTK cannot run Spotify's Widevine/EME playback.
- Not an audio extractor or audio analyzer; the visualizer is still simulated.
- Not a native Winamp skin loader in v1.
- Not a marketplace, visual skin editor or level 3 executable-skin runtime in v1.

## Architecture

```text
┌──────────────────────────── Cadence ────────────────────────────┐
│  Svelte 5 UI  ── Wails service calls/events ──  Go services     │
│                                                  │              │
│                                       player.Service            │
│                                                  │              │
│                                    golibrespot.Provider         │
│                                                  │              │
│                                    golibrespot.Runtime          │
└──────────────────────────────────────────────────┼──────────────┘
                                                   │ loopback HTTP
                                                   ▼
                                     ┌───────────────────────────┐
                                     │ go-librespot sidecar      │
                                     │ Spotify session + audio   │
                                     └─────────────┬─────────────┘
                                                   │ PulseAudio
                                                   ▼
                                            Speakers / headphones
```

- `internal/golibrespot/runtime.go` owns the sidecar process, its config and pairing state.
- `internal/golibrespot/client.go` is the loopback REST client.
- `internal/golibrespot/provider.go` maps engine state to the provider-independent player model.
- `internal/golibrespot/service.go` exposes pairing, library and playback selection to the frontend.
- `frontend/src/lib/MusicBrowser.svelte` and `music.svelte.ts` implement the Library panel.

The engine binds only to `127.0.0.1` on a random port and stores its credentials in `$XDG_CONFIG_HOME/cadence/audio/state.json`. The frontend receives token-free state.

## Stack

- Go 1.26+ (development environment: 1.27.3)
- Wails `v3.0.0-beta.34` (pinned)
- Svelte 5.57.2
- Vite 8.3.5
- TypeScript 6.0.3
- Vitest 5.0.5, ESLint 10.13.0, Prettier 3.9.10
- Node.js 26.12.1 / npm 12.2.1
- go-librespot pinned to `v0.10.6-0.20261102145552-522adcae8d65` (GPL-3.0 binary, separate process)

TypeScript 7 is intentionally not used because `svelte-check` 4.7.7 and `typescript-eslint` 8.72.0 are not compatible with it.

## Platform

Arch Linux x86_64 only in v1.

## Documentation

| Document | Purpose |
|---|---|
| [`plan/nombre_plan.md`](plan/nombre_plan.md) | Actionable implementation plan and test gates |
| [`docs/development.md`](docs/development.md) | Local setup, commands and tooling notes |
| [`docs/skin-format.md`](docs/skin-format.md) | Skin package, layout, theme and security reference |
| [`docs/skin-authoring.md`](docs/skin-authoring.md) | Step-by-step guide to creating and installing a skin |
| [`docs/spotify-setup.md`](docs/spotify-setup.md) | Local engine build, pairing, library and troubleshooting |
| [`documentacion-cliente-musical-skins.md`](documentacion-cliente-musical-skins.md) | Original design baseline (historical architecture sections) |
| [`docs/archive/documentacion-cliente-musical-skins.es.md`](docs/archive/documentacion-cliente-musical-skins.es.md) | Original Spanish design document (archived) |

## Prerequisites

Install the Arch Linux development and audio dependencies:

```bash
sudo pacman -S --needed go nodejs npm gtk4 webkitgtk-6.0 libsoup3 base-devel git pkgconf \
  libogg libvorbis flac mpg123 alsa-lib libpulse
```

PulseAudio or PipeWire with `pipewire-pulse` must be running for playback.

Install the pinned Wails CLI and Task:

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.34
go install github.com/go-task/task/v3/cmd/task@v3.54.0
```

Add the Go binary directory to `PATH`:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

Verify the toolchain:

```bash
wails3 doctor
wails3 version
task --version
```

## Development

Build the local audio engine and run the application:

```bash
task audio:build   # builds the pinned go-librespot into bin/go-librespot
task dev
```

Run all checks and tests:

```bash
task check
task test
```

Build the native binary or the release artifacts:

```bash
task build     # bin/cadence
task package   # AppImage, tar.gz and SHA-256 checksums in bin/
```

The engine binary is not committed. `task audio:build` downloads the pinned source and builds it with the local CGO libraries. Cadence also discovers `go-librespot` from `CADENCE_GO_LIBRESPOT`, `PATH` or next to the Cadence binary.

## Spotify setup

1. Build the engine with `task audio:build`.
2. Start Cadence without `--demo`.
3. Open the provider badge in the titlebar to show the Library panel.
4. If the engine is not paired, copy the pairing code or choose **Open pairing page**.
5. Approve **Cadence** at `spotify.com/pair`.
6. Browse playlists, albums, artists and Liked Songs; play a context or select a track.

Playback requires Spotify Premium and an audio sink. The engine runs as a separate process; its log is written to `$XDG_CACHE_HOME/cadence/logs/go-librespot.log`.

See [`docs/spotify-setup.md`](docs/spotify-setup.md) for prerequisites, binary discovery, reset pairing and troubleshooting.

## Skins

Clarity is the built-in native skin and is always available. Declarative skins are installed from a directory or ZIP in the skin library. They contain JSON layouts, CSS and local assets; no skin JavaScript is executed.

Development skins live in [`skins-dev/`](skins-dev/) and are discovered automatically when Cadence runs from the repository root. The included **Afterglow** skin demonstrates a different layout, theme files and the Shadow DOM runtime.

```bash
./bin/cadence --skins-dev ./skins-dev   # load development skins
./bin/cadence --safe-mode               # built-in skin only, session-only
./bin/cadence --demo                    # simulated player, no Spotify engine
```

See [`docs/skin-format.md`](docs/skin-format.md) and [`docs/skin-authoring.md`](docs/skin-authoring.md) before creating a skin.

### Wayland and tiling compositors

Cadence requests a 420×220 window, but tiling compositors such as Sway and i3 decide the final geometry. Add a floating window rule for the app:

```text
for_window [app_id="io.github.ivandec0.cadence"] floating enable
for_window [app_id="io.github.ivandec0.cadence"] resize set 420 220
```

The Library panel expands the same window to 460 px. The custom titlebar provides the drag region; window controls are marked as non-draggable.

## Repository layout

```text
.
├── main.go                          # Wails entry point, window options and service wiring
├── Taskfile.yml                     # Developer commands
├── build/                           # Wails metadata and Linux build tasks
├── frontend/                        # Svelte 5 + TypeScript UI
├── internal/
│   ├── golibrespot/                 # Managed engine, REST client, provider and library service
│   ├── player/                      # Provider-independent player service
│   ├── skins/                       # Skin validation, installation and runtime support
│   ├── settings/                    # Persisted configuration
│   ├── artcache/                    # Album-art proxying and caching
│   └── platform/                    # GTK4 window sizing helper
├── skins-dev/                       # Development skins, including Afterglow
├── documentacion-cliente-musical-skins.md
├── plan/
│   └── nombre_plan.md
├── docs/
│   ├── development.md
│   ├── skin-format.md
│   ├── skin-authoring.md
│   ├── spotify-setup.md
│   └── archive/
├── .github/workflows/ci.yml         # Arch Linux CI skeleton
├── LICENSE                          # MIT (code)
└── LICENSE-ASSETS                   # CC BY-SA 4.0 (original skin assets)
```

## License

- **Cadence code:** MIT. See [`LICENSE`](LICENSE).
- **Original skin assets and visual documentation:** CC BY-SA 4.0. See [`LICENSE-ASSETS`](LICENSE-ASSETS).
- **go-librespot:** GPL-3.0. It is a separate process and is not linked into Cadence. This project is intended for private personal use, not distribution.

Third-party trademarks, fonts and assets remain the property of their respective owners.

## Disclaimer

Spotify, Winamp and cPro are trademarks of their respective owners. Cadence is not affiliated with Spotify, Nullsoft, Winamp or the cPro authors.

Cadence uses `go-librespot`, an unofficial open-source Spotify client. It relies on reverse-engineered Spotify protocols, may stop working at any time, and may be considered outside Spotify's terms. Use it only with your own Premium account and at your own risk. The default Clarity skin is an original design; downloaded Winamp skins and copyrighted recreations are not part of this repository.
