# ADR 0003 — Error codes and i18n ownership

- **Status:** Proposed
- **Date:** 2026-11-04
- **Phase:** 2 (infrastructure), 7 (final sweep)
- **Source:** [`../../plan/cadence-v2.md`](../../plan/cadence-v2.md) §7

## Context

Cadence has no i18n infrastructure: the frontend contains roughly 220-240
English literals, the Go side about 180, and the backend returns preformatted
English prose. Translation cannot be added reliably while user-visible strings
are embedded in code and backend errors are opaque sentences.

## Decision

- The Go backend is **not localized**. It returns stable `{code, params}` error
  values; the frontend renders `errors.<code>` with the active locale.
- Frontend catalogs use ICU MessageFormat JSON at
  `frontend/src/shared/i18n/locales/{en,es}.json`, rendered with
  `intl-messageformat`; `Intl` handles numbers, dates and relative time.
- Typed keys are generated from `en.json` into `messages.d.ts`, so a missing key
  is a TypeScript error.
- The active locale is loaded eagerly; other locales use dynamic `import()`.
  First run uses `navigator.language` matched against available locales, then
  the preference persists in settings and updates `document.lang`.
- Skins may provide an optional `i18n` map in `manifest.json`; missing entries
  fall back to the raw label.
- An ESLint rule guards literal `aria-label`, `title`, `placeholder`, `alt` and
  text nodes in `.svelte` files. A test asserts key parity and no empty values.
- Migration is per module as modules are touched; new code uses `t()` from day
  one. The final sweep lands in Phase 7.

## Consequences

- `AudioState.Error`, `SkinCatalog.LastError`, validation warnings and pairing
  messages must migrate from prose to codes with parameters.
- The frontend owns all user-facing text and locale switching; the Go API stays
  language-neutral.
- New UI cannot merge with hardcoded user-facing strings once the guard is
  enabled.
- Catalogs, the key generator, the parity test and the ESLint rule become part
  of the build and review surface.
