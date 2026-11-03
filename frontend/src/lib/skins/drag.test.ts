import { describe, expect, it } from "vitest";
import { titlebarDragTarget } from "./drag";

function buildTitlebar(): {
  host: HTMLElement;
  shadow: ShadowRoot;
  titlebar: HTMLElement;
  button: HTMLButtonElement;
  blank: HTMLDivElement;
} {
  const host = document.createElement("div");
  const shadow = host.attachShadow({ mode: "open" });
  const titlebar = document.createElement("header");
  titlebar.className = "cad-titlebar";
  const button = document.createElement("button");
  const blank = document.createElement("div");
  titlebar.append(button, blank);
  shadow.appendChild(titlebar);
  return { host, shadow, titlebar, button, blank };
}

describe("titlebarDragTarget", () => {
  it("accepts a blank titlebar path", () => {
    const { host, shadow, titlebar, blank } = buildTitlebar();
    expect(titlebarDragTarget([blank, titlebar, shadow, host, document, window])).toBe(titlebar);
  });

  it("rejects paths that begin on an interactive control", () => {
    const { host, shadow, titlebar, button } = buildTitlebar();
    expect(titlebarDragTarget([button, titlebar, shadow, host, document, window])).toBeNull();
  });

  it("rejects paths without a titlebar", () => {
    const { host, shadow, blank } = buildTitlebar();
    expect(titlebarDragTarget([blank, shadow, host, document, window])).toBeNull();
  });
});
