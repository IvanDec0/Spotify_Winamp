import { afterEach, describe, expect, it } from "vitest";
import {
  applyTokens,
  BROWSE_TOKENS,
  readSkinTokens,
  syncBrowseStylesheet,
  tokensFromStyle,
} from "./theme.svelte";

const styles: HTMLStyleElement[] = [];

function addClarityStyles(): void {
  const style = document.createElement("style");
  style.textContent = `
    .clarity[data-theme="graphite"] { --bg: #090b0f; --accent: #5ef0ff; --text: #e8edf4; }
    .clarity[data-theme="ice"] { --bg: #061018; --accent: #8be9ff; --text: #e8f6ff; }
  `;
  document.head.appendChild(style);
  styles.push(style);
}

afterEach(() => {
  for (const style of styles.splice(0)) style.remove();
  document.getElementById("cadence-browse-skin")?.remove();
});

// jsdom computes custom properties from document stylesheets but not from
// :host rules inside a shadow root, so the declarative shadow probe is covered
// by the live app rather than here. The Clarity probe and the pure helpers are
// exercised below.
describe("browse theme bridge", () => {
  it("extracts only known non-empty tokens from a style reader", () => {
    const values: Record<string, string> = {
      "--bg": " #10141b ",
      "--accent": "#5ef0ff",
      "--text": "",
      "--danger": "red; background: url(x)",
      "--font-ui": '"Inter", system-ui, sans-serif',
    };
    const tokens = tokensFromStyle({
      getPropertyValue: (name) => values[name] ?? "",
    });

    expect(tokens).toEqual({
      "--bg": "#10141b",
      "--accent": "#5ef0ff",
      "--font-ui": '"Inter", system-ui, sans-serif',
    });
    expect(
      Object.keys(tokens).every((name) => (BROWSE_TOKENS as readonly string[]).includes(name)),
    ).toBe(true);
  });

  it("applies tokens and clears properties removed by a skin change", () => {
    const root = document.createElement("div");

    applyTokens(root, { "--bg": "#10141b", "--accent": "#5ef0ff" });
    expect(root.style.getPropertyValue("--bg")).toBe("#10141b");
    expect(root.style.getPropertyValue("--accent")).toBe("#5ef0ff");

    applyTokens(root, { "--bg": "#e9f1f8" });
    expect(root.style.getPropertyValue("--bg")).toBe("#e9f1f8");
    expect(root.style.getPropertyValue("--accent")).toBe("");
  });

  it("re-reads Clarity tokens when the theme changes", async () => {
    addClarityStyles();

    const graphite = await readSkinTokens(null, "graphite");
    expect(graphite["--bg"]).toBe("#090b0f");
    expect(graphite["--accent"]).toBe("#5ef0ff");

    const ice = await readSkinTokens(null, "ice");
    expect(ice["--bg"]).toBe("#061018");
    expect(ice["--accent"]).toBe("#8be9ff");

    const root = document.createElement("div");
    applyTokens(root, graphite);
    applyTokens(root, ice);
    expect(root.style.getPropertyValue("--bg")).toBe("#061018");
    expect(document.body.querySelector(".clarity")).toBeNull();
  });

  it("keeps the optional skin stylesheet as a single head link", () => {
    syncBrowseStylesheet("/skins/lumina/browse.css");
    syncBrowseStylesheet("/skins/lumina/browse.css");
    expect(document.querySelectorAll("#cadence-browse-skin")).toHaveLength(1);
    expect(document.getElementById("cadence-browse-skin")?.getAttribute("href")).toBe(
      "/skins/lumina/browse.css",
    );

    syncBrowseStylesheet("/skins/afterglow/browse.css");
    expect(document.querySelectorAll("#cadence-browse-skin")).toHaveLength(1);
    expect(document.getElementById("cadence-browse-skin")?.getAttribute("href")).toBe(
      "/skins/afterglow/browse.css",
    );

    syncBrowseStylesheet("");
    expect(document.getElementById("cadence-browse-skin")).toBeNull();
  });
});
