import { describe, expect, it } from "vitest";
import { Navigation } from "./navigation.svelte";

describe("Navigation", () => {
  it("starts on home with no panel", () => {
    const nav = new Navigation();
    expect(nav.panel).toBe("none");
    expect(nav.route).toEqual({ name: "home" });
    expect(nav.stack).toEqual([{ name: "home" }]);
  });

  it("keeps one panel open at a time", () => {
    const nav = new Navigation();
    nav.open("library");
    expect(nav.panel).toBe("library");
    nav.open("queue");
    expect(nav.panel).toBe("queue");
    nav.close();
    expect(nav.panel).toBe("none");
  });

  it("toggles the same panel without opening another", () => {
    const nav = new Navigation();
    nav.toggle("settings");
    expect(nav.panel).toBe("settings");
    nav.toggle("settings");
    expect(nav.panel).toBe("none");
  });

  it("pushes and pops routes", () => {
    const nav = new Navigation();
    nav.push({ name: "search", query: "daft punk" });
    nav.push({ name: "entity", uri: "spotify:album:1" });

    expect(nav.route).toEqual({ name: "entity", uri: "spotify:album:1" });
    expect(nav.stack).toHaveLength(3);

    nav.back();
    expect(nav.route).toEqual({ name: "search", query: "daft punk" });

    nav.back();
    expect(nav.route).toEqual({ name: "home" });
  });

  it("does nothing when going back from the root", () => {
    const nav = new Navigation();
    nav.back();
    expect(nav.stack).toEqual([{ name: "home" }]);
    expect(nav.route).toEqual({ name: "home" });
  });

  it("resets the panel and the stack", () => {
    const nav = new Navigation();
    nav.open("devices");
    nav.push({ name: "entity", uri: "spotify:track:1" });
    nav.reset();

    expect(nav.panel).toBe("none");
    expect(nav.stack).toEqual([{ name: "home" }]);
  });
});
