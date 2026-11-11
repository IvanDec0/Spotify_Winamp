import type { SkinRuntime } from "../../shared/api/types";

/**
 * Custom properties the Browse shell consumes. Only tokens present in the
 * active skin theme are applied; the base `browse.css` provides fallbacks.
 */
export const BROWSE_TOKENS = [
  "--bg",
  "--surface",
  "--surface-2",
  "--surface-3",
  "--panel",
  "--text",
  "--muted",
  "--faint",
  "--line",
  "--line-strong",
  "--accent",
  "--accent-2",
  "--accent-soft",
  "--accent-glow",
  "--danger",
  "--font-ui",
  "--font-mono",
  "--ease-out",
  "--ease-spring",
] as const;

export type BrowseToken = (typeof BROWSE_TOKENS)[number];

/** Minimal shape shared by `CSSStyleDeclaration` and test fakes. */
export interface StyleReader {
  getPropertyValue(name: string): string;
}

const PROBE_STYLE =
  "position:fixed;left:-10000px;top:-10000px;width:1px;height:1px;opacity:0;pointer-events:none";

const STYLESHEET_TIMEOUT_MS = 1500;
const BROWSE_SKIN_LINK_ID = "cadence-browse-skin";

/**
 * Reads the known tokens from a computed style and drops empty or obviously
 * invalid values. Pure so it can be unit tested without a DOM.
 */
export function tokensFromStyle(style: StyleReader): Record<string, string> {
  const tokens: Record<string, string> = {};
  for (const name of BROWSE_TOKENS) {
    const value = style.getPropertyValue(name).trim();
    if (value && !/[;{}]/.test(value)) {
      tokens[name] = value;
    }
  }
  return tokens;
}

/**
 * Applies tokens as inline custom properties on the Browse root. Tokens that
 * disappeared since the previous skin are removed so the base fallbacks win.
 */
export function applyTokens(root: HTMLElement, tokens: Record<string, string>): void {
  for (const name of BROWSE_TOKENS) {
    const value = tokens[name];
    if (value) {
      root.style.setProperty(name, value);
    } else {
      root.style.removeProperty(name);
    }
  }
}

function createStylesheet(href: string): HTMLLinkElement {
  const link = document.createElement("link");
  link.rel = "stylesheet";
  link.href = href;
  return link;
}

function waitForStylesheets(links: HTMLLinkElement[]): Promise<void> {
  return new Promise((resolve) => {
    let pending = links.length;
    let settled = false;
    const finish = (): void => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      resolve();
    };
    const timer = setTimeout(finish, STYLESHEET_TIMEOUT_MS);

    if (pending === 0) {
      finish();
      return;
    }
    for (const link of links) {
      const loaded = (): void => {
        pending -= 1;
        if (pending <= 0) finish();
      };
      link.addEventListener("load", loaded, { once: true });
      link.addEventListener("error", loaded, { once: true });
    }
  });
}

/**
 * Extracts the active theme's tokens without touching the Browse document
 * styles. Declarative skins are probed through a hidden shadow host that loads
 * their own `styles.css` and theme file, so the compact skin CSS is never
 * injected into this window. Built-in Clarity is read from the `.clarity`
 * rules already present in `app.css`.
 */
export async function readSkinTokens(
  runtime: SkinRuntime | null,
  theme: string,
): Promise<Record<string, string>> {
  if (typeof document === "undefined") return {};

  if (!runtime || runtime.builtin) {
    const probe = document.createElement("div");
    probe.className = "clarity";
    probe.dataset.theme = theme;
    probe.setAttribute("aria-hidden", "true");
    probe.style.cssText = PROBE_STYLE;
    document.body.appendChild(probe);
    try {
      return tokensFromStyle(getComputedStyle(probe));
    } finally {
      probe.remove();
    }
  }

  const host = document.createElement("div");
  host.dataset.theme = theme;
  host.style.cssText = PROBE_STYLE;
  host.setAttribute("aria-hidden", "true");
  const shadow = host.attachShadow({ mode: "open" });
  document.body.appendChild(host);

  try {
    const links = [createStylesheet(runtime.stylesheetUrl)];
    const themeURL = runtime.themeStylesheets?.[theme];
    if (themeURL) {
      links.push(createStylesheet(themeURL));
    }
    for (const link of links) {
      shadow.appendChild(link);
    }
    await waitForStylesheets(links);
    return tokensFromStyle(getComputedStyle(host));
  } finally {
    host.remove();
  }
}

/**
 * Keeps the optional skin Browse stylesheet as the last head link. An empty
 * URL removes it, and the dedicated window guarantees it never leaks into the
 * compact surface.
 */
export function syncBrowseStylesheet(url: string, version = ""): void {
  if (typeof document === "undefined") return;

  const existing = document.getElementById(BROWSE_SKIN_LINK_ID) as HTMLLinkElement | null;
  if (!url) {
    existing?.remove();
    return;
  }
  if (existing?.getAttribute("href") === url && existing.dataset.version === version) return;

  existing?.remove();
  const link = createStylesheet(url);
  link.id = BROWSE_SKIN_LINK_ID;
  link.dataset.version = version;
  document.head.appendChild(link);
}
