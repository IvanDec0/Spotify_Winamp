import { fireEvent, render, screen, waitFor } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { preferences } from "./lib/preferences.svelte";

const mocks = vi.hoisted(() => {
  const setSize = vi.fn(() => Promise.resolve());
  const minimise = vi.fn(() => Promise.resolve());
  const close = vi.fn(() => Promise.resolve());

  const state = {
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
    devices: [
      {
        id: "studio",
        name: "Studio monitors",
        type: "speaker",
        active: true,
        volumePercent: 64,
        restricted: false,
      },
      {
        id: "living-room",
        name: "Living room",
        type: "speaker",
        active: false,
        volumePercent: 38,
        restricted: false,
      },
    ],
    queue: [
      {
        track: {
          id: "neon-meridian",
          title: "Neon Meridian",
          artists: ["Aster Vale"],
          album: "Night Cartography",
          cover: "meridian",
          uri: "",
          durationMs: 252_000,
        },
        index: 0,
        isCurrent: true,
        isNext: false,
      },
    ],
    canSeek: true,
    canControl: true,
    revision: 3,
  };

  const player = {
    state,
    error: null as string | null,
    start: vi.fn(() => Promise.resolve()),
    stop: vi.fn(),
    toggle: vi.fn(() => Promise.resolve()),
    play: vi.fn(() => Promise.resolve()),
    pause: vi.fn(() => Promise.resolve()),
    next: vi.fn(() => Promise.resolve()),
    previous: vi.fn(() => Promise.resolve()),
    seek: vi.fn(() => Promise.resolve()),
    setVolume: vi.fn(() => Promise.resolve()),
    toggleShuffle: vi.fn(() => Promise.resolve()),
    cycleRepeat: vi.fn(() => Promise.resolve()),
    transfer: vi.fn(() => Promise.resolve()),
    dismissError: vi.fn(),
  };

  return { setSize, minimise, close, player };
});

vi.mock("@wailsio/runtime", () => ({
  Window: {
    SetSize: mocks.setSize,
    Minimise: mocks.minimise,
    Close: mocks.close,
  },
}));

vi.mock("./lib/player.svelte", () => ({ player: mocks.player }));

vi.mock("../bindings/cadence/internal/platform", () => ({
  WindowService: { SetSize: mocks.setSize },
}));

import App from "./App.svelte";

describe("App shell", () => {
  beforeEach(() => {
    mocks.setSize.mockClear();
    mocks.player.toggle.mockClear();
    mocks.player.transfer.mockClear();
    preferences.setTheme("graphite");
    preferences.setMotion("system");
    vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue(null);
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("expands and collapses the queue with the window", async () => {
    render(App);

    await fireEvent.click(screen.getByRole("button", { name: "Show queue" }));
    expect(mocks.setSize).toHaveBeenCalledWith(420, 460);
    expect(screen.getByLabelText("Playback queue")).not.toBeNull();

    await fireEvent.click(screen.getByRole("button", { name: "Hide queue" }));
    await waitFor(() => expect(mocks.setSize).toHaveBeenLastCalledWith(420, 220), {
      timeout: 1_000,
    });
    expect(screen.queryByLabelText("Playback queue")).toBeNull();
  });

  it("opens appearance settings, changes the theme and closes with Escape", async () => {
    render(App);

    await fireEvent.click(screen.getByRole("button", { name: "Open appearance settings" }));
    expect(screen.getByRole("dialog", { name: "Appearance settings" })).not.toBeNull();

    await fireEvent.click(screen.getByRole("button", { name: "Ice theme" }));
    expect(document.querySelector('[data-theme="ice"]')).not.toBeNull();

    await fireEvent.keyDown(window, { key: "Escape" });
    expect(screen.queryByRole("dialog", { name: "Appearance settings" })).toBeNull();
  });

  it("transfers playback to another device", async () => {
    render(App);

    await fireEvent.click(
      screen.getByRole("button", {
        name: "Playback device: Studio monitors. Choose a device",
      }),
    );
    expect(screen.getByRole("dialog", { name: "Playback devices" })).not.toBeNull();

    await fireEvent.click(screen.getByRole("button", { name: "Living room" }));
    expect(mocks.player.transfer).toHaveBeenCalledWith("living-room");
    expect(screen.queryByRole("dialog", { name: "Playback devices" })).toBeNull();
  });

  it("maps global keyboard shortcuts and ignores editable targets", async () => {
    render(App);

    await fireEvent.keyDown(window, { key: " " });
    expect(mocks.player.toggle).toHaveBeenCalledOnce();

    await fireEvent.keyDown(window, { key: "ArrowRight" });
    expect(mocks.player.seek).toHaveBeenCalledWith(43_000);

    mocks.player.toggle.mockClear();
    await fireEvent.keyDown(screen.getByRole("slider", { name: "Playback position" }), {
      key: " ",
    });
    expect(mocks.player.toggle).not.toHaveBeenCalled();
  });

  it("applies the reduced-motion preference to the shell", async () => {
    preferences.setMotion("reduced");
    render(App);

    const shell = document.querySelector(".clarity");
    expect(shell?.classList.contains("reduce-motion")).toBe(true);
  });
});
