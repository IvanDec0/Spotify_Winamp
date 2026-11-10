# Frontend visual polish across every Cadence surface

## Objective

Raise the visual quality of the compact player, the dedicated Browse window,
Settings and every built-in Clarity theme. The result must feel like one
product: shared tokens, shared motion language, richer surfaces and animation
that respects the reduced-motion preference. Branch: `feat/cadence-v2`.
Test runner: `npm test` (Vitest 5) from `frontend/`.

## Specs

- **S1.** Commit the existing working tree before any visual edit. The commit
  keeps the web-player startup provisioning, docs and i18n changes together as
  one work unit.
- **S2.** All six built-in themes (`graphite`, `ice`, `violet`, `amber`,
  `acid`, `crimson`) expose one richer token contract: background depth,
  panel, aurora colors, radii, shadows and motion curves. Each theme keeps its
  identity while sharing the same surface and glow behavior.
- **S3.** The compact player gains motion without regressions: an ambient
  layer behind the player, a glowing cover stage while playing, a keyed track
  transition, an orbiting play ring, staggered queue rows and an animated
  device popover/toast. Queue open/close, window sizing and keyboard behavior
  remain unchanged.
- **S4.** The Browse window shares the same visual language: brand lockup in
  the sidebar, animated active navigation, a glass topbar, view entrance
  animations, card/list hover lift, richer tabs and empty states, and a
  Now Playing bar with a playing accent line and cover equalizer.
- **S5.** Settings keeps every existing label, id, select and behavior, but
  presents them as a coherent surface: section cards with accent markers,
  custom select chevrons, switch-style checkboxes, themed range inputs,
  progress styling and a live theme swatch next to the theme picker.
- **S6.** Shared primitives (Button, Dialog, Toast, Skeleton) use the same
  motion, radius, border and glass treatment across windows.
- **S7.** `prefers-reduced-motion` and the in-app reduced-motion preference
  disable or shorten every new animation. No animation is required to
  understand content.
- **S8.** No new runtime dependency. Research covered `motion` (official
  Svelte 5 attachment API) and `svelte-motion`; the needed effects are
  expressible with Svelte transitions and CSS, so the bundle and offline
  packaging stay unchanged.
- **S9.** `npm run format`, `npm run lint`, `npm run check`, `npm run i18n:check`
  and `npm test` pass from `frontend/`.
- **S10.** The compact player gains real depth: a blurred copy of the current
  artwork (or its generated palette) sits behind the content, the cover gets a
  conic orbit, a floor glow, a live equalizer and a hover tilt, and the controls
  sit in a glass dock.
- **S11.** Both seek bars become waveforms: varied rounded bars, an accent fill
  clipped to the played portion, a shimmer while playing, a glowing playhead
  and a hover time bubble. The native range input stays as the accessible
  control and keeps every existing test contract.
- **S12.** The Browse Now Playing bar shares the artwork ambience and the
  waveform seek bar, so the two windows read as one player.
- **S13.** New motion respects the existing reduced-motion rules and adds no
  runtime dependency.

## Tasks

| ID | Specs | Route | Status | Evidence |
| --- | --- | --- | --- | --- |
| T1 | S1 | inline | done | commit `3f126b8` |
| T2 | S2, S3 | inline | done | commits `f905997`, `72b9a0b`, `6aa3517` |
| T3 | S4, S5, S6 | inline | done | commits `b8329f0`, `35cc3c4`, `7b0d92d` |
| T4 | S7, S8, S9 | inline | done | verification in L3; recorded in this commit |
| T5 | S10-S13 | inline | done | commits in L6; verification in L7 |

## Log

### L1 — original request (verbatim)

> Quiero que hagas un commit del working tree, y luego mejores la interfaz grafica, de todos los temas, quiero mas anims, mejores cosas, busca librerias de frontend para tener mejores cosas, quiero un frontend mas lindo y homogeneo, tanto del reproductor pequeño, como del grande, settings, y todo

### L2 — exploration and decisions

- The compact window renders the built-in **Clarity** skin with six themes;
  declarative skins render inside a shadow host with their own stylesheets.
  Browse reads Clarity tokens (or skin tokens) and owns its layout.
- Settings tests assert `getByLabelText("Theme")`, `"Visualizer"`, `"Motion"`
  and `"Backend"` with `fireEvent.change`, so those controls keep their
  `<select>` semantics. The polish is visual, not a control rewrite.
- Library research: `motion` (motion.dev) supports Svelte 5 through
  attachments and adds layout/view animations, but requires a new dependency
  and view-transition support is not guaranteed in the WebKitGTK webview.
  `svelte-motion` is an older Framer-style port last published in 2024.
  Svelte 5 already ships `transition:`, `animate:flip`, `Tween` and `Spring`.
  Decision: use built-ins and CSS keyframes; no new runtime dependency.
- Authorized scope: user explicitly requested the visual work, so this is a
  tracked large implementation. No unrelated features are added.

### L3 — verification

- `npm run check`: 0 errors, 0 warnings.
- `npm run lint`: clean.
- `npm run i18n:check`: clean.
- `npm run format`: all matched files use Prettier style.
- `npm test`: 46 files, 301 tests passed.
- `npm run build`: production Vite build succeeded (compact 85.66 kB JS,
  Browse 126.28 kB JS; CSS 41.60 kB and 54.53 kB before gzip).
- Browse now applies the in-app reduced-motion preference through a root
  class, not only the system media query.
- Delivery strategy: work-unit commits on `feat/cadence-v2`. No pull request
  was requested; push and PR creation remain the user's decision.

### L4 — user feedback (verbatim)

> Bien, pero parece un reproductor plano, la barra de tiempo es plana, sin anims, y es recta, quiero mas cosas, mejores cosas, busca componentes e ideas en internet

### L5 — research and implementation

- References reviewed: the CodeFronts liquid-glass audio player (blurred album
  art behind a frosted card, capsule seek, glowing thumb), FreeFrontend's CSS
  music player collection (waveform bars, view-transition morphs, liquid glass),
  Empire UI's waveform seek/audio-player guide, and the We.Inc glass podcast
  player bar. Common ideas adopted: artwork-derived ambience, a waveform
  scrubber, a luminous playhead, and a frosted control dock.
- Implemented `Waveform.svelte`: deterministic seeded bars, a muted base layer,
  an accent layer clipped to the progress percentage, a playing shimmer and a
  breathing fill. Both the compact `Progress` and the Browse `NowPlayingBar`
  use it above the native range input.
- `Progress.svelte` now renders a glass capsule, the waveform, a glowing
  playhead with a playing pulse and a hover/drag time bubble. The range input
  remains the accessible slider, so the redraw is purely visual.
- `Player.svelte` now renders the current artwork (or the generated cover
  palette) as a blurred ambience layer, adds a scrim for contrast, a conic
  cover orbit, a floor glow, a cover equalizer, a hover tilt and a glass
  control dock.
- `NowPlayingBar.svelte` mirrors the artwork ambience and waveform seek bar.
- Checked with headless Chromium screenshots of the built app at 420x220 and
  1100x700; the waveform, cover glow and glass dock are visible and the layout
  fits the 220px compact height.

### L6 — commits

- `abdd001` — `feat(player): turn the seek bar into an animated waveform`
- `3a9cb19` — `feat(player): add artwork ambience and depth to the compact player`
- `a0b2588` — `feat(browse): mirror the waveform and artwork ambience in now playing`
- The feature doc update is recorded in the next commit.

### L7 — verification after the waveform pass

- `npm run check`: 0 errors, 0 warnings.
- `npm run lint`: clean.
- `npm test`: 46 files, 301 tests passed.
- `npm run build`: production build succeeded.
- Headless screenshots: `/tmp/opencode/compact.png`, `/tmp/opencode/browse.png`
  (local validation only, not committed).


