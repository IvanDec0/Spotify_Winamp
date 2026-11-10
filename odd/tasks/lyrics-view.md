# Now Playing lyrics view and emoji rendering

## Objective

Clicking the compact player cover or song title opens a dedicated Now Playing
stage with the current track, its player and time-synced lyrics that follow
playback like Spotify. Emoji in titles, artists and lyrics must render as real
glyphs instead of the current replacement character. Branch:
`feat/cadence-v2`. Test runner: `npm test` from `frontend/`; Go tests via
`go test`.

## Specs

- **S1.** The compact player cover and title are buttons that open the existing
  Browse window on a new `nowplaying` section, which hides the sidebar, search
  field, theme chip and bottom bar for an immersive stage.
- **S2.** The stage shows the large cover, title, artists, album, waveform
  progress, transport, volume and a lyrics panel. A back button returns to the
  library; the window controls remain available.
- **S3.** Lyrics come from LRCLIB through a Go client: `/api/get` first,
  `/api/search` as fallback, a bounded HTTP timeout, a descriptive
  `User-Agent`, and a seven-day on-disk cache keyed by track metadata. Missing
  lyrics and provider failures are normal UI states, not crashes.
- **S4.** The Go side parses LRC timestamps (multiple tags, centiseconds,
  milliseconds, `[offset:]`, out-of-order lines), keeps plain unsynced lyrics,
  and normalizes instrumental tracks.
- **S5.** The frontend derives the active line locally from
  `player.positionMs`; it auto-scrolls the active line to the center, respects
  the reduced-motion preference, and lets the user click a timestamped line to
  seek. No per-tick backend call is made.
- **S6.** Missing, loading, error, instrumental and plain-lyrics states have
  dedicated copy, with retry where useful and LRCLIB attribution.
- **S7.** Emoji render correctly everywhere: Twemoji Mozilla is bundled with its
  license, added through `@font-face`, and appended to the UI/mono font stacks
  in Clarity, Browse, the Now Playing stage and the declarative skin base.
- **S8.** The existing test suites, `svelte-check`, ESLint, Prettier, the i18n
  check, the Go tests and `go build ./...` pass.

## Tasks

| ID | Specs | Route | Status | Evidence |
| --- | --- | --- | --- | --- |
| T1 | S3, S4 | inline | done | `internal/lyrics`, `internal/wailsapi/lyrics`, tests |
| T2 | S1, S2, S5, S6 | inline | done | `NowPlayingView.svelte`, section wiring, `lyrics.ts` test |
| T3 | S7 | inline | done | bundled Twemoji font and font stacks |
| T4 | S8 | inline | done | verification in L4 |

## Log

### L1 — original request (verbatim)

> Quiero que agregres algo nuevo:
> Si le doy click al nombre de la cancion, o a la portada (la imagen), que me abra como otra vista, donde me aparezca la cancion, el reproductor, y la letra de la cancion, y se vaya moviendo como en spotify, busca como hacer esto, busca en internet y hacelo lindo
>
> Y un bugfix, los emojis no los toma bien, y muestra un caracter raro, fijate si podes arreglar eso

### L2 — research and decisions

- `plan/cadence-v3-roadmap.md` already specifies LRCLIB `/api/get` and
  `/api/search`, an LRC parser, an on-disk cache and a frontend active line
  derived from playback. This implementation follows that baseline.
- LRCLIB is a free, keyless lyrics API with synced and plain lyrics. The client
  uses a descriptive User-Agent as requested by the project, an eight-second
  timeout, positive/negative caching (7 days / 6 hours) and a search fallback
  when `/api/get` misses.
- The stage reuses the existing Browse window instead of adding a third native
  window: it hides Browse chrome, keeps the compact-to-Browse lifecycle and
  gives the lyrics enough room. This avoids duplicating window shutdown and
  restore logic.
- The active line is computed in the frontend from the interpolated position;
  the backend is called once per track, not per tick.
- Emoji: the system has no emoji font installed (`fc-list` has none), so the
  replacement character is a missing-glyph issue. Twemoji Mozilla (COLRv0,
  1.5 MB) is bundled with its Apache-2.0/CC-BY-4.0 license and added to every
  relevant font stack.

### L3 — implementation

- `internal/lyrics`: client, LRC parser, provider selection, cache and tests.
- `internal/wailsapi/lyrics`: Wails service returning `available`,
  `instrumental`, `synced`, `source`, `reason` and `lines`; missing lyrics are
  a result, transport errors are errors.
- `main.go`: lyrics client using `$XDG_CACHE_HOME/cadence/lyrics`; service
  registered with Wails. Bindings regenerated.
- `platform`: `nowplaying` accepted by `browseSection`.
- `NowPlayingView.svelte`: full stage with cover depth, blurred artwork,
  waveform player, transport, volume, lyrics list, active-line scrolling and
  click-to-seek. `lyrics.ts` holds the pure active-index helper and its test.
- `Player.svelte`, `App.svelte`, `NowPlayingBar.svelte`, `BrowseApp.svelte`,
  `browse.svelte.ts`, `window.ts`: click wiring, section, stage layout and
  visible nav entry.
- i18n: English and Spanish copy for the stage, states, retry and seek label.
- Emoji: `Twemoji.Mozilla.ttf` + license, `@font-face`, font stacks in
  `app.css`, `browse.css`, `skin-base.css` and the four development skins.
- The stage is visually verified in headless Chromium with a mocked synced
  lyrics result; emoji are verified with a dedicated Twemoji rendering page.

### L4 — verification

- `go test ./internal/lyrics/ ./internal/wailsapi/lyrics/ ./internal/platform/`:
  pass.
- `go build ./...`: pass.
- `npm run check`: 0 errors, 0 warnings.
- `npm run lint`: clean.
- `npm run format`: clean.
- `npm run i18n:check`: clean.
- `npm test`: 47 files, 304 tests passed.
- `npm run build`: production build succeeded.
