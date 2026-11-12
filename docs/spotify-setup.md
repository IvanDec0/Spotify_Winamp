# Spotify audio engine setup

Cadence is a real Spotify player that runs locally on your machine. Audio
playback is handled by the **Spotify web player** by default: Spotify's official
Web Playback SDK running with Widevine in a Chromium runtime. **Spotify Soloist**
and **go-librespot** remain explicit fallbacks. The catalog is served by
**go-librespot**, a GPL-3.0 open-source Spotify Connect client. Cadence does not
require Spotify Desktop to be running.

| Backend | Plays audio | Library, search, context, catalog session |
|---|---|---|
| `web` (default) | Spotify Web Playback SDK in the Chromium runtime | go-librespot |
| `soloist` (fallback) | Spotify Soloist | go-librespot |
| `librespot` (fallback) | go-librespot | go-librespot |

The backend is persisted in `config.json` and applied at startup. Changing it in
Settings requires a Cadence restart.

With the web backend Cadence needs no manual setup step: at startup it downloads
the Chromium runtime (~120 MB, SHA-256 verified) and opens the Spotify login in
your system browser only when no session is stored. The playback helper starts
on the first play; enable **Keep Cadence Web available** to keep it connected at
startup. The runtime lives under
`$XDG_CACHE_HOME/cadence/webplayer`; the OAuth session lives in
`$XDG_CONFIG_HOME/cadence/webplayer/tokens.json` (`0600`). Its refresh token
never reaches the app frontend or the logs, and the isolated helper page receives
only a short-lived access token.

This is not an official Spotify client. The go-librespot engine uses
reverse-engineered Spotify protocols; Soloist is Spotify's own player, but
Cadence's integration with it is unofficial. Either path may stop working after
Spotify changes and may be considered outside Spotify's terms. Use them only
with your own Premium account and at your own risk.

## Requirements

- Spotify **Premium** account.
- `go-librespot` built for Linux x86_64. It serves the catalog in every backend
  mode.
- PulseAudio or PipeWire (`pipewire-pulse`) for audio output.
- CGO build libraries if you build the engine yourself:
  `libogg`, `libvorbis`, `flac`, `mpg123`, `alsa-lib` and `libpulse`.
- HTTPS access to GitHub for the Chromium runtime and to Spotify for the first
  OAuth login. The runtime (~120 MB) is downloaded automatically; no manual
  install step is needed.

## Build the engine

Cadence looks for the engine in this order:

1. `CADENCE_GO_LIBRESPOT` environment variable.
2. `go-librespot` in `PATH`.
3. A `go-librespot` binary next to the Cadence binary.
4. `bin/go-librespot` in the repository root.

Build a pinned development version into `bin/go-librespot`:

```bash
task audio:build
```

The engine is a separate process. It is not linked into Cadence. It is used in
every backend mode: in web and Soloist modes it serves the library, search,
context and catalog session but does not play audio.

## Pair the device

The active playback backend advertises the playback device: **Cadence Web** in
the default web mode, and **Cadence** in the Soloist and `librespot` fallbacks.
The catalog engine appears separately as **Cadence Library** in web and Soloist
modes; in `librespot` mode the same go-librespot process owns both roles.

1. Start Cadence without `--demo`.
2. Click **Browse** in the compact titlebar.
3. If the catalog engine is not paired, Browse shows a pairing code.
4. Choose **Open pairing page**, sign in to Spotify, and approve the device.
5. Cadence updates automatically. The catalog device appears as **Cadence
   Library** in web and Soloist modes.
6. In the default web mode, complete the Spotify login in your system browser
   if prompted, then play a track in Cadence. **Cadence Web** starts
   automatically; choose it in another Spotify app if you want to control
   playback from there.

With Soloist, playback pairing is a separate one-time step: open Spotify on a
phone, tablet or another computer on the same Wi-Fi network and choose
**Cadence**. See [Soloist pairing](#pairing) for the guided flow.

The go-librespot credentials are stored by the engine under:

```text
$XDG_CONFIG_HOME/cadence/audio/state.json
```

The directory is owner-only. The app UI does not receive Spotify playback
credentials. **Reset pairing** removes the engine state and starts a new pairing
flow.

## Library and playback

The Browse window is the Spotify account/library surface:

- browse playlists, saved albums, followed artists, Liked Songs and Your Episodes;
- open an entity page and play any track;
- play an entire playlist, album, artist or Liked Songs;
- search the Spotify catalog for tracks, albums, artists, playlists, shows and episodes;
- start a Spotify radio station from a track, album, artist or playlist when the
  `librespot` playback backend is active;
- add tracks to the play queue;
- use the Queue section and the now-playing bar for playback state.

The library, search, podcast and radio endpoints are provided by go-librespot's
internal Spotify session, not the rate-limited public Web API. The local engine
patches add the search/radio, search-group and collection-write endpoints. Audio
output uses the configured PulseAudio device (`default`).

## Engine version and options

Cadence generates `$XDG_CONFIG_HOME/cadence/audio/config.yml` on every start;
manual edits to that file are overwritten. The playback and cache options below
are persisted in `$XDG_CONFIG_HOME/cadence/config.json` under `engine`. Changes
are validated and saved immediately but only take effect after an explicit
engine restart.

| Option | Default and purpose |
|---|---|
| `bitrate` | `320` kbps. Accepted values: `96`, `160`, `320`. |
| `crossfade_duration` | `0` ms (disabled). Range: `0`-`12000` ms. |
| `normalisation_disabled` | `false`; set to `true` to disable track/album normalisation. |
| `normalisation_use_album_gain` | `false`; use album gain instead of track gain. |
| `normalisation_pregain` | `0` dB. Range: `-20` to `20` dB. |
| `cache.enabled` | `false`; cache downloaded audio files on disk. |
| `cache.dir` | Empty in settings; Cadence resolves it to `$XDG_CACHE_HOME/cadence/audio-cache`. |
| `cache.size_limit` | `1GB`. Accepted values: empty (engine default), `0` (unlimited), `512MB`, `1GB`, `2GB`, `4GB`, `8GB`. |
| `flac_enabled` | `false`; prefer FLAC when Spotify offers it. |
| `mpris_enabled` | `false`; expose MPRIS media controls on Linux. |
| `device_is_group` | `false` by default. When enabled, Spotify can show a group icon for Cadence; it does not enable Spotify Jam. Jam participation uses the engine's Connect/Jam mode independently, and live Jam behavior is unverified. |
| `accesspoint_ports` | Engine option not exposed by Cadence yet. It is not written to the generated file; `prefer_firewall_friendly_ports: true` is active instead. |
| `server.image_size` | Cover-art source variant: `default`, `small`, `large` or `xlarge`. Cadence writes `default` unless changed in Settings. |

The engine writes its running version as its first log line
(`running go-librespot <version>`). Cadence parses it after startup and exposes it
in the audio state and the diagnostics snapshot:

```bash
./bin/cadence --diagnostics
# copy the logged http://127.0.0.1:PORT/debug/stats URL
curl -sS http://127.0.0.1:PORT/debug/stats | jq -r .engine_version
```

Without `--diagnostics`, the same line is the first entry in
`$XDG_CACHE_HOME/cadence/logs/go-librespot.log`.

## Troubleshooting

| Symptom | Fix |
|---|---|
| `Local audio engine unavailable` | Run `task audio:build` or set `CADENCE_GO_LIBRESPOT` |
| No pairing code | Retry the engine from the Browse pairing screen; check the engine log |
| Pairing code expired | Select **Reset pairing** and use the new code |
| No sound | Check that PipeWire/PulseAudio is running and the default sink is correct |
| Engine exits immediately | Run `bin/go-librespot --config_dir ~/.config/cadence/audio` manually to see its output |
| `go-librespot is not installed` | Build the pinned engine with `task audio:build` |

The engine log is written to:

```text
$XDG_CACHE_HOME/cadence/logs/go-librespot.log
```

## Spotify Soloist backend (optional fallback)

Spotify Soloist is an explicit playback fallback. It plays audio; go-librespot
keeps the library, search, context resolution, queue metadata and the catalog
session. `librespot` remains another explicit fallback in Settings for accounts
that work with it.

Soloist is never bundled with Cadence. The app downloads the newest build for the
host architecture from Spotify at startup (~10 MB), so the AppImage and tarball
stay light and the release checksums never include a Soloist archive.

### Why Soloist exists

Some Premium accounts are refused when go-librespot asks for an audio key:

```text
failed retrieving audio key: failed retrieving aes key with code 1
skipping track: Spotify refused the audio key (code 1)
```

`code 1` is a server-side refusal of the account's legacy AES key request, not a
local configuration error. Retry and backoff only help `code 2`; changing the
bitrate, clearing state, re-authenticating, changing the engine version, DNS or
CDN and toggling DJ mode do not help. The same account and machine play
correctly through Spotify's official player, Soloist. Soloist has no library or
search API, so Cadence keeps go-librespot for the catalog and uses Soloist only
for audio.

### What stays and what changes

| Area | `librespot` fallback | Soloist fallback |
|---|---|---|
| Audio playback | go-librespot | Soloist |
| Library, search, context, catalog session | go-librespot | go-librespot, unchanged |
| Spotify Connect device | `Cadence` | `Cadence` (Soloist) and `Cadence Library` (go-librespot) |
| Playback queue | go-librespot | Soloist's local queue |
| MPRIS | optional | off |
| go-librespot audio cache | optional | off |

In Soloist mode go-librespot still owns the catalog session, library, search,
context resolution and metadata. Its device is renamed `Cadence Library` so it
is clear that it is not the playback device. Soloist owns the audio path,
playback state and its own queue.

### First run: automatic install and guided key

1. Select **Spotify Soloist** in **Audio backend**, apply the change and restart
   Cadence. At startup Cadence discovers the build for the host architecture,
   downloads it, verifies it with `soloist --version`, and installs it under a
   build-numbered directory. Only Spotify hosts are accepted, and the archive is
   never redistributed. Progress appears in the audio state, the setup dialog and
   the Settings card.
2. When the build is installed and no key is stored, a setup modal opens in the
   compact player and in Browse. It is shown once per install and does not block
   startup; **Later** dismisses it immediately. If the first download failed, the
   same modal shows the error and **Retry download**.
3. **Generate API key** opens Spotify's developer dashboard in the system
   browser. Spotify requires a signed-in dashboard session, accepting the terms
   and clicking **Generate API key**. Cadence cannot create the key through an
   API and does not automate or scrape the dashboard.
4. Paste the key into the modal and select **Save and start**. Cadence validates
   it, stores it in `$XDG_CONFIG_HOME/cadence/soloist/api-key` (`0600`), starts
   Soloist and shows the one-time pairing step.
5. **Later** dismisses the modal. A small banner in Browse and the **Spotify
   Soloist** card in Settings remain the way back. The dismissal is stored in the
   webview `localStorage` key `cadence.soloist.prompt.v1`, shared by both windows,
   and is cleared when a key is saved, so Cadence does not ask on every launch.

If the key is rejected, the modal shows one localized, actionable message
(`soloist_api_key_invalid` or `soloist_api_key_rejected`) instead of a toast
storm; generate a new key and paste it again. The flow never opens a headless
browser, never asks for your Spotify password, and never reads the dashboard for
you. The API key and the one-time device selection are the only manual steps.

### Pairing

The API key authenticates Soloist against Spotify's audio service; it does not
sign a Spotify account in. The key and the account login are separate. Soloist
requires a one-time selection of **Cadence** from the Spotify app on a phone,
tablet or another computer on the same network. The Spotify app does not need to
be installed on the PC running Cadence.

1. Install Soloist and save an API key, as above. Cadence starts Soloist and
   shows the pairing step.
2. On a phone, tablet or another computer on the same Wi-Fi, open Spotify, open
   the device picker and choose **Cadence** once. Do not choose **Cadence
   Library**, which is the go-librespot catalog engine.
3. Cadence watches the Soloist state and closes the setup step as soon as
   Spotify reports the selection. Soloist stores the session, so later launches
   connect automatically.

Cadence cannot complete the selection for you: Spotify must pick the device once
from its own app. If Spotify or the Settings card still reports **Pairing
required**, choose **Cadence** again in Spotify; the device is only registered
after that selection. **Done** finishes the setup manually and **Later** closes
the dialog without finishing.

### Settings

- **Audio backend**: `web` is the default; `soloist` and `librespot` are explicit
  fallbacks. Select **Apply backend**. The saved value applies after a Cadence
  restart.
- **Spotify Soloist**: install state, version, remaining days, **Install
  Soloist**/**Update Soloist**, the API key field, and links to **Spotify
  dashboard** ([`https://developer.spotify.com/dashboard/soloist`](https://developer.spotify.com/dashboard/soloist))
  and **Download page**.
- **Start**, **Stop** and **Restart** appear only while Soloist is the selected
  backend.

The install, update and key controls are available in every backend mode, so
you can prepare Soloist before switching. Automatic install, update and pruning
run only while `soloist` is the selected backend.

The installer maps Spotify's Linux x86_64, arm64 and arm32 archives plus Apple
Silicon macOS and Windows x86_64 archives. Process supervision is Linux-only in
this release, so treat Linux as the supported target.

### Limitations in Soloist mode

- **No Spotify radio.** Radio needs a streaming-session resolver that
  go-librespot does not expose. The action is hidden in Soloist mode.
- **Queue is append-only.** `add_to_queue` accepts track URIs only; Soloist
  cannot reorder or clear its local queue.
- **`skipToURI` loses the parent context.** The Soloist `play` command accepts a
  URI but no offset, so clicking a track inside a playlist, album or artist
  plays that track standalone.
- **Artist and Liked Songs contexts are not confirmed yet.** Track, album,
  playlist and episode URIs are documented by Spotify. Artist and
  `spotify:user:<user>:collection` playback is sent as a URI but remains pending
  manual verification on real hardware; if Spotify rejects it, the action reports
  a Soloist protocol error.
- **go-librespot-only options are disabled.** Bitrate, FLAC, normalisation,
  crossfade, audio cache, MPRIS and speaker-group advertising do not apply.
  Soloist uses its own defaults, and the Settings controls are disabled with a
  notice.
- **Seek during loading may retry.** Soloist can drop a seek sent before the
  track is ready; Cadence resends it a bounded number of times.

Volume, shuffle, repeat, next, previous and pause work through Soloist.

### Automatic updates

Soloist builds expire 90 days after their build timestamp. While `soloist` is the
selected backend, Cadence runs a maintenance pass at startup and every six hours:

1. Install the build when none is installed.
2. When the installed build is at least 89 days old, or within 24 hours of
   expiry, check Spotify for a newer build, download and verify it, and install
   it under a new build-numbered directory. The old build keeps running until the
   switch.
3. Prune old builds: keep the newest build and the one currently running, and
   remove interrupted download and staging directories older than 24 hours.
4. Restart Soloist when it was running, so the new build is active before the old
   one expires. Playback is briefly interrupted at that point, around day 89,
   before the build would stop working.

A fresh build makes no network request. Network and download failures retry
silently on the next pass; the current build keeps playing. A failed first
download leaves the app usable: the setup dialog and the Settings card show a
localized error with **Retry download**. Settings always keeps the manual
**Update Soloist** action for an early refresh.

An expired build exits with code `10`; Cadence maps that to
`soloist_build_expired`, blocks playback and shows **This build has expired.
Install an update to keep playing.** The Settings card shows the remaining days
for an installed build and warns when 14 days or fewer remain. A Cadence package
upgrade never replaces Soloist, and the maintenance pass is the only automatic
update path.

### Security

- The API key is stored in `$XDG_CONFIG_HOME/cadence/soloist/api-key` with
  `0600` permissions. It is outside `config.json`, so copying or exporting
  settings does not copy the key.
- Soloist requires the key on its command line, so the key is passed through
  argv. A local process listing can see it; this is unavoidable with Soloist's
  CLI. Cadence redacts the key from Soloist log output and never logs argv.
- The key is never returned to the frontend. The UI only reports whether a key
  is stored.

### Paths and logs

| Purpose | Path |
|---|---|
| Installed builds | `$XDG_DATA_HOME/cadence/soloist/programs/<build-unix>/soloist` |
| Soloist state and accounts | `$XDG_DATA_HOME/cadence/soloist/data` |
| Soloist cache | `$XDG_CACHE_HOME/cadence/soloist/cache` |
| Soloist log | `$XDG_CACHE_HOME/cadence/logs/soloist.log` |
| API key (`0600`) | `$XDG_CONFIG_HOME/cadence/soloist/api-key` |

The data, cache and log directories are created owner-only. The log is
truncated on each start and contains no API key. The maintenance pass removes
old build directories as described in [Automatic updates](#automatic-updates).

### Reverting to go-librespot

Choose **go-librespot** in **Audio backend**, select **Apply backend**, and
restart Cadence. No library, pairing or playlist data is lost. The go-librespot
credentials and cache are still where they were; only the audio path changes.

In `librespot` mode Cadence runs no Soloist maintenance: nothing is downloaded,
updated or pruned automatically. The manual install, update and key controls stay
available in the Soloist card. Soloist's data and cache remain on disk and can be
removed separately.

### Soloist troubleshooting

| Symptom | Fix |
|---|---|
| `soloist_not_installed` | Cadence retries automatically on the next pass; use **Retry download** in the setup dialog or **Install Soloist** in Settings; check the network and any proxy |
| `soloist_api_key_missing` | Select **Configure** in the Browse banner or **Save key** in Settings; create a key in the Spotify dashboard |
| `soloist_api_key_invalid` | The value is empty or contains whitespace or control characters; paste a single-line key again |
| `soloist_api_key_rejected` | Spotify refused the key; create a new key in the dashboard and save it |
| `soloist_pairing_required` | On a phone, tablet or another computer on the same Wi-Fi, open Spotify and choose **Cadence**, not **Cadence Library** |
| `soloist_not_logged_in` | Repeat the pairing step and confirm the account is signed in |
| `soloist_build_expired` | Cadence updates in the background before expiry; select **Update Soloist** to refresh now; the old build cannot be revived |
| `soloist_download_failed` | Cadence retries silently; use **Retry download** to try immediately, and confirm HTTPS access to Spotify and that the system clock is correct |
| `soloist_start_failed` | Check `$XDG_CACHE_HOME/cadence/logs/soloist.log` for the last line |
| `soloist_protocol_error` | The installed binary or Spotify's protocol changed; update Soloist or Cadence |
| `soloist_unsupported` | The action is not available in Soloist mode, for example radio |
| `another Spotify Soloist process is already using its data directory` | Close other Cadence or Soloist instances using the same data directory; stale processes are cleaned up on the next start |

Exact start-failure lines are matched against Soloist's documented
data-directory message and the reference implementation's key-rejection
messages. A mismatch falls back to `soloist_start_failed` with the raw last log
line, so the error stays actionable even before the strings are confirmed on the
failing account.
