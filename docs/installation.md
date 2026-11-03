# Installation

Cadence v0.1.0 is distributed for **Arch Linux x86_64** as:

- `cadence-0.1.0-linux-x86_64.AppImage` — a self-contained application bundle
  that already contains `go-librespot`.
- `cadence-0.1.0-linux-x86_64.tar.gz` — Cadence, `go-librespot`, the desktop
  entry, licenses, notices and documentation.
- `cadence-0.1.0-SHA256SUMS` — checksums for both artifacts.

The AppImage and tarball are built from the repository with `task package`.

## Requirements

- Arch Linux x86_64.
- Spotify **Premium** account.
- PipeWire with `pipewire-pulse`, or PulseAudio.
- For the tarball: `gtk4`, `webkitgtk-6.0`, `libsoup3`, `libogg`, `libvorbis`,
  `flac`, `mpg123`, `alsa-lib` and `libpulse`.
- For the AppImage: the audio libraries used by the engine (`libogg`,
  `libvorbis`, `flac`, `mpg123`, `alsa-lib`, `libpulse`). GTK/WebKitGTK are
  bundled inside the AppImage.

```bash
sudo pacman -S --needed libogg libvorbis flac mpg123 alsa-lib libpulse
```

## Verify the download

```bash
sha256sum -c cadence-0.1.0-SHA256SUMS
```

## AppImage

```bash
chmod +x cadence-0.1.0-linux-x86_64.AppImage
./cadence-0.1.0-linux-x86_64.AppImage
```

If FUSE is unavailable, run it in extraction mode:

```bash
./cadence-0.1.0-linux-x86_64.AppImage --appimage-extract-and-run
```

The AppImage includes `go-librespot` in its bundled `usr/bin`, so no separate
engine installation is needed.

## Tarball

```bash
tar -xzf cadence-0.1.0-linux-x86_64.tar.gz
cd cadence-0.1.0
./cadence
```

`go-librespot` is next to the `cadence` binary and is discovered automatically.
To install it system-wide for the current user:

```bash
mkdir -p ~/.local/opt/cadence ~/.local/bin
cp -R cadence-0.1.0/. ~/.local/opt/cadence/
ln -sf ~/.local/opt/cadence/cadence ~/.local/bin/cadence
```

Install the desktop entry and icon if your launcher needs them:

```bash
install -Dm644 cadence.desktop ~/.local/share/applications/cadence.desktop
install -Dm644 cadence.png ~/.local/share/icons/hicolor/512x512/apps/cadence.png
```

## Skins included in the release

The AppImage and tarball include the development skins **Lumina** (four glossy themes with animated chrome) and **Afterglow** as a second layout example. They are discovered from the bundled `skins/` directory and appear in the skin library, where you can activate them and switch themes. To install additional skins, use **Install .zip…** or **Install folder…**.

## First run and pairing

1. Start Cadence without `--demo`.
2. Click the **LOCAL** badge in the titlebar to open the Library.
3. If this is the first run, Cadence shows a Spotify pairing code.
4. Choose **Open pairing page**, sign in with a Premium account and approve
   **Cadence**.
5. Browse playlists, albums, artists or Liked Songs and start playback.

Cadence plays audio locally through the engine; the official Spotify desktop
client is not required.

## Files and state

| Purpose | Path |
|---|---|
| UI preferences | `$XDG_CONFIG_HOME/cadence/config.json` and webview local storage |
| Engine credentials/config | `$XDG_CONFIG_HOME/cadence/audio/` |
| Engine log | `$XDG_CACHE_HOME/cadence/logs/go-librespot.log` |
| Installed skins | `$XDG_DATA_HOME/cadence/skins/` |

Engine credentials are owner-only. The frontend never receives them.
**Reset pairing** in the Library footer removes the engine credentials and
starts a new device-pairing flow.

## Demo mode

You can run the interface without Spotify or the engine:

```bash
./cadence --demo
```

## Updating

Replace the old AppImage or extract the new tarball over the previous directory.
Your settings and engine pairing live under `$XDG_CONFIG_HOME/cadence` and are
preserved. Run `task package` from the repository to build new artifacts.

## Uninstalling

```bash
rm -rf ~/.local/opt/cadence
rm -f ~/.local/bin/cadence
rm -f ~/.local/share/applications/cadence.desktop
```

Remove user data only if you also want to delete the pairing and preferences:

```bash
rm -rf ~/.config/cadence ~/.cache/cadence ~/.local/share/cadence
```

## Troubleshooting

See [`spotify-setup.md`](spotify-setup.md) for engine discovery, pairing and
audio-output problems.
