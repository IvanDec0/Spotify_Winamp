# Contributing to Cadence

Thanks for taking the time to improve Cadence. The project targets Arch Linux x86_64 and is built with Go, Wails v3, Svelte 5, Vite and TypeScript. Bug reports, focused fixes, documentation improvements and original skins are welcome.

## Before you start

- Read [`docs/development.md`](docs/development.md) for the toolchain, commands and troubleshooting.
- For a larger change, open an issue first so the approach can be discussed before you invest in implementation.
- Use English for code, comments, commit messages and documentation. User-facing copy belongs in the English and Spanish catalogs.

## Setup

Install the system packages, the pinned Wails CLI and Task as described in the [README](README.md#build-from-source), then build the engine and start the development loop:

```bash
task audio:build
task dev
```

`task audio:build` downloads the pinned go-librespot source, applies the local GPL-3.0 patches and builds `bin/go-librespot`. The binary is generated, not committed. Frontend dependencies are installed by the Taskfile; direct npm work runs from `frontend/`.

## Before opening a pull request

Run the full local gate:

```bash
task verify
```

`task verify` runs `task check`, `task test` and `task audio:check`. For a faster loop, `task check` plus `task test` covers the Go and frontend suites. For UI copy changes, also run:

```bash
cd frontend
npm run i18n:check
```

Update the English and Spanish catalogs together and keep both windows translated. Include tests with behavior changes, and update the affected documentation in the same pull request.

## Commits

Use [Conventional Commits](https://www.conventionalcommits.org/): `feat:`, `fix:`, `docs:`, `test:`, `refactor:`, `build:`, `chore:`. Keep each commit focused on one logical change, and keep tests and documentation with the behavior they describe. Do not add AI attribution or co-author trailers.

## Pull requests

- Keep one logical change per pull request and explain the user-visible effect.
- Include reproduction steps for fixes and the expected result.
- Update documentation and screenshots when the UI, installation flow or skin format changes. Use only screenshots that contain no personal account data.
- Do not commit credentials, API keys, engine binaries, downloaded Soloist or Chromium builds, `bin/` artifacts or `skins-local/`.
- Regenerate the Wails bindings when a service signature changes; see [`docs/development.md`](docs/development.md).
- Keep unrelated refactors out of the change and stay responsive to review comments.

## License

By contributing, you agree that your changes are licensed under the repository licenses: MIT for code, and CC BY-SA 4.0 for original visual assets and documentation illustrations. Third-party assets must keep their original license and attribution.
