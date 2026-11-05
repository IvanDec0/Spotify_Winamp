# ADR 0003 — Error codes and i18n ownership

- **Status:** Accepted
- **Date:** 2026-11-04 (Phase 7 implementation 2026-11-05)
- **Phase:** 2 (infrastructure), 7 (implemented)
- **Source:** [`../../plan/cadence-v2.md`](../../plan/cadence-v2.md) §7

## Context

Cadence has no i18n infrastructure: the frontend contains roughly 220-240
English literals, the Go side about 180, and the backend returns preformatted
English prose. Translation cannot be added reliably while user-visible strings
are embedded in code and backend errors are opaque sentences.

## Decision

- The Go backend is **not localized**. `engine.AudioState` exposes a stable
  `ErrorCode` (`engine_not_installed`, `engine_disabled`, `engine_not_running`,
  etc.) and `playback.PlayerError` exposes a stable `Code`; the raw `Error` /
  `Message` strings remain diagnostic detail. The frontend renders the matching
  `errors.*` key with the active locale and falls back to a generic headline for
  unknown codes.
- Frontend catalogs are TypeScript modules with ICU MessageFormat strings at
  `frontend/src/shared/i18n/locales/{en,es}.ts`, rendered with
  `intl-messageformat`; `Intl` handles numbers, dates and relative time.
- `en.ts` is `as const` and exports `MessageKey = keyof typeof en`; `es.ts` is
  typed `Record<MessageKey, string>`. Missing or unknown keys are a
  compile-time error, so there is no generator and no `.d.ts` build step.
- The base locale is bundled eagerly; other locales are separate chunks loaded
  with dynamic `import()`, and switching locale clears the formatter cache.
- First run reads `localStorage["cadence.locale"]`, then falls back to
  `navigator.language` (`es*` to `es`, otherwise `en`). The choice persists in
  `localStorage` and updates `document.documentElement.lang`.
- Locale changes are mirrored across the compact and Browse windows on
  `cadence.locale`; `syncLocale` applies a received catalog without persisting,
  and `initI18n()` installs the listener once per webview.
- The Phase 7 translation guard is `frontend/scripts/check-i18n.mjs`: it walks
  the Svelte compiler AST for literal text nodes and literal `title`,
  `aria-label`, `placeholder` and `alt` values, with a small brand/symbol/unit
  allowlist. `npm run i18n:check` runs in the Taskfile and CI and has no `--fix`
  mode. Key parity is compile-time and tests assert no empty catalog values.
- The compact player, declarative-skin components and Browse are fully
  translated; new code uses `t()` from day one and the guard enforces it.
  Skin-provided labels and Go skin-validation warnings remain raw English
  diagnostic detail until they gain structured codes.

## Consequences

- Engine and player error surfaces use `AudioState.ErrorCode` /
  `PlayerError.Code`; unknown errors keep a raw diagnostic detail.
- The frontend owns all user-facing text and locale switching; the Go API stays
  language-neutral.
- New UI cannot merge with hardcoded user-facing strings once the guard is
  enabled.
- Catalogs, the `MessageKey` union, the `Record<MessageKey, string>` parity
  type and the translation guard are part of the review surface. Adding a key
  is a two-file change with no generated artifacts to commit.
- Known limitation: raw Go skin-validation warnings and the live GTK/locale
  smoke test are pending.
