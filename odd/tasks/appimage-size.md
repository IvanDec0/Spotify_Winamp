# AppImage size optimization

> **Status:** Complete — final artifacts rebuilt and verified; live GTK smoke
> remains pending the user.
> **Branch:** `feat/cadence-v2`
> **Scope:** reduce the v0.2.0 AppImage without removing runtime features.

## Baseline (measured 2026-11-05)

- AppImage: **171 MB**.
- AppDir: **484 MB**.
- Duplicate WebKit stacks: `webkitgtk-6.0` (GTK4, used by Cadence) and
  `webkit2gtk-4.1` (GTK3, unused), each with its own JavaScriptCore, helpers,
  typelibs and GTK3 dependencies.
- Wails `v3.0.0-beta.34` copies every `WebKitWebProcess`/`WebKitNetworkProcess`
  helper it finds under `/usr/lib*`, so both WebKit majors end up in the AppDir;
  linuxdeploy then pulls the whole GTK3/WebKit4.1 dependency stack.
- `bin/go-librespot` is built without `-s -w`: 26 MB with debug info.
- The AppImage docs copy includes `docs/screenshots`: 2.4 MB.

## Tasks

### O1 — Prune the unused GTK3/WebKit4.1 stack

- [x] Remove the AppDir before `wails3 generate appimage` so stale WebKit
      generations cannot leak into a later package. After generation, remove
      `usr/lib/webkit2gtk-4.1`, `usr/lib/girepository-1.0`,
      `usr/lib/gtk-3.0` and the `libwebkit2gtk-4.1`,
      `libjavascriptcoregtk-4.1`, `libgtk-3` and `libgdk-3` libraries before
      the final linuxdeploy pack. Keep the GTK4/WebKit6 stack untouched.
- [x] Omit `docs/screenshots` from the AppImage docs copy.
- Result: AppDir **323 MB**, AppImage **110 MB** (with the stripped engine and
  without the screenshots).

### O2 — Strip the audio engine

- [x] Add `-s -w` to the `go build` ldflags in
      `scripts/build-go-librespot.sh` while keeping the injected engine version.
- [x] Rebuild `bin/go-librespot`: **18 MB** instead of 26 MB, stripped ELF;
      `--help` still works.

### O3 — Rebuild and verify

- [x] `task audio:build`, `task package`.
- [x] Verify AppImage size, `sha256sum -c`, `--version` from AppImage and
      tarball, no `webkit2gtk-4.1`/`gtk-3` files in the AppImage, the engine is
      stripped, and the docs no longer contain screenshots.
- [x] `ldd` over the extracted `usr/bin/cadence` and WebKit helpers with the
      AppDir library path: no `not found`.
- [x] Full gates: Go tests/vet/gofmt, frontend i18n/check/lint/format/test/build,
      `task build`, `git diff --check`.
- [ ] Live GTK smoke pending user: open the AppImage and confirm the UI,
      WebKit content, skins and audio engine still work.

## Progress log

| Work unit | Changes | Verification | Commit |
|---|---|---|---|
| Prune unused WebKit/GTK3 stack | `build/linux/Taskfile.yml` starts from a clean AppDir and removes `usr/lib/webkit2gtk-4.1`, `usr/lib/girepository-1.0`, `usr/lib/gtk-3.0`, the 4.1/GTK3 libraries and the AppImage docs screenshots before the final linuxdeploy pack. | AppDir 484 → 323 MB; AppImage 171 → 110 MB; no `webkit2gtk-4.1`/`gtk-3` files; `ldd` over `cadence` and all WebKit helpers reports no missing libraries. | `6f5ba1f` |
| Strip the audio engine | `scripts/build-go-librespot.sh` adds `-s -w`; engine rebuilt. | 26 → 18 MB; stripped ELF; `--help` exit 0. | `6f5ba1f` |
| Final verification | `task package`, checksums, versions, full Go/frontend/build gates. | `sha256sum -c` OK; AppImage **110 MB** and tarball **16 MB**; both report `0.2.0`; no `webkit2gtk-4.1`/`gtk-3`; engine stripped; docs screenshots absent; icon/desktop present; no missing libraries; 239 frontend tests; Go tests/vet/gofmt; `task build`; `git diff --check`. | `6f5ba1f` |
| Final release packaging | Rebuilt at the release commit after the tracker update. | AppImage/tarball/checksums rebuilt; `--version` reports `0.2.0`; live GTK smoke pending user. | this commit |

## Acceptance

- The AppImage keeps the GTK4/WebKit6 runtime and loses only the unused
  GTK3/WebKit4.1 stack.
- The engine remains functional and reports the pinned version.
- Size is materially lower than the 171 MB baseline.
- No source, binding, i18n or packaging regression.

## Known limitations

- WebKitGTK 6.0 remains the dominant payload (~140 MB uncompressed); the
  AppImage cannot become tiny while staying self-contained.
- `xz` is not available in the bundled `mksquashfs`; only zstd is supported.
  zstd level 22 measured 118 MB versus 119 MB at the default level, so it is
  not worth the slower packing.
