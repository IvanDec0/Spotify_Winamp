# Spotify audio engine setup

Cadence is a real Spotify player for a private, local setup. Audio is decoded
and played by **go-librespot**, a GPL-3.0 open-source Spotify Connect client.
Cadence does not use the official Web Playback SDK, does not use DRM, and does
not require Spotify Desktop to be running.

This is not an official Spotify client. It uses reverse-engineered Spotify
protocols, so it may stop working after Spotify changes or be considered
outside Spotify's terms. Use it only with your own Premium account and at your
own risk.

## Requirements

- Spotify **Premium** account.
- `go-librespot` built for Linux x86_64.
- PulseAudio or PipeWire (`pipewire-pulse`) for audio output.
- CGO build libraries if you build the engine yourself:
  `libogg`, `libvorbis`, `flac`, `mpg123`, `alsa-lib` and `libpulse`.

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

The engine is a separate process. It is not linked into Cadence.

## Pair the device

1. Start Cadence without `--demo`.
2. Click **Browse** in the compact titlebar.
3. If the engine is not paired, Browse shows a pairing code.
4. Choose **Open pairing page**, sign in to Spotify, and approve the device.
5. Cadence updates automatically. The local device appears as **Cadence**.

The go-librespot credentials are stored by the engine under:

```text
$XDG_CONFIG_HOME/cadence/audio/state.json
```

The directory is owner-only. Cadence does not receive Spotify playback
credentials. **Reset pairing** removes the engine state and starts a new pairing
flow.

## Library and playback

The Browse window is the Spotify account/library surface:

- browse playlists, saved albums, followed artists, Liked Songs and Your Episodes;
- open an entity page and play any track;
- play an entire playlist, album, artist or Liked Songs;
- search the Spotify catalog for tracks, albums, artists, playlists, shows and episodes;
- start a Spotify radio station from a track, album, artist or playlist;
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
| `server.image_size` | Cover-art source variant: `default`, `small`, `large` or `xlarge`. Cadence writes `default` unless changed; Phase 4 selects the variant after local resizing exists. |

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
