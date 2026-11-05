# Compact player skin review

> **Status:** In progress.
> **Branch:** `feat/cadence-v2`
> **Scope:** review the four declarative skins in the compact player and fix
> the redundant controls and layout defects found in the review.

## Review findings

- **Lumina** — polished, but the titlebar already provides the device chip and
  settings button, while the bottom deck repeats both (`device-picker` and
  `settings-button`). The deck is crowded and two actions are duplicated.
- **Afterglow** — same duplicate device and settings controls in the titlebar
  and the controls row. The row is the most crowded of the four skins.
- **Vanta** — the fixed cockpit is `192px` tall under a `30px` titlebar in a
  `220px` window, so the bottom `2px` of the utilities row is clipped. The
  telemetry display also leaves a large empty region because the inline
  visualizer is only a `56px` strip at the bottom.
- **Nightstar** — no layout defect found in the review; its only device picker
  and settings entry are in the core and titlebar respectively. Leave it
  unchanged.

## Tasks

### S1 — Remove duplicated controls

- [ ] Lumina `layout.json`: remove the bottom `device-picker` and
      `settings-button`; keep the titlebar device chip (shows the device name)
      and titlebar settings. The deck keeps transport (shuffle/repeat built in)
      and volume.
- [ ] Afterglow `layout.json`: remove the controls-row `device-picker` and
      `settings-button`; keep the titlebar device chip and settings. The row
      keeps shuffle, transport, repeat and volume.
- Commit: `fix(skins): remove duplicated compact controls`

### S2 — Fix Vanta geometry

- [ ] Vanta `styles.css`: make the cockpit fit the `220px` window exactly
      (`190px` under the `30px` titlebar, or an equivalent flex rule) so the
      utilities row is no longer clipped.
- [ ] Vanta `styles.css`: grow the inline display visualizer from the bottom
      strip to a taller background layer that fills the empty telemetry area,
      keeping the text and progress above it and the grid mode legible.
- Commit: `fix(skins): fit the Vanta cockpit and display`

### S3 — Verification

- [ ] Go tests/vet/gofmt (no Go change expected), frontend i18n check, check,
      lint, format, test, build, `task build`.
- [ ] `task package` and inspect that the AppImage still bundles all four
      skins with the updated layouts/styles.
- [ ] `git diff --check`.
- [ ] Live visual check pending user: compact window at 100% and 200% scale,
      each skin, queue open and closed.

## Acceptance

- Lumina and Afterglow expose device/settings once, in the titlebar.
- Vanta's utilities row is fully visible and the display no longer looks empty.
- Nightstar is unchanged.
- No layout overflows at the `420x220` compact canvas or the expanded queue
  height.

## Known limitations

- The skin renderer is WebKit inside the native window; layout is verified by
  code/CSS inspection and existing tests, not by a headless visual harness.
- The user's live visual verification is required for aesthetic sign-off.
