# Soloist onboarding, default backend and auto-updates

## Objective

Make Spotify Soloist the default playback backend and remove the manual setup
from the normal path: download the right build automatically, prompt once for
the API key with a guided dashboard flow, validate and start it automatically,
and refresh the build before its 90-day expiry without the user managing files
or versions.

## Problem

The Soloist backend works, but the current experience still asks the user to:

- install the binary from Settings;
- create an API key in the Spotify developer dashboard and paste it;
- watch the 90-day expiry and update manually;
- choose `soloist` explicitly instead of receiving it by default.

For the accounts that need Soloist (go-librespot audio-key code 1), that is the
difference between "the app works" and "the app asks me to do Spotify
plumbing". The build must stay light: a bundled browser or a new runtime is not
acceptable.

## API key constraint

Spotify documents the key at
`https://developer.spotify.com/dashboard/soloist`: the user must be logged in,
accept the terms, and click **Generate API key**. There is no public API to
create developer apps or keys, and the dashboard is an authenticated web app.

Consequences:

- A headless browser would add hundreds of MB, require the user's Spotify
  password or session, fight CAPTCHA/2FA, and break on dashboard changes. It is
  rejected: it conflicts with the light-build requirement and is fragile.
- The lightest safe flow is a one-time guided modal: open the official
  dashboard in the system browser, paste the key once, validate and start
  automatically. Everything else (download, install, update, restart, prune)
  stays automatic.

## Scope

In:

- `settings.Default()` and normalization default to `soloist`; an explicit
  `librespot` remains available.
- Startup maintenance in the audio adapter:
  - auto-install Soloist when missing and the backend is Soloist;
  - auto-update when the installed build is at least 89 days old or expires
    within a day;
  - prune old versioned builds (keep the newest and the running one);
  - start Soloist automatically when a key exists;
  - leave it stopped when no key exists, so the onboarding modal can guide the
    user.
- Guided onboarding modal in the Browse window:
  - appears once when Soloist is the backend, installed, and no key is stored;
  - **Generate API key** opens the official dashboard and advances to a paste
    step;
  - paste + save validates the format, stores the key, starts Soloist, and
    shows the pairing hint when Spotify waits for the device;
  - **Later** dismisses the prompt for this install and leaves a small
    non-blocking banner with a Settings shortcut.
- Auto-install progress surfaced through the existing `audio:state` payload
  (`Soloist.Installing`, stage, downloaded, total); no new event names.
- Docs and tests.

Out:

- Headless browser, embedded Chromium, stored Spotify passwords, dashboard
  scraping, or any automation of the Spotify developer dashboard.
- Bundling or redistributing Soloist.
- Runtime backend hot-swap; the persisted backend still applies at startup.
- Changing the 90-day lifetime or the Spotify-side terms.

## Decisions

- **Default Soloist, explicit librespot.** New and legacy configs without a
  `playback` section resolve to Soloist. A config that explicitly says
  `librespot` is respected.
- **Backend orchestrates setup.** Auto-install, update and prune run in the
  audio adapter, not in the Browse webview, so they work even when Browse is
  closed. The frontend only renders state and asks for the key.
- **Download before asking for the key.** The binary download needs no secret.
  The modal can show install progress while the user decides.
- **One prompt, not a nag.** Dismissal is stored in `localStorage` under a
  versioned key; Settings always exposes the key card. The prompt is cleared
  when a key is saved.
- **Update at 89 days, not at expiry.** A successful update is installed while
  the old build still runs (versioned directories). If Soloist is running, it
  is restarted after the update so the new build is active before expiry.
- **Keep two builds.** Prune removes every versioned directory except the
  newest and the currently running one; after the restart, the old one is
  removed on the next maintenance pass. Stale download/staging dirs older than
  24 hours are removed too.
- **No new dependencies.** The modal uses the existing Dialog and UI
  components; the scheduler uses the existing installer.

## Tasks

### Phase A — Default, auto-install and auto-update

- [x] A1 `settings`: default `soloist`; `NormalizePlaybackBackend` maps empty
      and unknown values to `soloist`; `Default()` and the audio adapter's
      nil-section fallback follow. Keep `librespot` explicit.
- [x] A2 `soloist.Service`: add `UpdateDue(now)`, `AutoUpdate(ctx, progress)`
      and `Prune()`. `AutoUpdate` discovers, installs when missing or due
      (age >= 89 days or `DaysLeft <= 1`), reports whether the build changed,
      and prunes. `Prune` keeps the newest and the running build, removes older
      numeric dirs and stale temp dirs older than 24h.
- [x] A3 `wailsapi/audio`: replace the startup Soloist goroutine with a
      maintenance loop:
      - ensure installed/updated with progress + `audio:state` emission;
      - start when a key exists;
      - do not start without a key;
      - restart Soloist after an update when it is running;
      - retry every 6h, silently;
      - cancel the loop on shutdown.
- [x] A4 Tests: settings default/normalization; `UpdateDue` boundaries (89
      days, 1 day left); `AutoUpdate` no-op when fresh, install when missing,
      update when due, prune keeps newest/running; audio maintenance starts
      with a key, skips without one, updates and restarts.

### Phase B — Guided onboarding modal

- [x] B1 New `SoloistSetupDialog.svelte` in `frontend/src/app/browse/`:
      steps `ask` → `dashboard` → `pairing`, with:
      - headline and body explaining the one-time key;
      - **Generate API key** (opens `audio.openSoloistDashboard()`);
      - paste field, optional clipboard paste, **Save and start**;
      - inline, localized errors for `soloist_api_key_invalid` and
        `soloist_api_key_rejected`;
      - pairing hint when `needsPairing`;
      - **Later** dismisses.
      Use the existing `Dialog`; no new modal component.
- [x] B2 `BrowseApp.svelte`: show the dialog once when the backend is Soloist,
      the build is installed, no key is stored, and the dismissal flag is not
      set. Show a small banner with a **Configure** button when dismissed but
      the key is still missing. Do not show it in librespot mode.
- [x] B3 `music`/`audio` facade: reuse the existing methods; no new Wails
      surface unless a small read-only helper is unavoidable.
- [x] B4 i18n: all new copy in `en.ts` and `es.ts`; run `i18n:check`.
- [x] B5 Tests: modal visibility matrix (soloist/installed/no key, dismissed,
      librespot), dashboard open, save+start, rejected key, pairing hint,
      Later dismissal, banner.

### Phase C — Docs and verification

- [x] C1 Update `docs/spotify-setup.md`, `docs/installation.md`, ADR 0004,
      `README.md` and `docs/README.md`: Soloist default, auto-download,
      89-day update, pruning, guided key flow, and the explicit librespot
      escape hatch.
- [x] C2 Run the full checks: Go tests, `npm run check`, `npm run test`,
      `npm run lint`, `npm run i18n:check`, `task verify`.
- [x] C3 Manual E2E on the failing PC: fresh config → Soloist default →
      auto-download → modal → dashboard → paste → start → pair → play; then
      simulate an old build (89 days) and verify the background update and
      prune.

## File map

New:

- `frontend/src/app/browse/SoloistSetupDialog.svelte`
- `frontend/src/app/browse/SoloistSetupDialog.test.ts`
- `odd/tasks/soloist-onboarding.md`

Modified:

- `internal/settings/settings.go`, `settings_test.go`
- `internal/soloist/service.go`, `service_test.go`
- `internal/soloist/install.go` (prune helper if it belongs there)
- `internal/wailsapi/audio/service.go`, `service_test.go`
- `main.go` (fallback default and maintenance wiring)
- `frontend/src/app/browse/BrowseApp.svelte`, `BrowseApp.test.ts`
- `frontend/src/app/browse/settings.svelte.ts` (only if the modal needs a
  shared action)
- `frontend/src/lib/music.svelte.ts` (default normalization to `soloist`)
- `frontend/src/shared/i18n/locales/en.ts`, `es.ts`
- `docs/spotify-setup.md`, `docs/installation.md`, `docs/adr/0004-dual-audio-backend.md`,
  `README.md`, `docs/README.md`

Not changed:

- `playback.Provider`, `catalog.Playback`
- the installer's download/host policy
- the 90-day lifetime
- packaging (Soloist is still never bundled)

## Acceptance criteria

- A fresh install with no config starts in Soloist mode, downloads the right
  build for the host architecture automatically, and shows the guided key
  prompt once.
- Accepting the prompt opens the official dashboard; pasting a valid key
  stores it, starts Soloist, and reaches the pairing hint without a terminal.
- A rejected key produces one localized, actionable message in the modal, not a
  toast storm or a dead player.
- Cancelling leaves the app usable; the Settings card and a small banner remain
  the path back.
- An installed build older than 89 days is replaced in the background; the old
  directories are pruned; a running Soloist is restarted onto the new build.
- No download, update or prune work blocks app construction or the window.
- `task verify` passes; no new dependencies; no Soloist archive or key enters
  git.

## Checks

- Go: `go test ./internal/settings ./internal/soloist ./internal/wailsapi/audio`
- Full: `task verify`
- Frontend: `npm run check`, `npm run test`, `npm run lint`, `npm run i18n:check`
- Bindings: regenerate only if a Wails signature changes.
- Manual: the E2E in C3.

## Risks

- **API key automation is not possible.** The modal is the closest safe flow;
  do not add a headless browser later without revisiting the build-size and
  terms constraints.
- **Auto-download on first launch.** It is ~10 MB from Spotify and needs no
  key. A network failure must leave the app usable and retry silently.
- **Restart after update.** A running Soloist is briefly interrupted. It
  happens around day 89, before the build would stop working.
- **Default change for existing users.** A config without a `playback` section
  moves to Soloist. The explicit `librespot` value and the Settings selector
  remain the escape hatch.
- **Prune safety.** Never remove the running build. On Linux deleting a
  running binary is safe, but keeping it avoids surprising restarts.
- **Pairing cannot be fully automatic.** Soloist is a Spotify Connect device,
  so the Spotify app must select it once. The reference implementation's
  ZeroConf credential handoff did not work. The setup dialog opens Spotify,
  watches `loggedIn` and closes itself; **Done** remains a manual fallback.
- **Two webviews.** The setup dialog renders in Browse and in the compact
  player; both share the dismissal flag and the `audio:state` stream.

## Progress

- [x] Phase A: Soloist default, automatic install/update/prune and the
      maintenance loop (`f6c1045`).
- [x] Phase B: guided API-key modal, Browse banner and launch prompt in the
      compact player (`5c38292`, `5eb9b16`).
- [x] Phase C: documentation, ADR and default/URL alignment (`8d6c918`,
      `87c888d`).
- [ ] Manual E2E on the failing PC (C3), including a simulated 89-day-old
      build.

## Verification evidence

- `task verify`: passed. Go vet, Svelte check, Prettier, all Go tests, 46
  frontend test files / 280 tests, and the go-librespot patches apply cleanly.
- Focused suites: `go test ./internal/settings ./internal/soloist
  ./internal/wailsapi/audio`, `-race` on the same packages, `npm run check`,
  `npm run lint`, `npm run i18n:check`.
- New tests cover the default change, `UpdateDue` boundaries, auto-install,
  no-op when fresh, 89-day update, prune keep-set, maintenance start/restart/
  no-key paths, modal steps, rejected key, pairing hint, dismissal and the
  launch prompt.
- The API key remains a one-time guided paste; no headless browser or Spotify
  credentials are used. The dashboard link now opens the Soloist key page
  directly.

## Next step

Run C3 on the failing PC: fresh config → Soloist default → automatic download →
launch modal → dashboard → paste → start → pair → play. Then simulate an old
build (89 days) and confirm the background update, prune and restart, and the
librespot fallback after cancelling the prompt.
