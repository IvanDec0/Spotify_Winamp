# Cadence

A compact, skinnable desktop Spotify player for Arch Linux, with a dedicated Browse window and declarative, Winamp-inspired skins.

[![CI](https://github.com/IvanDec0/Spotify_Winamp/actions/workflows/ci.yml/badge.svg)](https://github.com/IvanDec0/Spotify_Winamp/actions/workflows/ci.yml)
[![Latest release](https://img.shields.io/github/v/release/IvanDec0/Spotify_Winamp?display_name=tag&sort=semver)](https://github.com/IvanDec0/Spotify_Winamp/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Platform: Arch Linux](https://img.shields.io/badge/platform-Arch%20Linux-1793d1.svg)](docs/installation.md)

![Cadence compact player running the Lumina Azure skin](docs/screenshots/lumina-azure.png)

## Overview

Cadence pairs a small, skinnable player with a dedicated Browse window for search, library, podcasts, queue and settings. It talks to Spotify through your own Premium account and keeps playback local: the default backend is Spotify's official Web Playback SDK inside a Chromium runtime that Cadence downloads and verifies at startup. The app UI never receives playback credentials; the isolated playback page gets only a short-lived access token. Only one window is visible at a time: opening Browse hides the compact player, and closing Browse restores it without interrupting playback.

## Features

- **Two surfaces, one player.** A compact skinnable player and a frameless Browse window with custom chrome, search, library, podcasts, queue and settings.
- **Web player by default.** Spotify's official Web Playback SDK runs with Widevine in a Chromium runtime downloaded at startup (~120 MB, checksum-verified). No manual engine install is required.
- **go-librespot catalog.** A pinned, patched go-librespot process serves library, search, context resolution, queue metadata and the catalog session in every backend mode. It appears in Spotify as **Cadence Library** while another backend plays audio.
- **Local audio and fallbacks.** Playback runs through separate local processes on PulseAudio or PipeWire (`pipewire-pulse`). Spotify Soloist (audio only, downloaded at runtime, one-time API key) and the `librespot` backend remain selectable in Browse Settings and apply after a restart.
- **Browse covers the account.** Playlists, saved albums, followed artists, Liked Songs, Your Episodes, entity pages, save/follow actions, playlist editing and player parity (shuffle, repeat, seek, mute, volume, queue).
- **Search and podcasts.** Tracks, albums, artists, playlists, shows and episodes, with podcast browsing and episode playback.
- **Language and demo mode.** Switch between English and Spanish at runtime; run `cadence --demo` to explore the interface without Spotify or network access.
- **Declarative skins.** JSON layouts, CSS and local assets; no skin JavaScript. Built-in Clarity skin plus the bundled Lumina, Vanta, Nightstar and Afterglow skins.
- **Real system visualizer.** Captures the default PulseAudio/PipeWire monitor through `parec`, with a simulated animation when capture is unavailable.

## Project status

Cadence 0.2.0 is an early public release.

| | |
|---|---|
| Version | 0.2.0 |
| Platform | Arch Linux x86_64 only |
| Account | Spotify Premium required |
| Stage | Early public release |

The catalog engine and the playback fallbacks use unofficial Spotify integrations. Read the [disclaimer](#disclaimer) before using them. Release history is in [`CHANGELOG.md`](CHANGELOG.md).

## Installation

Cadence is distributed as an AppImage and a tarball. Both include the patched go-librespot engine and the bundled skins.

```bash
# AppImage
chmod +x cadence-0.2.0-linux-x86_64.AppImage
./cadence-0.2.0-linux-x86_64.AppImage

# Tarball
tar -xzf cadence-0.2.0-linux-x86_64.tar.gz
./cadence-0.2.0/cadence
```

At first launch Cadence downloads the Chromium web player runtime, opens the embedded Spotify login when no session is stored, and starts the playback helper automatically. If the catalog engine is not paired, approve **Cadence Library** once at [spotify.com/pair](https://spotify.com/pair) from Browse.

System requirements:

- Arch Linux x86_64.
- A Spotify **Premium** account.
- PipeWire with `pipewire-pulse`, or PulseAudio.
- `gtk4`, `webkitgtk-6.0` and `libsoup3` for the tarball; the AppImage bundles them.
- `libogg`, `libvorbis`, `flac`, `mpg123`, `alsa-lib` and `libpulse` for the engine.
- `parec` for the real visualizer; playback works without it.

Full instructions, file paths, updating and uninstall steps: [`docs/installation.md`](docs/installation.md).

## Build from source

```bash
git clone https://github.com/IvanDec0/Spotify_Winamp.git
cd Spotify_Winamp

sudo pacman -S --needed go nodejs npm gtk4 webkitgtk-6.0 libsoup3 base-devel git pkgconf \
  libogg libvorbis flac mpg123 alsa-lib libpulse

go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.34
go install github.com/go-task/task/v3/cmd/task@v3.54.0
export PATH="$PATH:$(go env GOPATH)/bin"
```

Build the engine and start the development loop:

```bash
task audio:build   # pinned go-librespot with the local GPL-3.0 patches
task dev           # Wails and Vite with HMR
task check         # go vet, svelte-check, ESLint and Prettier
task test          # Go tests and Vitest
task package       # AppImage, tar.gz and SHA-256 checksums in bin/
```

The frontend dependencies are installed by the Taskfile. Direct npm work happens in `frontend/` with `npm ci`. The toolchain is Go 1.26+, Wails `v3.0.0-beta.34`, Svelte 5, Vite 8 and TypeScript 6; see [`docs/development.md`](docs/development.md) for the full workflow and troubleshooting.

### Repository layout

```text
.
├── main.go                  # Wails composition root and service wiring
├── Taskfile.yml             # Developer and packaging commands
├── build/                   # Wails metadata, Linux packaging and go-librespot patches
├── frontend/                # Svelte 5 + TypeScript UI
├── internal/                # Go domain services and Wails adapters
├── skins-dev/               # Bundled and development skins
├── docs/                    # Public documentation and screenshots
├── CHANGELOG.md
├── CONTRIBUTING.md
├── SECURITY.md
├── LICENSE
├── LICENSE-ASSETS
├── THIRD-PARTY-NOTICES.md
└── LICENSES/                # GPL-3.0 text for the separate go-librespot engine
```

## Architecture

```text
┌────────────────────────────── Cadence ──────────────────────────────┐
│  Svelte 5 UI  ◀── Wails services and events ──▶  Go domain services │
│                                                         │           │
│                                             playback.Provider       │
│                                                         │           │
│              ┌──────────────────┬───────────────────────┴────────┐  │
│              │ web (default)    │ soloist / librespot (fallback) │  │
│              └────────┬─────────┴───────────────┬────────────────┘  │
└───────────────────────┼─────────────────────────┼───────────────────┘
                        ▼                         ▼
        Chromium + Spotify Web Playback SDK   Spotify Soloist or
        (downloaded at startup)               go-librespot process
                        └─────────────┬───────────┘
                                      ▼
                             PulseAudio / PipeWire

go-librespot also serves the catalog in every mode:
library, search, context resolution and queue metadata.
```

- `internal/engine` supervises the go-librespot sidecar; `internal/catalog` owns library, search and context orchestration.
- `internal/webplayer` provisions the Chromium runtime and SDK helper; `internal/soloist` owns the Soloist fallback (protocol client, installer, updater, credentials and process supervisor).
- `internal/playback` exposes the provider seam; `internal/wailsapi/*` are the only Wails service adapters.
- `internal/skins` validates and loads declarative packages; `internal/platform` hosts the native Browse window on GTK4; the composition root is `main.go`.

The engine binds only to `127.0.0.1` on a random port. go-librespot credentials stay in `$XDG_CONFIG_HOME/cadence/audio/state.json`; web player OAuth tokens stay in `$XDG_CONFIG_HOME/cadence/webplayer/tokens.json` (`0600`). The frontend receives token-free state. Package graph, dependency rule and event flow: [`docs/architecture.md`](docs/architecture.md).

## Skins

Clarity is the built-in native skin and the fallback when a declarative skin cannot load. The bundled skins are **Lumina**, **Vanta**, **Nightstar** and **Afterglow**; they live in [`skins-dev/`](skins-dev/), which is also the development skin folder. A skin is a JSON layout plus CSS and local assets. No skin JavaScript is executed.

<details>
<summary><strong>Lumina</strong> — glossy media-player chrome with ambient light and four themes</summary>

![Lumina Azure](docs/screenshots/lumina-azure.png)

![Lumina Carbon](docs/screenshots/lumina-carbon.png)

An original design inspired by the Windows Media Player era: chrome surfaces, animated ambient light, a rotating album-art halo, LED status accents and four themes (Azure, Carbon, Ember, Violet).
</details>

<details>
<summary><strong>Vanta</strong> — cockpit layout with a reactor-core album art, telemetry display and slide-out queue</summary>

![Vanta Darkstar](docs/screenshots/vanta-darkstar.png)

![Vanta Bionic](docs/screenshots/vanta-bionic.png)

![Vanta Reactor](docs/screenshots/vanta-reactor.png)

Vanta changes geometry per theme: **Darkstar** uses chamfered carbon panels and an octagonal core, **Bionic** a glossy circular core and rounded glass panels, and **Reactor** a hexagonal core and slanted blade pod.
</details>

<details>
<summary><strong>Nightstar</strong> — four-blade star chassis in a transparent shaped window</summary>

![Nightstar Crimson](docs/screenshots/nightstar-crimson.png)

![Nightstar Ion](docs/screenshots/nightstar-ion.png)

![Nightstar Toxin](docs/screenshots/nightstar-toxin.png)

An original interpretation of the mid-2000s sci-fi media-player silhouette. The native window is transparent, so only the chrome star is visible over the desktop. Themes: **Crimson**, **Ion** and **Toxin**.
</details>

**Afterglow** demonstrates a different layout, theme files and the Shadow DOM runtime. Cadence discovers development skins automatically when it runs from the repository root:

```bash
./bin/cadence --skins-dev ./skins-dev
```

Read [`docs/skin-format.md`](docs/skin-format.md) for the package and security reference, or [`docs/skin-authoring.md`](docs/skin-authoring.md) for a step-by-step authoring guide.

## Documentation

| Document | Purpose |
|---|---|
| [`CHANGELOG.md`](CHANGELOG.md) | Release history |
| [`docs/architecture.md`](docs/architecture.md) | Package layout, dependency rule, event flow and backend seam |
| [`docs/development.md`](docs/development.md) | Local setup, commands and tooling notes |
| [`docs/installation.md`](docs/installation.md) | Release installation, first run, pairing and uninstall |
| [`docs/spotify-setup.md`](docs/spotify-setup.md) | Backend setup, pairing and troubleshooting |
| [`docs/skin-format.md`](docs/skin-format.md) | Skin package, layout, theme and security reference |
| [`docs/skin-authoring.md`](docs/skin-authoring.md) | Step-by-step skin authoring guide |

## Contributing

Read [`CONTRIBUTING.md`](CONTRIBUTING.md) before opening a pull request. Run `task verify` to execute the Go checks, tests and the engine patch check in one pass.

## Security

Report vulnerabilities privately through GitHub Security Advisories. See [`SECURITY.md`](SECURITY.md) for scope and supported versions.

## License

- **Cadence code:** MIT. See [`LICENSE`](LICENSE).
- **Original skin assets and documentation illustrations:** CC BY-SA 4.0. See [`LICENSE-ASSETS`](LICENSE-ASSETS).
- **go-librespot:** GPL-3.0. It is a separate process and is not linked into Cadence. See [`THIRD-PARTY-NOTICES.md`](THIRD-PARTY-NOTICES.md) and [`LICENSES/go-librespot-GPL-3.0.txt`](LICENSES/go-librespot-GPL-3.0.txt).

Third-party trademarks, fonts and assets remain the property of their respective owners.

## Disclaimer

Spotify, Winamp and cPro are trademarks of their respective owners. Cadence is not affiliated with Spotify, Nullsoft, Winamp or the cPro authors.

Cadence is not an official Spotify client. It combines Spotify's Web Playback SDK and Soloist with the unofficial go-librespot engine; those integrations depend on Spotify behavior that can change, may stop working at any time, and may be considered outside Spotify's terms. Use Cadence only with your own Premium account and at your own risk.

Screenshots may show third-party album artwork for demonstration only. All artwork remains the property of its respective owners.
