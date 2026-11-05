# Cadence skin format v1

Cadence skins are declarative. A skin package contains a manifest, a layout tree, CSS and local assets. The v1 runtime loads levels 1 and 2 only: visual themes and custom layouts. Skins never contain JavaScript, HTML, modules, Maki or executable code.

## Package layout

```text
afterglow/
├── manifest.json
├── layout.json
├── styles.css
├── browse.css           # optional, dedicated Browse-window styling
├── themes/              # optional, one file per declared theme
│   ├── ember.css
│   └── tide.css
└── assets/
    ├── cover.png
    ├── grain.png
    └── fonts/
        └── display.woff2
```

`manifest.json`, `layout.json` and `styles.css` must be at the package root. A ZIP archive must also place those files at its root, not inside a wrapper directory. `browse.css` is optional and must also be a root-level file when declared.

Installed skins live at:

```text
$XDG_DATA_HOME/cadence/skins/<id>/
```

The fallback location is `~/.local/share/cadence/skins/<id>/`. Settings live at `$XDG_CONFIG_HOME/cadence/config.json` (fallback `~/.config/cadence/config.json`). The built-in Clarity skin is native and cannot be uninstalled or replaced.

During development, directories under `skins-dev/` are discovered when Cadence runs from the repository root. Use `--skins-dev /path/to/dir` to point at another development directory. Development skins appear in the skin library with a **Development** badge and cannot be uninstalled from the app.

## Manifest

```json
{
  "schemaVersion": 1,
  "id": "afterglow",
  "name": "Afterglow",
  "version": "1.0.0",
  "description": "A warm development skin.",
  "author": "Cadence",
  "license": "CC BY-SA 4.0",
  "entry": {
    "layout": "layout.json",
    "styles": "styles.css"
  },
  "capabilities": [
    "player.read",
    "player.control",
    "queue.read",
    "devices.read",
    "devices.control"
  ],
  "themes": ["ember", "tide"],
  "defaultTheme": "ember",
  "visualizer": {
    "modes": ["spectrum", "radial"],
    "default": "spectrum",
    "intensity": 0.55
  },
  "window": {
    "width": 420,
    "height": 220,
    "minWidth": 380,
    "minHeight": 200,
    "queueHeight": 460,
    "resizable": true
  },
  "browse": {
    "css": "browse.css"
  }
}
```

Rules:

| Field | Rule |
|---|---|
| `schemaVersion` | Must be `1`. It is independent of the app version. |
| `id` | Lowercase slug, 1–64 characters, must match the install directory. `clarity` is reserved. |
| `name` | 1–80 characters. |
| `version` | Semantic version such as `1.0.0` or `1.2.0-beta.1`. |
| `description` | Up to 400 characters. |
| `author`, `license` | 1–100 characters each. |
| `entry` | Must be `layout.json` and `styles.css`. |
| `capabilities` | 1–5 entries from `player.read`, `player.control`, `queue.read`, `devices.read`, `devices.control`. |
| `themes` | 1–12 unique lowercase theme IDs. |
| `defaultTheme` | Must appear in `themes`. |
| `visualizer.modes` | 1–4 unique modes from `spectrum`, `radial`, `oscilloscope`, `grid`. |
| `visualizer.default` | Must appear in `modes`. |
| `visualizer.intensity` | Number from `0` to `1`. |
| `window` | Logical design size `420`×`height`, where `height` is 220–460; `minHeight` is 200–460; `queueHeight` is 300–760. Cadence starts at this size and renders it with an adaptive 125–300% UI scale. `resizable` may be `true` or `false`. |
| `browse` | Optional. `browse.css` must be a root-level `.css` filename with no path separators and cannot be `styles.css`. The declared file must exist and validates like every other stylesheet. |

Unknown fields are rejected. The complete manifest is limited to 64 KB.

Capabilities are declarations checked against the layout. For example, a skin that includes the queue must declare `queue.read`; a skin that includes transport controls must declare both `player.read` and `player.control`.

## Layout

`layout.json` is a tree of containers, registered components and declarative primitives. Unknown nodes, components and props are rejected. Skins contain no HTML or JavaScript: a primitive is a safe, built-in leaf that the stylesheet can shape.

```json
{
  "schemaVersion": 1,
  "root": {
    "type": "column",
    "name": "shell",
    "props": { "gap": 8, "padding": 10 },
    "children": [
      {
        "type": "component",
        "component": "titlebar",
        "props": { "showProvider": true, "showWindowControls": true }
      },
      {
        "type": "row",
        "name": "hero",
        "props": { "gap": 12, "align": "center" },
        "children": [
          {
            "type": "component",
            "component": "album-art",
            "props": { "size": 92, "frame": true }
          },
          {
            "type": "column",
            "props": { "gap": 4, "justify": "center" },
            "children": [
              { "type": "component", "component": "track-info" },
              { "type": "component", "component": "progress" }
            ]
          }
        ]
      },
      { "type": "component", "component": "transport-controls" },
      { "type": "component", "component": "queue" }
    ]
  }
}
```

Rules:

- The root must be a container and must contain at least one child.
- Every container must contain at least one child.
- Maximum depth: 12. Maximum nodes: 500. Maximum `props` keys per node: 12.
- `name` is an optional lowercase slug used for CSS targeting. Names must be unique.
- Every component may appear at most once.
- The `titlebar` component is required so the frameless window can be moved, minimised and closed.
- Only component props listed below are accepted. Prop values must have the documented type and range.
- Layout JSON is limited to 256 KB.

### Containers

| Type | Behaviour | Props |
|---|---|---|
| `stack` | Overlays all children in the same grid cell. Useful for a background visualizer. | `gap`, `padding` (0–64), `align` |
| `row` | Horizontal flex row. | `gap`, `padding` (0–64), `align`, `justify`, `wrap` |
| `column` | Vertical flex column. | `gap`, `padding` (0–64), `align`, `justify` |
| `grid` | CSS grid with equal columns and/or rows. | `gap`, `padding` (0–64), `columns` (1–12), `rows` (1–12), `align`, `flow` |

`align` is `start`, `center`, `end` or `stretch`. `justify` is `start`, `center`, `end`, `space-between` or `space-around`. `flow` is `row` or `column`.

### Components

| Component | Props (default) | Required capability |
|---|---|---|
| `titlebar` | `brand` ("CADENCE", 1–24 chars), `showBrand` (true), `showMark` (true), `showProvider` (true), `showDevice` (true), `showWindowControls` (true), `compact` (false) | `player.read` |
| `album-art` | `size` (88, range 48–160), `frame` (false), `reflection` (false) | `player.read` |
| `track-info` | `showAlbum` (true), `showStatus` (true), `align` (`left` or `center`) | `player.read` |
| `progress` | `showTime` (true), `showRemaining` (false) | `player.read` |
| `transport-controls` | `size` (`compact`, `regular`, `large`), `showShuffle` (true), `showRepeat` (true) | `player.read`, `player.control` |
| `volume` | `orientation` (`horizontal` or `vertical`), `showIcon` (true) | `player.read`, `player.control` |
| `shuffle` | `showLabel` (false) | `player.read`, `player.control` |
| `repeat` | `showLabel` (false) | `player.read`, `player.control` |
| `queue` | `maxItems` (20, range 1–100), `showHeader` (true), `showArt` (true), `showTime` (true) | `queue.read` |
| `visualizer` | `mode` (one of the declared modes), `layer` (`background` or `inline`), `intensity` (0–1) | `player.read` |
| `device-picker` | `compact` (false) | `devices.read`, `devices.control` |
| `settings-button` | `showLabel` (false) | none |

The queue component is hidden while the queue is collapsed. The app updates the host's `data-queue-open` attribute; base styles display `.cad-queue` only when that attribute is `"true"`. A skin does not need its own visibility logic.

### Primitives

Primitives are leaf nodes for artwork and custom controls. They can be placed anywhere a component can and are styled through their unique `name` with `[data-name="..."]`. They cannot contain children or declare a `component`.

| Type | Props | Purpose |
|---|---|---|
| `box` | none | A styleable empty element. Use it for decorative chassis layers, vents, rails, masks and clipping shapes. |
| `text` | `text` (1–120 chars) | A styleable text label. Content is escaped; it cannot inject markup. |
| `image` | `src` (1–200 chars, local asset), `alt` (0–120), `fit` (`contain` or `cover`) | A local PNG/JPEG/WebP image from the skin package. The runtime resolves `src` against the skin root. |
| `button` | `action` (see below), `text` (0–24), `label` (0–80; accessible name) | A native button. It is excluded from titlebar dragging and can drive player or app actions. |

At least one of `text` or `label` is required on a `button`. The allowed actions are:

| Action | Effect | Capability |
|---|---|---|
| `open-library` | Open the Browse window | none |
| `open-settings` | Open Browse Settings | none |
| `open-devices` | Open the playback device popover | `devices.read` |
| `toggle-queue` | Show or hide the queue | `queue.read` |
| `play-pause` | Toggle playback | `player.control` |
| `next`, `previous` | Skip tracks | `player.control` |
| `shuffle` | Toggle shuffle | `player.control` |
| `repeat` | Cycle repeat mode | `player.control` |
| `cycle-visualizer` | Cycle the visualizer mode | none |
| `cycle-theme` | Cycle the active theme | none |
| `minimise`, `close` | Window controls | none |

Images are validated when the skin is installed or activated: `src` must be a relative path inside the package, must not use `..`, and must reference a real PNG, JPEG or WebP file. SVG is not accepted from skins in v1; render original artwork to PNG instead. Test a primitive layout with the development loop before shipping it.

## Themes

Theme IDs are lowercase slugs. Cadence applies the active theme as the host attribute `data-theme`, and loads an optional `themes/<id>.css` file after `styles.css`.

Use CSS custom properties for the palette:

```css
/* themes/ember.css */
:host {
  --bg: #140d08;
  --surface: #1f1510;
  --text: #f7ede4;
  --muted: #b59f8e;
  --accent: #ffb347;
  --accent-2: #58e0c0;
}
```

Cadence's components consume these fallback variables when present:

```text
--bg --surface --surface-2 --text --muted --faint
--accent --accent-2 --line --line-strong
--font-ui --font-mono
```

Themes may also change fonts, borders, shadows, backgrounds, spacing, transitions and animations. Reduced-motion preferences are applied to the host as `data-reduced-motion="true"`; Cadence's base stylesheet reduces animation durations in that state.

## Browse window stylesheet

The Catalog opens in a separate Browse window. Its layout is app-owned, but it
inherits the active skin theme through custom properties. Cadence reads those
tokens from the skin theme and applies them to the `.cadence-browse` root:

```text
--bg --surface --surface-2 --surface-3 --panel --text --muted --faint
--line --line-strong --accent --accent-2 --danger
--font-ui --font-mono --ease-out --ease-spring
```

`styles.css` is **never** injected into the Browse window. A skin that wants to
style the Browse subtree declares an optional `browse.css`:

```json
"browse": {
  "css": "browse.css"
}
```

```css
/* browse.css */
.cadence-browse {
  background:
    radial-gradient(60% 50% at 0% 0%, color-mix(in srgb, var(--accent) 12%, transparent), transparent),
    var(--bg);
}

.cadence-browse .topbar {
  border-bottom-color: color-mix(in srgb, var(--accent) 18%, var(--line));
}
```

Rules:

- `browse.css` must exist at the package root, is limited to 1 MB and goes
  through the same CSS validation and asset checks as `styles.css`.
- Scope every rule to `.cadence-browse` (or a descendant). The file loads only
  in the dedicated Browse window.
- Use theme tokens instead of hardcoded colors so light themes (for example
  Vanta bionic) remain readable.
- Skins without `browse` remain valid and receive Cadence's default Browse
  styling plus their theme tokens.

## Shaped windows

The native window uses a transparent background, so a declarative skin can leave `.cad-runtime` transparent and clip a full-size shell to an irregular silhouette:

```css
.cad-runtime {
  background: transparent;
}

[data-name="shell"] {
  width: 100%;
  height: 100%;
  clip-path: polygon(0 0, 50% 14%, 100% 0, 86% 50%, 100% 100%, 50% 86%, 0 100%, 14% 50%);
  filter: drop-shadow(0 20px 40px rgb(0 0 0 / 60%));
}
```

Set `window.height` to the square or tall logical canvas the silhouette needs. Keep the titlebar inside the visible shape so the window can be moved. Interactive primitives remain native buttons; do not put controls outside the opaque silhouette. App-owned surfaces such as the Browse window remain rectangular and opaque; opening Browse hides the compact window instead of expanding it.

The bundled **Nightstar** skin uses this approach with a four-blade star chassis, a custom sigil image, decorative boxes, text labels and custom arm buttons. It is a concrete example of primitives and a shaped window.

## CSS rules

Compact skin CSS (`styles.css` and `themes/*.css`) is loaded inside a Shadow DOM. It cannot style the app chrome, the skin library or any element outside the skin host. The optional `browse.css` is the only skin stylesheet loaded into the Browse document, and it targets the `.cadence-browse` subtree directly.

Allowed at-rules:

```text
@charset @container @font-face @keyframes @-webkit-keyframes
@layer @media @property @starting-style @supports
```

Rejected constructs include:

- `@import` and all other at-rules.
- Remote, protocol-relative, absolute, `data:` and `javascript:` URLs.
- `url()` references outside the skin directory, including encoded `..` traversal.
- Query strings and fragments in asset URLs.
- `expression()`, `behavior`, `-moz-binding`, `image-set()` and other executable or remote-fetching functions.
- Unsupported asset types. Only PNG, JPEG, WebP and WOFF2 may be referenced.

Commit the active theme styles in `styles.css` and put palette-only rules in `themes/<id>.css`. If a declared theme file is missing, Cadence simply uses `styles.css`; the theme ID still appears in the library. Asset URLs in a theme stylesheet resolve relative to `themes/`, so reference shared assets as `../assets/file.png`.

## Runtime markup and CSS hooks

The renderer produces stable classes:

```text
.cad-runtime              root inside the shadow tree
.cad-container            every stack/row/column/grid
.cad-stack .cad-row .cad-column .cad-grid
.cad-root                 the layout root
.cad-component            every component wrapper
.cad-titlebar .cad-brand .cad-brand-mark .cad-brand-name .cad-provider
.cad-device .cad-device-name .cad-title-button .cad-window-button
.cad-cover .cad-cover-inner
.cad-track-info .cad-track-status .cad-status-dot .cad-track-title
.cad-track-artists .cad-track-album
.cad-progress .cad-progress-time
.cad-transport .cad-control .cad-play .cad-repeat-one
.cad-shuffle-button .cad-repeat-button
.cad-volume .cad-volume-button
.cad-device-picker .cad-settings-button
.cad-visualizer
.cad-queue .cad-queue-head .cad-queue-list .cad-queue-row
.cad-queue-index .cad-queue-art .cad-queue-copy .cad-queue-title
.cad-queue-artist .cad-queue-time
```

Containers and wrappers also carry `data-name` when the layout declares a name. Component wrappers carry `data-component`. Target named regions for layout-specific styling:

```css
[data-name="hero"] { flex: 1 1 auto; }
[data-name="controls"] { border-top: 1px solid var(--line); }
[data-component="queue"] { background: var(--surface); }
```

## Validation limits

| Limit | Value |
|---|---|
| Manifest | 64 KB |
| Layout | 256 KB |
| Stylesheet | 1 MB per CSS file |
| Single asset | 10 MB |
| Total skin | 50 MB uncompressed |
| Files | 500 |
| Layout depth | 12 |
| Layout nodes | 500 |
| ZIP archive | 20 MB |
| ZIP uncompressed | 100 MB |
| ZIP compression ratio | 100:1 |

ZIP installation also rejects encrypted entries, unsupported compression methods, duplicate names, case-colliding names, symlinks, device files, special permissions, absolute paths, `..` traversal and nested archives. Every package is extracted to a staging directory, validated again, then renamed into place. The previous version is kept for rollback during the install.

## Installation and safe mode

Install from the skin library:

1. Choose **Install .zip…** or **Install folder…**.
2. Cadence validates the package and copies it into the managed skins directory.
3. The new skin appears in the list. Select **Use** to activate it.
4. If the frontend cannot mount a declarative skin, it reports the failure to Go, which restores the last known good skin or built-in Clarity.

Useful commands:

```bash
cadence --safe-mode              # built-in skin only, session-only
cadence --demo                   # simulated player
cadence --skins-dev ./skins-dev  # load development skins
```

Safe mode does not overwrite the persisted active skin. It disables third-party loading and installation for that session. Settings expose **Reset skin settings**, which returns to Clarity and clears the persisted skin state.

## Security model

- Skin content runs in a Shadow DOM and never receives JavaScript execution.
- The layout is JSON, not HTML; unknown fields and components are rejected.
- CSS is parsed and validated before activation; all asset URLs are local and canonicalized.
- The asset route serves only the active skin, only CSS and allowlisted asset types.
- The application response includes a Content Security Policy that disallows remote scripts, remote styles and remote connections.
- Skins never receive Spotify playback credentials, engine state files, network access or arbitrary filesystem access.
- A broken active skin cannot prevent startup: Cadence falls back to the last known good skin, then to Clarity.
