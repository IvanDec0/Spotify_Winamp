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
- **Vanta** — the fixed cockpit is `192px` tall under a `28px` titlebar in a
  `220px` window, so the bottom `2px` of the utilities row is clipped. The
  telemetry display also leaves a large empty region because the inline
  visualizer is only a `56px` strip at the bottom.
- **Nightstar** — no layout defect found in the review; its only device picker
  and settings entry are in the core and titlebar respectively. Leave it
  unchanged.

## Tasks

### S1 — Remove duplicated controls

- [x] Lumina `layout.json`: remove the bottom `device-picker` and
      `settings-button`; keep the titlebar device chip (shows the device name)
      and titlebar settings. The deck keeps transport (shuffle/repeat built in)
      and volume.
- [x] Afterglow `layout.json`: remove the controls-row `device-picker` and
      `settings-button`; keep the titlebar device chip and settings. The row
      keeps shuffle, transport, repeat and volume.

### S2 — Fix Vanta geometry

- [x] Vanta `styles.css`: make the cockpit fit the `220px` window exactly
      (`190px` under the `28px` titlebar) so the utilities row is no longer
      clipped.
- [x] Vanta `styles.css`: grow the inline display visualizer from the bottom
      strip to a taller background layer (`84px`, `opacity: 0.55`) that fills
      the empty telemetry area, keeping the text and progress above it (the
      existing `[data-name="display"] > .cad-component { z-index: 1 }` rule
      still wins over the visualizer's `z-index: 0`) and the grid mode legible.

S1 and S2 ship in one work-unit commit:
`fix(skins): remove duplicated compact controls and fit Vanta`.

### S3 — Verification

- [x] Go tests/vet/gofmt (no Go change expected), frontend i18n check, check,
      lint, format, test, build, `task build`.
- [ ] `task package` and inspect that the AppImage still bundles all four
      skins with the updated layouts/styles. Deferred to the orchestrator; not
      run in this work unit.
- [x] `git diff --check`.
- [ ] Live visual check pending user: compact window at 100% and 200% scale,
      each skin, queue open and closed.

## Progress log

| Work unit | Scope | Verification | Commit |
|-----------|-------|--------------|--------|
| Compact skin fixes | S1: remove duplicated `device-picker`/`settings-button` from the Lumina and Afterglow control rows. S2: Vanta cockpit `192px` → `190px`; display visualizer `56px`/`0.8` → `84px`/`0.55`. | JSON parse of all three layouts; frontend i18n check, check, lint, format, test and build; Go tests and vet; `task build`; `git diff --check`. | reported in session |

AppImage packaging inspection is deferred to the orchestrator. The live visual
check remains pending the user.

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
