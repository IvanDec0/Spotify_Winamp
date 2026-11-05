# Third-party notices

Cadence includes or distributes the following third-party components.

## go-librespot

- **Project:** [go-librespot](https://github.com/devgianlu/go-librespot)
- **Pinned revision:** `v0.10.6-0.20261104085532-16b65bb4cd2b`
- **License:** GNU General Public License v3.0
- **Role:** separate local Spotify audio engine managed by Cadence

Cadence does not link go-librespot into its own binary. The engine is shipped as
an adjacent executable in the tarball and inside the AppImage. Its complete
license text is in [`LICENSES/go-librespot-GPL-3.0.txt`](LICENSES/go-librespot-GPL-3.0.txt).

The corresponding source is available from the upstream repository at the
pinned revision. Cadence applies four local patches, all GPL-3.0:

- [`build/patches/go-librespot-search-radio.patch`](build/patches/go-librespot-search-radio.patch)
  adds the internal search and radio endpoints.
- [`build/patches/go-librespot-search-groups.patch`](build/patches/go-librespot-search-groups.patch)
  exposes podcast shows and episodes in the search endpoint.
- [`build/patches/go-librespot-library-collection.patch`](build/patches/go-librespot-library-collection.patch)
  adds generic collection write and contains routes.
- [`build/patches/go-librespot-http-proxy.patch`](build/patches/go-librespot-http-proxy.patch)
  routes proxy-aware access-point dialing.

This project is intended for private personal use; if you redistribute the
engine, comply with the GPL-3.0, including source availability.

## GTK, WebKitGTK and runtime libraries

The AppImage bundles GTK, WebKitGTK and their runtime dependencies. Those
libraries remain under their respective licenses and copyrights. The tarball
uses the GTK/WebKitGTK and audio libraries provided by the system.

## Fonts and skin assets

- Inter and JetBrains Mono are distributed under the SIL Open Font License.
- Original Cadence skin assets are licensed under CC BY-SA 4.0; see
  [`LICENSE-ASSETS`](LICENSE-ASSETS).

Third-party trademarks and assets remain the property of their respective
owners.
