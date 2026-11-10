# Installation

Cadence 0.2.0 is distributed for **Arch Linux x86_64** as:

- `cadence-0.2.0-linux-x86_64.AppImage` — a self-contained bundle with the
  patched `go-librespot` engine.
- `cadence-0.2.0-linux-x86_64.tar.gz` — Cadence, the engine, the four bundled
  skins, README, CHANGELOG, notices, licenses and `docs/`.
- `cadence-0.2.0-SHA256SUMS` — SHA-256 checksums for both artifacts.

Both artifacts are built from the repository with `task package`.

## Requirements

- Arch Linux x86_64.
- A Spotify **Premium** account.
- PipeWire with `pipewire-pulse`, or PulseAudio.
- For the tarball: `gtk4`, `webkitgtk-6.0` and `libsoup3`. The AppImage bundles
  these libraries.
- Engine codec libraries: `libogg`, `libvorbis`, `flac`, `mpg123`, `alsa-lib`
  and `libpulse`.
- `parec` and `pactl` for the real visualizer. Both ship in the `libpulse`
  package on Arch. Playback works without `parec`; only the real visualizer
  needs it.

The Spotify web player is the default playback backend, but its Chromium runtime
is never part of the AppImage, the tarball or the release checksums. Cadence
downloads the right build at first launch (~120 MB, SHA-256 verified), opens the
embedded Spotify login only when no session is stored and starts the playback
helper automatically, so no manual install step is required. go-librespot is
still bundled because it serves the library, search and catalog in every backend
mode, and it remains the explicit `librespot` fallback in Settings; Spotify
Soloist is the other explicit fallback. Linux is the supported target for all
playback backends in this release.

```bash
sudo pacman -S --needed gtk4 webkitgtk-6.0 libsoup3 \
  libogg libvorbis flac mpg123 alsa-lib libpulse
```

The visualizer is Linux-only. It captures the system-wide default PulseAudio or
PipeWire monitor, so it reacts to all audio on that monitor, not just Cadence.

## Verify the download

```bash
sha256sum -c cadence-0.2.0-SHA256SUMS
chmod +x cadence-0.2.0-linux-x86_64.AppImage
./cadence-0.2.0-linux-x86_64.AppImage --appimage-extract-and-run --version
tar -xzf cadence-0.2.0-linux-x86_64.tar.gz
./cadence-0.2.0/cadence --version
```

Both version commands print `Cadence 0.2.0 (<short commit>)` and exit without
opening a window. The AppImage command uses extraction mode so it also works on
systems without FUSE.

## AppImage

```bash
chmod +x cadence-0.2.0-linux-x86_64.AppImage
./cadence-0.2.0-linux-x86_64.AppImage
```

If FUSE is unavailable, run it in extraction mode:

```bash
./cadence-0.2.0-linux-x86_64.AppImage --appimage-extract-and-run
```

The AppImage contains `go-librespot` in its bundled `usr/bin`, so no separate
engine installation is needed. The engine codec libraries are still required
from the system.

## Tarball

```bash
tar -xzf cadence-0.2.0-linux-x86_64.tar.gz
cd cadence-0.2.0
./cadence
```

`go-librespot` is next to the `cadence` binary and is discovered automatically.
To install it system-wide for the current user:

```bash
mkdir -p ~/.local/opt/cadence ~/.local/bin
cp -R . ~/.local/opt/cadence/
ln -sf ~/.local/opt/cadence/cadence ~/.local/bin/cadence
```

Install the desktop entry and icon if your launcher needs them:

```bash
install -Dm644 cadence.desktop ~/.local/share/applications/cadence.desktop
install -Dm644 cadence.png ~/.local/share/icons/hicolor/512x512/apps/cadence.png
```

## First run and pairing

1. Start Cadence without `--demo`. Spotify Soloist is already the selected
   backend, and Cadence downloads its build in the background.
2. Click **Browse** in the compact titlebar. It opens the Browse window on
   Library.
3. If the catalog engine is not paired, Browse shows a pairing code. Choose
   **Open pairing page**, or copy the code.
4. Sign in with a Premium account and approve **Cadence Library** at
   `spotify.com/pair`.
5. When the Soloist download finishes, the setup modal asks for the one-time API
   key. Choose **Generate API key** to open Spotify's dashboard, sign in, accept
   the terms, click **Generate API key**, then paste the key into the modal and
   select **Save and start**.
6. Open Spotify on your phone on the same Wi-Fi network and choose **Cadence**,
   not **Cadence Library**. Come back and select **Done** in the modal.
7. Browse updates automatically. Search the catalog, open the Library, or play
   a track.

Choosing **Later** in the modal dismisses it for this install; a small banner in
Browse and the **Spotify Soloist** card in Settings remain the way back. Cadence
does not reopen the modal on every launch.

Cadence plays audio locally through Soloist; the official Spotify desktop client
is not required.

If the catalog engine is unpaired and the pairing code is stale, use **Reset
pairing** on the Browse pairing screen and approve the new code. To reset an
installation that is already paired, stop Cadence, remove
`$XDG_CONFIG_HOME/cadence/audio/state.json`, and start Cadence again.

## The Browse window

Browse is a separate window with five sections:

| Section | What it does |
|---|---|
| **Search** | Queries the Spotify catalog. Results are grouped into Tracks, Albums, Artists, Playlists, Shows and Episodes. With no query, it shows library-based Discover suggestions. |
| **Library** | Liked Songs plus tabs for Playlists, Albums, Artists and Your Episodes. Playlists, albums and artists open entity pages; episodes play immediately. |
| **Podcasts** | Your saved episodes, a shortcut to podcast search, and show pages opened from an episode or search result. Saved shows are not listed in the library yet. |
| **Queue** | The engine's current and upcoming tracks with covers, artists and durations. It is display-only: there is no reorder control. |
| **Settings** | Engine playback and cache options, features, appearance and skins. |

Entity pages exist for playlists, albums, artists and shows. They support **Play
all**, track playback, queueing and playlist adds. Albums and shows can be saved
or removed from the library, and artists can be followed or unfollowed. Albums,
artists and playlists have a **Radio** action. Editable playlists can add or
remove tracks.

The now-playing bar in Browse matches the compact player: shuffle, previous,
play/pause, next, repeat (off, queue, one track), seek, mute, volume and the
track heart. Mute stores the current volume and restores it when unmuted.

Browse is frameless and owns its chrome: drag the topbar to move it, use the
minimize and close buttons, and drag the bottom-right grip to resize it. The
default size is 960×640 and the minimum is 720×480. Only one of the compact
player and Browse is visible at a time. Closing Browse restores the compact
player, and playback continues.

## Settings

Browse Settings is the single settings surface:

- **Audio backend**: **Spotify Soloist** is the default; **go-librespot** is the
  explicit fallback. Select **Apply backend**. The saved value applies only after
  a Cadence restart.
- **Spotify Soloist**: automatic download at first launch (the card also has
  **Install Soloist**/**Update Soloist** as a manual path), version and expiry
  countdown, start/stop/restart in Soloist mode, and the one-time guided Spotify
  developer API key. The card links to the Spotify dashboard and the official
  download page.
- **Engine playback**: bitrate, crossfade, normalisation, pregain and album
  gain. These controls are disabled in Soloist mode.
- **Audio cache**: enable the disk cache, choose a size limit, and see the
  resolved cache directory. The cache is disabled in Soloist mode.
- **Features**: FLAC, MPRIS, speaker-group advertising and artwork image size.
  FLAC, MPRIS and speaker-group advertising are disabled in Soloist mode.
- **Appearance**: language, theme, visualizer mode and motion.
- **Skins**: install a ZIP or folder, activate, uninstall, reset, disable
  third-party skins, and see the safe-mode notice.

Engine changes apply only after an explicit **Restart engine** confirmation.
Appearance changes apply immediately. The language selector switches English
and Spanish in both windows and persists across restarts.

## Visualizer

The compact player and Browse both draw a visualizer. On Linux, while playback
is active, Cadence captures the system-wide default monitor with:

```text
parec --format=s16le --rate=44100 --channels=1 --latency-msec=20 --device=@DEFAULT_MONITOR@
```

The samples become 24 log-spaced bands, RMS level and a waveform. If `parec` or
the default monitor is unavailable, the frontend falls back to its simulated
animation. Playback is never blocked by a missing visualizer dependency.

## Files and state

| Purpose | Path |
|---|---|
| UI preferences and engine settings | `$XDG_CONFIG_HOME/cadence/config.json`; theme, visualizer, motion and language also use webview local storage |
| Engine credentials and generated config | `$XDG_CONFIG_HOME/cadence/audio/` (`state.json`, `config.yml`) |
| Engine log | `$XDG_CACHE_HOME/cadence/logs/go-librespot.log` |
| Artwork cache | `$XDG_CACHE_HOME/cadence/art` |
| Audio cache (default) | `$XDG_CACHE_HOME/cadence/audio-cache` |
| Installed skins | `$XDG_DATA_HOME/cadence/skins/` |
| Soloist builds | `$XDG_DATA_HOME/cadence/soloist/programs/<build-unix>/soloist` |
| Soloist data and accounts | `$XDG_DATA_HOME/cadence/soloist/data` |
| Soloist cache | `$XDG_CACHE_HOME/cadence/soloist/cache` |
| Soloist log | `$XDG_CACHE_HOME/cadence/logs/soloist.log` |
| Soloist API key (`0600`) | `$XDG_CONFIG_HOME/cadence/soloist/api-key` |

`config.yml` is regenerated on every engine start; manual edits are overwritten.
Engine credentials stay in `state.json` and are not sent to the frontend. The
Soloist key file is owner-only and separate from `config.json`; it is never
returned to the frontend.

The one-time Soloist setup prompt is dismissed through the webview
`localStorage` key `cadence.soloist.prompt.v1`, not a file. The compact player
and Browse share the value. Deleting the webview data makes the prompt eligible
again; saving an API key clears it.

## Demo mode

You can run the interface without Spotify or the engine:

```bash
./cadence --demo
```

## Updating

Replace the old AppImage, or extract the new tarball over the previous
installation. Settings, pairing and installed skins live under the XDG paths
above and are preserved. Run `--version` after the update to confirm `0.2.0`.

Soloist builds expire 90 days after their build timestamp, so Cadence refreshes
them automatically while `soloist` is the selected backend. At startup and every
six hours it checks the installed build; at 89 days of build age, or within 24
hours of expiry, it downloads and verifies a new build, prunes old ones (keeping
the newest and the currently running build) and restarts Soloist if it was
running. Network failures retry silently. **Update Soloist** in Browse Settings
performs the same update immediately.

A Cadence update never replaces Soloist, and no Soloist archive is shipped in the
release artifacts.

## Uninstalling

For a tarball installation:

```bash
rm -rf ~/.local/opt/cadence
rm -f ~/.local/bin/cadence
rm -f ~/.local/share/applications/cadence.desktop
rm -f ~/.local/share/icons/hicolor/512x512/apps/cadence.png
```

For an AppImage, delete the AppImage file.

These steps leave user data in place. Remove it only if you also want to delete
the pairing, preferences and installed skins:

```bash
rm -rf ~/.config/cadence ~/.cache/cadence ~/.local/share/cadence
```

To remove only the Soloist builds, data, cache and API key:

```bash
rm -rf ~/.local/share/cadence/soloist ~/.cache/cadence/soloist ~/.config/cadence/soloist
```

The `~/.cache/cadence/soloist` path may not exist when Soloist was never used;
removing a missing path is harmless.

## Clean installation checklist

Use this on a fresh Arch Linux x86_64 machine:

- [ ] Install the system dependencies and start PipeWire with
      `pipewire-pulse`, or PulseAudio.
- [ ] Download both artifacts and `cadence-0.2.0-SHA256SUMS`; run
      `sha256sum -c cadence-0.2.0-SHA256SUMS`.
- [ ] Run `--version` from the AppImage and from the tarball; both print
      `Cadence 0.2.0 (<short commit>)` without opening a window.
- [ ] Start Cadence without `--demo`, click **Browse**, and approve the
      **Cadence Library** catalog device at `spotify.com/pair`.
- [ ] Wait for the Soloist download, choose **Generate API key**, generate the
      key at
      `https://developer.spotify.com/dashboard/soloist`, and paste it into the modal.
- [ ] Open Spotify on the same Wi-Fi network, choose **Cadence**, approve the
      pairing, and play a track. Confirm audio through the default sink.
- [ ] Optional fallback for accounts that work with go-librespot: in Settings,
      apply the **go-librespot** backend and restart Cadence. Automatic Soloist
      download and updates stop in that mode.
- [ ] Open Browse, drag it by the topbar, minimize it, resize it from the
      bottom-right grip, then close it. The compact player returns and playback
      continues.
- [ ] Switch between English and Spanish in Settings, confirm both windows
      change, and restart Cadence to confirm the choice persisted.
- [ ] Open Queue and confirm the current track, upcoming tracks, covers,
      artists and durations.
- [ ] Confirm the visualizer reacts to system audio. If `parec` or the monitor
      is missing, confirm the simulated animation appears instead.
- [ ] Confirm the files in `$XDG_CONFIG_HOME/cadence`,
      `$XDG_CACHE_HOME/cadence` and `$XDG_DATA_HOME/cadence` match the table
      above.
- [ ] Uninstall the app and confirm `$XDG_CONFIG_HOME/cadence` still contains
      the pairing and preferences.

## Troubleshooting

See [`spotify-setup.md`](spotify-setup.md) for engine discovery, pairing and
audio-output problems.
