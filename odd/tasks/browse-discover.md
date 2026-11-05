# Browse Discover — empty Search recommendations

> **Status:** In progress.
> **Branch:** `feat/cadence-v2`
> **Scope:** the dedicated Browse window's Search section when the query is empty.
> **Constraint:** go-librespot exposes no recommendation endpoint. This is a
> personalized discover surface built from the user's library, local recents,
> recent searches and curated genre cards; it is not a Spotify algorithmic feed.

## Objective

Make the empty Search state feel like Spotify's browse/home surface instead of a
blank prompt: quick access to the user's favorites, recently opened entities,
library picks, podcasts, recent searches and genre categories.

## Tasks

### D1 — Store

- [ ] Add persisted `recentEntities` (max 8, deduped by URI) and
      `recentSearches` (max 6) under `cadence.browse.*`; safe parsing and
      version-tolerant storage.
- [ ] Record an entity when it is opened/played from Browse; record a search
      only on explicit submit (Enter/chip/genre), not on every debounced
      keystroke.
- [ ] Add a cached `ensureDiscover()` that loads a small library slice
      (playlists, albums, artists) plus Your Episodes for the empty state.
- [ ] Expose `openEntity` reuse for Liked Songs (`spotify:collection:tracks`)
      and a Your Episodes shortcut (Podcasts section).

### D2 — Discover UI

- [ ] Add `DiscoverView.svelte` under `src/app/browse/`:
      greeting by time of day; quick favorites (Liked Songs, Your Episodes);
      "Escuchado recientemente" from `recentEntities`; "De tu biblioteca" from
      the cached library slice; "Podcasts para vos" from Your Episodes with a
      search CTA when empty; "Explorá por género" with curated colored cards;
      "Búsquedas recientes" chips with clear.
- [ ] Genre cards set the query and run the search (music → tracks, podcasts →
      shows) and keep the search input in sync.
- [ ] Show skeletons/empty/error states; keep the token-based skin-aware
      styling; genre cards may use curated gradients.
- [ ] Integrate the empty branch in `SearchView.svelte` without changing the
      populated results behavior.

### D3 — i18n

- [ ] Add `browse.discover.*` and `browse.genre.*` keys to `en`/`es`; keep
      `i18n:check` at 0 findings. Genre names that are proper/loanwords may stay
      as-is only if they are truly language-neutral.

### D4 — Tests and verification

- [ ] Store tests: recents persistence/dedupe, explicit search recording,
      discover loading/caching, Liked/Your Episodes shortcuts.
- [ ] `DiscoverView` tests: sections render, genre click searches, recent chip
      searches, empty library/podcasts CTAs.
- [ ] `npm run i18n:check`, check/lint/format/test/build, `task build`,
      `git diff --check`.

## Acceptance

- Empty Search shows Discover content, never a blank panel.
- The content is real: no invented recommendation data.
- Recent entities/searches persist across Browse window closes.
- The search input and tabs stay in sync with genre/chip clicks.
- Light and dark skin themes remain readable; no hardcoded dark-only colors.

## Known limitations

- No Spotify algorithmic recommendations; the library/recents are the signal.
- Saved-shows listing is still unavailable; Podcasts uses Your Episodes.
