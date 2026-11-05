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
- Skins may provide an optional `i18n` map in `manifest.json`; missing entries
  fall back to the raw label.
- The Phase 7 translation guard is `frontend/scripts/check-i18n.mjs`: it walks
  the Svelte compiler AST for literal text nodes and literal `title`,
  `aria-label`, `placeholder` and `alt` values, with a small brand/symbol/unit
  allowlist. `npm run i18n:check` runs in the Taskfile and CI and has no `--fix`
  mode. Key parity is compile-time and tests assert no empty catalog values.
- Migration is per module as modules are touched; new code uses `t()` from day
  one. The compact and declarative-skin sweep lands in Phase 7.

## Consequences

- `AudioState.Error`, `SkinCatalog.LastError`, validation warnings and pairing
  messages must migrate from prose to codes with parameters.
- The frontend owns all user-facing text and locale switching; the Go API stays
  language-neutral.
- New UI cannot merge with hardcoded user-facing strings once the guard is
  enabled.
- Catalogs, the `MessageKey` union, the `Record<MessageKey, string>` parity
  type and the ESLint rule become part of the review surface. Adding a key is
  a two-file change with no generated artifacts to commit.
