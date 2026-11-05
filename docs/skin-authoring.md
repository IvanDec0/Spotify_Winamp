# Skin authoring guide

This guide walks through creating a level 2 Cadence skin: a custom layout, a theme palette and local assets. The formal reference is [`skin-format.md`](skin-format.md).

## 1. Create the development directory

```bash
mkdir -p skins-dev/my-skin/assets
```

Cadence discovers `skins-dev/*` automatically when it runs from the repository root. For a directory elsewhere:

```bash
cadence --skins-dev ~/cadence-skins
```

The skin ID must match the directory name. `clarity` is reserved for the built-in skin.

## 2. Write `manifest.json`

Start with one theme and one visualizer mode while iterating:

```json
{
  "schemaVersion": 1,
  "id": "my-skin",
  "name": "My Skin",
  "version": "0.1.0",
  "description": "A development skin.",
  "author": "Your name",
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
  "themes": ["sunset"],
  "defaultTheme": "sunset",
  "visualizer": {
    "modes": ["spectrum", "radial"],
    "default": "spectrum",
    "intensity": 0.5
  },
  "window": {
    "width": 420,
    "height": 220,
    "minWidth": 380,
    "minHeight": 200,
    "queueHeight": 460,
    "resizable": true
  }
}
```

`capabilities` must cover every component in the layout. A component's required capabilities are listed in the [component table](skin-format.md#components).

The optional `browse` block styles the separate Browse window:

```json
  "browse": { "css": "browse.css" }
```

## 3. Build `layout.json`

A practical starting point:

```json
{
  "schemaVersion": 1,
  "root": {
    "type": "stack",
    "children": [
      {
        "type": "component",
        "component": "visualizer",
        "props": { "mode": "spectrum", "layer": "background", "intensity": 0.4 }
      },
      {
        "type": "column",
        "name": "shell",
        "children": [
          { "type": "component", "component": "titlebar" },
          {
            "type": "row",
            "name": "hero",
            "props": { "gap": 12, "padding": 12, "align": "center" },
            "children": [
              { "type": "component", "component": "album-art", "props": { "size": 88, "frame": true } },
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
    ]
  }
}
```

Things to remember:

- Put layout properties under `props`. A `gap` directly on a node is rejected.
- The root and every container need at least one child.
- The titlebar is mandatory.
- Each component can be used once. If you want separate shuffle and repeat buttons, use the `shuffle` and `repeat` components and set `showShuffle`/`showRepeat` to `false` on `transport-controls`.
- Use `name` on containers to create stable CSS hooks such as `[data-name="hero"]`.
- Use `stack` for overlays such as a background visualizer.

## 4. Add `styles.css`

The base stylesheet provides a usable structure and sensible colors. Override the parts that define your skin:

```css
:host {
  --font-ui: "Inter", system-ui, sans-serif;
  --font-mono: "JetBrains Mono", monospace;
  --line: rgb(255 255 255 / 10%);
  --line-strong: rgb(255 255 255 / 18%);
}

.cad-runtime {
  background:
    radial-gradient(80% 90% at 85% 0%, color-mix(in srgb, var(--accent) 18%, transparent), transparent 60%),
    var(--bg);
  color: var(--text);
}

[data-name="shell"] {
  height: 100%;
}

[data-name="hero"] {
  flex: 1 1 auto;
  min-height: 0;
}

.cad-cover {
  border-radius: 45% 6% 45% 6%;
}

.cad-track-title {
  font-family: Georgia, serif;
  font-size: 22px;
}

[data-name="controls"] {
  border-top: 1px solid var(--line);
}
```

You do not need to define structural defaults for rows, columns, grids, titlebar drag regions, queue visibility or range inputs. Cadence's base stylesheet handles those inside the shadow tree.

### Theme file

```css
/* themes/sunset.css */
:host {
  --bg: #140d08;
  --surface: #1f1510;
  --text: #f7ede4;
  --muted: #b59f8e;
  --accent: #ffb347;
  --accent-2: #58e0c0;
}
```

Declare `sunset` in `themes` and set `defaultTheme` to it. The theme file is loaded after `styles.css`, so it can override palette rules. If the file is omitted, the theme ID still works, but you must define `[data-theme="sunset"]` rules in `styles.css`.

### Optional Browse stylesheet

The Catalog opens in its own window. Cadence applies the active theme tokens to
`.cadence-browse`, but it never injects the compact `styles.css` there. To
restyle the Browse shell, add a root-level `browse.css` and declare it:

```css
/* browse.css */
.cadence-browse .sidebar {
  border-right-color: color-mix(in srgb, var(--accent) 20%, var(--line));
}

.cadence-browse .card {
  background: color-mix(in srgb, var(--surface) 70%, transparent);
}
```

Use the same theme variables as `styles.css`. They are read from the active
theme, so a light theme stays readable without extra light-specific rules.
`browse.css` is validated like `styles.css`, must stay inside the package and
should scope every rule to `.cadence-browse`.

### Assets

Reference assets with paths relative to the stylesheet that uses them:

```css
.cad-runtime::after {
  background-image: url(assets/grain.png);
}
```

Allowed types are PNG, JPEG, WebP and WOFF2. SVG is not accepted from skins in v1. Do not use remote URLs, `data:` URLs, absolute paths, query strings or fragments.

## 5. Run and iterate

```bash
task dev
# or
wails3 build && ./bin/cadence
```

Open **Skins** from the titlebar or the appearance panel, select **My Skin**, then **Use**. Development skins are rescanned at startup, so restart Cadence after editing `manifest.json`, `layout.json` or `styles.css`.

Keyboard shortcuts while iterating:

| Key | Action |
|---|---|
| `Space` / `K` | Play or pause |
| `Shift+←` / `Shift+→` | Previous or next track |
| `←` / `→` | Seek 5 seconds |
| `↑` / `↓` | Volume |
| `Q` | Expand or collapse the queue |
| `V` | Cycle visualizer modes |
| `T` | Cycle themes |

## 6. Validate and package

Cadence validates automatically on installation and activation. The same rules are exercised by the Go test suite.

Create a ZIP with the required files at the archive root:

```bash
cd skins-dev/my-skin
zip -r ../my-skin.zip manifest.json layout.json styles.css themes assets
# Include browse.css when the manifest declares it:
zip -r ../my-skin.zip browse.css
```

Then use **Install .zip…** in the skin library. Installation does not activate the skin automatically; select **Use** after it appears.

Test checklist:

- The layout renders at its declared logical size with the queue closed: width 420 and height 220–460 (scaled by the current UI scale).
- Expanding and collapsing the queue works; the window returns to the base size.
- Every theme is selectable and changes the palette.
- Every declared visualizer mode renders.
- The queue displays current and upcoming items.
- Play/pause, seek, volume, shuffle and repeat controls work.
- The window can still be dragged from the titlebar, minimised and closed.
- **Browse** opens the dedicated window and follows the active theme; an
  optional `browse.css` loads only there.
- Reduced motion stops animated movement.
- Invalid props, unknown components, remote URLs or unsupported assets are rejected.

## 7. Common validation errors

| Error | Fix |
|---|---|
| `layout root must be a stack, row, column or grid container` | Wrap the root in a container. |
| `layout must include the titlebar component` | Add a `titlebar` component. |
| `component "x" appears more than once` | Use each component once; split controls into separate registered components. |
| `unknown titlebar prop "..."` | Check the props table in the format reference. |
| `prop "gap" must be an integer` | Put numeric props under `props` and use whole numbers. |
| `component "queue" requires the "queue.read" capability` | Add the capability to the manifest. |
| `visualizer mode "grid" is not declared in the manifest` | Add it to `visualizer.modes` or change the layout. |
| `stylesheet references a missing asset` | Check the relative path and file name. |
| `remote or absolute URL is not allowed` | Bundle the asset locally and use a relative path. |
| `unsupported file type` | Use PNG, JPEG, WebP or WOFF2 only. |

## 8. Security expectations

A skin is untrusted input. The validator rejects executable CSS, remote requests, traversal, symlinks, oversized files and unknown layout data before the skin can be activated. Keep skins self-contained and local. Never add a build step that fetches remote assets at runtime.
