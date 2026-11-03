import { render, waitFor } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { SkinRuntime } from "../../../bindings/cadence/internal/skins";
import SkinHost from "./SkinHost.svelte";

const mocks = vi.hoisted(() => {
  const reportReady = vi.fn(() => Promise.resolve());
  const reportFailure = vi.fn(() => Promise.resolve({}));
  const invoke = vi.fn();
  const player = {
    state: {
      provider: "demo",
      status: "playing",
      track: {
        id: "neon-meridian",
        title: "Neon Meridian",
        artists: ["Aster Vale"],
        album: "Night Cartography",
        cover: "meridian",
        uri: "",
        durationMs: 252_000,
      },
      progressMs: 38_000,
      durationMs: 252_000,
      volume: 64,
      shuffle: false,
      repeat: "off",
      device: {
        id: "studio",
        name: "Studio monitors",
        type: "speaker",
        active: true,
        volumePercent: 64,
        restricted: false,
      },
      devices: [],
      queue: [],
      canSeek: true,
      canControl: true,
      revision: 1,
    },
    error: null,
    setVolume: vi.fn(),
    toggle: vi.fn(),
  };
  return { reportReady, reportFailure, invoke, player };
});

vi.mock("@wailsio/runtime", () => ({
  System: { invoke: mocks.invoke },
  Window: { Minimise: vi.fn(), Close: vi.fn() },
  Events: { On: vi.fn(() => () => {}) },
}));

vi.mock("../player.svelte", () => ({ player: mocks.player }));

vi.mock("../../../bindings/cadence/internal/skins", () => ({
  Service: {
    ReportRuntimeReady: mocks.reportReady,
    ReportRuntimeFailure: mocks.reportFailure,
    GetRuntime: vi.fn(() =>
      Promise.resolve({
        id: "clarity",
        name: "Clarity",
        version: "1.0.0",
        author: "Cadence",
        license: "CC BY-SA 4.0",
        description: "Built-in",
        builtin: true,
        themes: ["graphite"],
        defaultTheme: "graphite",
        visualizerModes: ["spectrum"],
        defaultVisualizer: "spectrum",
        intensity: 0.7,
        layout: { schemaVersion: 1, root: { type: "column", children: [] } },
        stylesheetUrl: "",
        themeStylesheets: {},
        baseHeight: 220,
        queueHeight: 460,
      }),
    ),
  },
}));

const runtime: SkinRuntime = {
  id: "afterglow",
  name: "Afterglow",
  version: "1.0.0",
  author: "Cadence",
  license: "CC BY-SA 4.0",
  description: "Test skin",
  builtin: false,
  themes: ["ember", "tide"],
  defaultTheme: "ember",
  visualizerModes: ["spectrum"],
  defaultVisualizer: "spectrum",
  intensity: 0.55,
  layout: {
    schemaVersion: 1,
    root: {
      type: "column",
      children: [
        { type: "component", component: "titlebar" },
        { type: "component", component: "track-info" },
      ],
    },
  },
  stylesheetUrl: "/skins/afterglow/styles.css",
  themeStylesheets: {},
  baseHeight: 220,
  queueHeight: 460,
};

describe("SkinHost", () => {
  beforeEach(() => {
    mocks.reportReady.mockClear();
    mocks.reportFailure.mockClear();
    mocks.invoke.mockClear();
  });

  it("mounts a declarative layout in a shadow root and reports readiness", async () => {
    const { container } = render(SkinHost, { runtime, theme: "ember", queueOpen: false });
    const host = container.querySelector<HTMLElement>(".skin-host");
    expect(host).not.toBeNull();

    await waitFor(() => {
      expect(host?.shadowRoot?.querySelector(".cad-titlebar")).not.toBeNull();
    });
    expect(host?.dataset.theme).toBe("ember");
    expect(host?.dataset.queueOpen).toBe("false");
    expect(mocks.reportReady).toHaveBeenCalledWith("afterglow", "ember");
  });

  it("updates queue and theme attributes without leaving stale content", async () => {
    const { container, rerender } = render(SkinHost, {
      runtime,
      theme: "ember",
      queueOpen: false,
    });
    const host = container.querySelector<HTMLElement>(".skin-host");
    await waitFor(() => expect(host?.shadowRoot?.querySelector(".cad-titlebar")).not.toBeNull());

    await rerender({ runtime, theme: "tide", queueOpen: true });
    await waitFor(() => expect(host?.dataset.theme).toBe("tide"));
    expect(host?.dataset.queueOpen).toBe("true");
    expect(host?.shadowRoot?.querySelectorAll(".cad-runtime")).toHaveLength(1);
  });

  it("replaces the mounted runtime when the skin id changes", async () => {
    const { container, rerender } = render(SkinHost, {
      runtime,
      theme: "ember",
      queueOpen: false,
    });
    const host = container.querySelector<HTMLElement>(".skin-host");
    await waitFor(() => expect(host?.shadowRoot?.querySelector(".cad-titlebar")).not.toBeNull());

    const other: SkinRuntime = {
      ...runtime,
      id: "other",
      name: "Other",
      layout: {
        schemaVersion: 1,
        root: {
          type: "column",
          children: [{ type: "component", component: "titlebar" }],
        },
      },
    };
    await rerender({ runtime: other, theme: "tide", queueOpen: false });
    await waitFor(() => expect(host?.shadowRoot?.querySelectorAll(".cad-runtime")).toHaveLength(1));
    expect(host?.shadowRoot?.querySelector(".cad-track-info")).toBeNull();
  });

  it("starts a native drag from the titlebar but not from its controls", async () => {
    const { container } = render(SkinHost, { runtime, theme: "ember", queueOpen: false });
    const host = container.querySelector<HTMLElement>(".skin-host");
    await waitFor(() => expect(host?.shadowRoot?.querySelector(".cad-titlebar")).not.toBeNull());

    const titlebar = host?.shadowRoot?.querySelector<HTMLElement>(".cad-titlebar");
    titlebar?.dispatchEvent(
      new MouseEvent("mousedown", {
        bubbles: true,
        composed: true,
        button: 0,
        detail: 1,
        clientX: 10,
        clientY: 10,
      }),
    );
    window.dispatchEvent(new MouseEvent("mousemove", { bubbles: true, clientX: 30, clientY: 30 }));
    expect(mocks.invoke).toHaveBeenCalledWith("wails:drag");

    window.dispatchEvent(new MouseEvent("mouseup", { bubbles: true }));
    mocks.invoke.mockClear();

    const button = host?.shadowRoot?.querySelector<HTMLButtonElement>(".cad-title-button");
    button?.dispatchEvent(
      new MouseEvent("mousedown", {
        bubbles: true,
        composed: true,
        button: 0,
        detail: 1,
        clientX: 10,
        clientY: 10,
      }),
    );
    window.dispatchEvent(new MouseEvent("mousemove", { bubbles: true, clientX: 30, clientY: 30 }));
    expect(mocks.invoke).not.toHaveBeenCalled();
  });

  it("reports an invalid layout instead of mounting it", async () => {
    const invalid: SkinRuntime = {
      ...runtime,
      id: "broken",
      layout: {
        schemaVersion: 1,
        root: {
          type: "column",
          children: [{ type: "component", component: "not-registered" }],
        },
      },
    };
    const { container } = render(SkinHost, { runtime: invalid, theme: "ember", queueOpen: false });
    await waitFor(() => expect(mocks.reportFailure).toHaveBeenCalled());
    expect(
      container.querySelector(".skin-host")?.shadowRoot?.querySelector(".cad-runtime"),
    ).toBeFalsy();
  });
});
