# Cadence

> **Working title.** A compact Arch Linux controller for Spotify Connect with installable, declarative, Winamp-inspired skins.

## Status

**M0 complete.** The Wails v3 scaffold builds, runs and opens the frameless 420×220 Cadence shell. The next milestone is M1: the demo player and the original Clarity skin.

- Design source of truth: [`documentacion-cliente-musical-skins.md`](documentacion-cliente-musical-skins.md)
- Implementation plan: [`plan/nombre_plan.md`](plan/nombre_plan.md)
- Development workflow: [`docs/development.md`](docs/development.md)
- Spotify setup: [`docs/spotify-setup.md`](docs/spotify-setup.md)

## What it is

Cadence is a desktop **controller** for Spotify, not a music player. Audio continues to play in the official Spotify client through Spotify Connect. Cadence owns authentication, playback state, queue state, device selection and the visual skin runtime; it never receives or processes audio.

The app uses a single compact window (420×220 by default) with an expandable queue. The default skin, **Clarity**, is an original design inspired by the minimal, elegant, animated cPro Pure lineage from the 2010–2015 Winamp modern-skin era. It contains no copied skin assets.

## What it is not

- Not a Spotify player or streaming client.
- Not a Web Playback SDK integration.
- Not a librespot, Spicetify or CEF wrapper.
- Not an audio extractor or audio analyzer.
- Not a native Winamp skin loader in v1.
- Not a marketplace, visual skin editor or level 3 executable-skin runtime in v1.

## Stack

- Go 1.26+ (development environment: 1.27.3)
- Wails `v3.0.0-beta.34` (pinned)
- Svelte 5.57.2
- Vite 8.3.5
- TypeScript 6.0.3
- Vitest 5.0.5, ESLint 10.13.0, Prettier 3.9.10
- Node.js 26.12.1 / npm 12.2.1

TypeScript 7 is intentionally not used because `svelte-check` 4.7.7 and `typescript-eslint` 8.72.0 are not compatible with it.

## Platform

Arch Linux x86_64 only in v1.

## Documentation

| Document | Purpose |
|---|---|
| [`documentacion-cliente-musical-skins.md`](documentacion-cliente-musical-skins.md) | Full v1 technical design |
| [`plan/nombre_plan.md`](plan/nombre_plan.md) | Actionable implementation plan and test gates |
| [`docs/development.md`](docs/development.md) | Local setup, commands and tooling notes |
| [`docs/spotify-setup.md`](docs/spotify-setup.md) | Spotify Developer app and OAuth setup |
| [`docs/archive/documentacion-cliente-musical-skins.es.md`](docs/archive/documentacion-cliente-musical-skins.es.md) | Original Spanish design document (archived) |

## Prerequisites

Install the Arch Linux development dependencies:

```bash
sudo pacman -S --needed go nodejs npm gtk4 webkitgtk-6.0 libsoup3 base-devel git pkgconf
```

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

Run the application in development mode:

```bash
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

Packaging tasks are scaffolded in M0 and will be validated in M4.

### Wayland and tiling compositors

Cadence requests a 420×220 window, but tiling compositors such as Sway and i3 decide the final geometry. Add a floating window rule for the app:

```text
for_window [app_id="io.github.ivandec0.cadence"] floating enable
for_window [app_id="io.github.ivandec0.cadence"] resize set 420 220
```

The custom titlebar provides the drag region; window controls are marked as non-draggable.

## Repository layout

```text
.
├── main.go                          # Wails entry point and window options
├── Taskfile.yml                     # Developer commands
├── build/                           # Wails metadata and Linux build tasks
├── frontend/                        # Svelte 5 + TypeScript UI
├── documentacion-cliente-musical-skins.md
├── plan/
│   └── nombre_plan.md
├── docs/
│   ├── development.md
│   ├── spotify-setup.md
│   └── archive/
├── .github/workflows/ci.yml         # Arch Linux CI skeleton
├── LICENSE                          # MIT (code)
└── LICENSE-ASSETS                   # CC BY-SA 4.0 (original skin assets)
```

## License

- **Code:** MIT. See [`LICENSE`](LICENSE).
- **Original skin assets and visual documentation:** CC BY-SA 4.0. See [`LICENSE-ASSETS`](LICENSE-ASSETS).

Third-party trademarks, fonts and assets remain the property of their respective owners.

## Disclaimer

Spotify, Winamp and cPro are trademarks of their respective owners. Cadence is not affiliated with Spotify, Nullsoft, Winamp or the cPro authors. The default Clarity skin is an original design; downloaded Winamp skins and copyrighted recreations are not part of this repository.
