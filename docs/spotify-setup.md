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
2. Open the provider badge in the titlebar (**CONNECT** / **DEMO**).
3. If the engine is not paired, Cadence shows a pairing code.
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

The Library panel is the Spotify account/library surface:

- browse playlists, saved albums and followed artists;
- open a context and play any track;
- play an entire playlist, album, artist or Liked Songs;
- search the Spotify catalog for tracks, albums, artists and playlists;
- start a Spotify radio station from a track, album, artist or playlist;
- add tracks to the local play queue;
- queue expansion and playback controls use the local engine.

The library, search and radio endpoints are provided by go-librespot's internal
Spotify session, not the rate-limited public Web API. The local engine patch in
`build/patches/go-librespot-search-radio.patch` adds the search and radio
endpoints. Audio output uses the configured PulseAudio device (`default` in v1).

## Engine version and options

Cadence generates `$XDG_CONFIG_HOME/cadence/audio/config.yml` on every start;
manual edits to that file are overwritten.

| Option | Value and purpose |
|---|---|
| `device_is_group` | `false` by default. When enabled, Spotify can show a group icon for Cadence; it does not enable Spotify Jam. Jam participation uses the engine's Connect/Jam mode independently, and live Jam behavior is unverified. |
| `accesspoint_ports` | Engine option not exposed by Cadence yet. It is not written to the generated file; `prefer_firewall_friendly_ports: true` is active instead. |
| `server.image_size` | Cover-art source variant: `default`, `small`, `large` or `xlarge`. Cadence currently writes `default`; Phase 4 selects the variant after local resizing exists. |

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
| No pairing code | Restart the engine from the Library panel; check the engine log |
| Pairing code expired | Select **Reset pairing** and use the new code |
| No sound | Check that PipeWire/PulseAudio is running and the default sink is correct |
| Engine exits immediately | Run `bin/go-librespot --config_dir ~/.config/cadence/audio` manually to see its output |
| `go-librespot is not installed` | Build the pinned engine with `task audio:build` |

The engine log is written to:

```text
$XDG_CACHE_HOME/cadence/logs/go-librespot.log
```
