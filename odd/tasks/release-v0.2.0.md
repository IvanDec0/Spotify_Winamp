# Release v0.2.0 — Phase 8

> **Status:** Complete — v0.2.0 artifacts built and verified locally; the live
> clean Arch VM install remains pending user verification.
> **Branch:** `feat/cadence-v2`
> **Scope:** close the v2 plan with a versioned, reproducible release.

## Tasks

### R1 — Version stamping

- [x] Bump the version to `0.2.0` in `Taskfile.yml` (`APP_VERSION`) and
      `build/config.yml`; the release tag must be `v0.2.0`.
- [x] Add `internal/buildinfo` (`Version`, `Commit`, `String`) and a
      `--version` flag in `main.go` that prints and exits before Wails starts.
- [x] Inject `-X cadence/internal/buildinfo.Version` and `.Commit` in the Linux
      build flags, with a `GIT_COMMIT` task variable.
- [x] Update the artifact/version references in README, docs/installation and
      the release workflow expectations.

### R2 — Packaging contents

- [x] AppImage and tarball include `go-librespot` built with all four patches,
      the bundled skins, README, notices, licenses, docs and `CHANGELOG.md`.
- [x] Add `CHANGELOG.md` with a 0.2.0 entry covering the v2 work (modular Go,
      event-driven state, catalog/podcasts, Browse window, i18n, real
      visualizer, settings, engine patches).
- [x] Verify the AppImage's `usr/bin/go-librespot` and the tarball engine are the
      rebuilt four-patch binary. The tarball engine is byte-identical to
      `bin/go-librespot`; linuxdeploy adjusts the AppImage copy's ELF RPATH
      (different SHA-256, same BuildID and the same patch marker count), so the
      four patches are present in both.

### R3 — Docs and installation

- [x] Update `docs/installation.md` to 0.2.0: Browse/pairing flow (the badge is
      Browse, not LOCAL), search tabs (tracks/albums/artists/playlists/shows/
      episodes), library/podcasts/settings, language selector, queue, real
      visualizer (`parec`), cache paths, four engine patches, version/checksums.
- [x] Update README features/stack/architecture and `docs/development.md`
      (`task package`, `--version`, required `parec`).
- [x] Add the installation requirements for `parec`/PulseAudio tools and the
      optional-ness/fallback of the monitor visualizer.

### R4 — Build and verify

- [x] `task package` (AppImage, tarball, checksums) and inspect both artifacts.
- [x] `./bin/cadence-0.2.0-linux-x86_64.AppImage --appimage-extract-and-run
      --version` and the extracted tarball `./cadence --version` print 0.2.0.
- [x] `sha256sum -c cadence-0.2.0-SHA256SUMS`.
- [x] Full gates: Go tests/race/vet/gofmt, four engine patches, bindings no
      diff, `npm run i18n:check`, frontend check/lint/format/test/build,
      `task build`, `git diff --check`.
- [x] Clean-machine install checklist documented; live Arch VM test pending the
      user if unavailable here.
- Commit: `chore(release): prepare v0.2.0`

## Progress log

| Work unit | Status | Verification | Commit |
|---|---|---|---|
| Version stamping: `APP_VERSION`/`info.version` 0.2.0, frontend package 0.2.0, `internal/buildinfo`, `--version` flag, `GIT_COMMIT` and `-X` ldflags | Done | `go test ./internal/buildinfo`, `task build`, `./bin/cadence --version` prints `0.2.0 (<short hash>)` | `8da22dc` |
| `CHANGELOG.md` 0.2.0 entry covering the v2 work | Done | Reviewed against `git log` and `plan/cadence-v2.md` | `8da22dc` |
| Packaging contents: AppImage `usr/share/doc/cadence` and tarball include `CHANGELOG.md` and `docs/` | Done | `task package`; both artifacts contain the docs and changelog | `8da22dc` |
| Release workflow: assert `./bin/cadence --version` matches `build/config.yml` before checksums | Done | `release.yml` diff | `8da22dc` |
| Docs: 0.2.0 installation/Browse tour/visualizer paths, README v2 architecture/features, development release flow, docs index and architecture badge fixes | Done | `rg` consistency checks, `git diff --check` | `77225f1` |
| Tracker update for R1/R2 | Done | This file | `8da22dc` |
| Packaging and artifact verification | Done | `task package` exit 0; `sha256sum -c` OK; AppImage and tarball print `Cadence 0.2.0 (<short commit>)`; tarball engine byte-identical to `bin/go-librespot`; AppImage engine has the same BuildID and patch markers; Go tests/race/vet/gofmt, `audio:check`, bindings, frontend gates, `task build` and `git diff --check` green | `5e81c81` |
| Follow-up rebuild after the AppImage icon and compact-skin fixes | Done | Icon/desktop entry installed in `usr/share`; Lumina/Afterglow duplicate controls removed; Vanta cockpit and display fixed; artifacts rebuilt and checksums re-verified | follow-up commit |

## Acceptance

- Artifacts are named 0.2.0, contain the engine and docs, and checksums verify.
- `--version` reports 0.2.0 both from the AppImage and the tarball.
- The installation docs match the actual v2 UI and paths.
- No source, binding or i18n gate regressions.

## Known limitations

- The AppImage build needs `linuxdeploy`/FUSE; use `--appimage-extract-and-run`
  when FUSE is unavailable.
- A clean Arch VM install test cannot be executed in this environment; record
  the checklist and mark it pending the user.
- The real visualizer's monitor capture needs `parec`; the app falls back to the
  simulated animation when it is missing.
