# Release v0.2.0 — Phase 8

> **Status:** In progress.
> **Branch:** `feat/cadence-v2`
> **Scope:** close the v2 plan with a versioned, reproducible release.

## Tasks

### R1 — Version stamping

- [ ] Bump the version to `0.2.0` in `Taskfile.yml` (`APP_VERSION`) and
      `build/config.yml`; the release tag must be `v0.2.0`.
- [ ] Add `internal/buildinfo` (`Version`, `Commit`, `String`) and a
      `--version` flag in `main.go` that prints and exits before Wails starts.
- [ ] Inject `-X cadence/internal/buildinfo.Version` and `.Commit` in the Linux
      build flags, with a `GIT_COMMIT` task variable.
- [ ] Update the artifact/version references in README, docs/installation and
      the release workflow expectations.

### R2 — Packaging contents

- [ ] AppImage and tarball include `go-librespot` built with all four patches,
      the bundled skins, README, notices, licenses, docs and `CHANGELOG.md`.
- [ ] Add `CHANGELOG.md` with a 0.2.0 entry covering the v2 work (modular Go,
      event-driven state, catalog/podcasts, Browse window, i18n, real
      visualizer, settings, engine patches).
- [ ] Verify the AppImage's `usr/bin/go-librespot` and the tarball engine are the
      rebuilt four-patch binary.

### R3 — Docs and installation

- [ ] Update `docs/installation.md` to 0.2.0: Browse/pairing flow (the badge is
      Browse, not LOCAL), search tabs (tracks/albums/artists/playlists/shows/
      episodes), library/podcasts/settings, language selector, queue, real
      visualizer (`parec`), cache paths, four engine patches, version/checksums.
- [ ] Update README features/stack/architecture and `docs/development.md`
      (`task package`, `--version`, required `parec`).
- [ ] Add the installation requirements for `parec`/PulseAudio tools and the
      optional-ness/fallback of the monitor visualizer.

### R4 — Build and verify

- [ ] `task package` (AppImage, tarball, checksums) and inspect both artifacts.
- [ ] `./bin/cadence-0.2.0-linux-x86_64.AppImage --appimage-extract-and-run
      --version` and the extracted tarball `./cadence --version` print 0.2.0.
- [ ] `sha256sum -c cadence-0.2.0-SHA256SUMS`.
- [ ] Full gates: Go tests/race/vet/gofmt, four engine patches, bindings no
      diff, `npm run i18n:check`, frontend check/lint/format/test/build,
      `task build`, `git diff --check`.
- [ ] Clean-machine install checklist documented; live Arch VM test pending the
      user if unavailable here.
- Commit: `chore(release): prepare v0.2.0`

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
