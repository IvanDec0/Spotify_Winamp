# AppImage size optimization

> **Status:** In progress.
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

- [ ] After `wails3 generate appimage`, remove
      `usr/lib/webkit2gtk-4.1`, `usr/lib/girepository-1.0`,
      `usr/lib/gtk-3.0` and the `libwebkit2gtk-4.1`,
      `libjavascriptcoregtk-4.1`, `libgtk-3` and `libgdk-3` libraries before
      the final linuxdeploy pack. Keep the GTK4/WebKit6 stack untouched.
- [ ] Omit `docs/screenshots` from the AppImage docs copy.
- Expected: AppDir ~333 MB, AppImage ~119 MB.

### O2 — Strip the audio engine

- [ ] Add `-s -w` to the `go build` ldflags in
      `scripts/build-go-librespot.sh` while keeping the injected engine version.
- [ ] Rebuild `bin/go-librespot`; expect ~18 MB instead of 26 MB and a
      stripped ELF. The engine must still print its help/version.

### O3 — Rebuild and verify

- [ ] `task audio:build`, `task package`.
- [ ] Verify AppImage size, `sha256sum -c`, `--version` from AppImage and
      tarball, no `webkit2gtk-4.1`/`gtk-3` files in the AppImage, the engine is
      stripped, and the docs no longer contain screenshots.
- [ ] `ldd` over the extracted `usr/bin/cadence` and WebKit helpers with the
      AppDir library path: no `not found`.
- [ ] Full gates: Go tests/vet/gofmt, frontend i18n/check/lint/format/test/build,
      `task build`, `git diff --check`.
- [ ] Live GTK smoke pending user: open the AppImage and confirm the UI,
      WebKit content, skins and audio engine still work.

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
