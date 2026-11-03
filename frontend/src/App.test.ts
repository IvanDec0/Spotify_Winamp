import { fireEvent, render, screen, waitFor } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { music } from "./lib/music.svelte";
import { preferences } from "./lib/preferences.svelte";
import { skins } from "./lib/skins.svelte";

const mocks = vi.hoisted(() => {
  const setSize = vi.fn(() => Promise.resolve());
  const setMinSize = vi.fn(() => Promise.resolve());
  const getZoom = vi.fn(() => Promise.resolve(2));
  const setZoom = vi.fn(() => Promise.resolve());
  const size = vi.fn(() => Promise.resolve({ width: 840, height: 440 }));
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

  const catalog = {
    skins: [
      {
        id: "clarity",
        name: "Clarity",
        version: "1.0.0",
        description: "Built-in skin",
        author: "Cadence",
        license: "CC BY-SA 4.0",
        source: "builtin",
        builtin: true,
        removable: false,
        active: true,
        themes: ["graphite"],
        defaultTheme: "graphite",
        visualizerModes: ["spectrum"],
      },
    ],
    activeId: "clarity",
    activeTheme: "graphite",
    safeMode: false,
    thirdPartyDisabled: false,
    warnings: [],
    lastError: "",
    canInstall: true,
    canChangeActiveSkin: true,
  };

  const runtime = {
    id: "clarity",
    name: "Clarity",
    version: "1.0.0",
    author: "Cadence",
    license: "CC BY-SA 4.0",
    description: "Built-in skin",
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
  };

  const audio = {
    available: true,
    running: true,
    authenticated: false,
    engine: "go-librespot",
    engineVersion: "",
    deviceName: "Cadence",
    deviceType: "",
    pairing: null,
    error: "",
    binaryPath: "/tmp/go-librespot",
    configDirectory: "/tmp/cadence-audio",
    audioBackend: "pulseaudio",
  };

  const on = vi.fn(() => () => {});

  return {
    setSize,
    setMinSize,
    getZoom,
    setZoom,
    size,
    minimise,
    close,
    player,
    catalog,
    runtime,
    audio,
    on,
  };
});

vi.mock("@wailsio/runtime", () => ({
  Window: {
    Size: mocks.size,
    SetSize: mocks.setSize,
    SetMinSize: mocks.setMinSize,
    GetZoom: mocks.getZoom,
    SetZoom: mocks.setZoom,
    Minimise: mocks.minimise,
    Close: mocks.close,
  },
  Screens: {
    GetCurrent: vi.fn(() =>
      Promise.resolve({
        WorkArea: { X: 0, Y: 0, Width: 1920, Height: 1048 },
        PhysicalWorkArea: { X: 0, Y: 0, Width: 1920, Height: 1048 },
      }),
    ),
  },
  Events: { On: mocks.on },
  System: { invoke: vi.fn() },
}));

vi.mock("./lib/player.svelte", () => ({ player: mocks.player }));

vi.mock("../bindings/cadence/internal/platform", () => ({
  WindowService: { SetSize: mocks.setSize },
}));

vi.mock("../bindings/cadence/internal/golibrespot", () => ({
  Service: {
    GetState: vi.fn(() => Promise.resolve(mocks.audio)),
    OpenPairing: vi.fn(() => Promise.resolve()),
    ResetSession: vi.fn(() => Promise.resolve(mocks.audio)),
    Restart: vi.fn(() => Promise.resolve(mocks.audio)),
    Playlists: vi.fn(() => Promise.resolve({ total: 0, offset: 0, limit: 100, items: [] })),
    Albums: vi.fn(() => Promise.resolve({ total: 0, offset: 0, limit: 100, items: [] })),
    Artists: vi.fn(() => Promise.resolve({ total: 0, offset: 0, limit: 100, items: [] })),
    ContextTracks: vi.fn(() =>
      Promise.resolve({ uri: "", ready: true, length: 0, cached: 0, items: [] }),
    ),
    PlayContext: vi.fn(() => Promise.resolve()),
    PlayTrack: vi.fn(() => Promise.resolve()),
    AddToQueue: vi.fn(() => Promise.resolve()),
  },
}));

vi.mock("../bindings/cadence/internal/skins", () => ({
  Service: {
    GetCatalog: vi.fn(() => Promise.resolve(mocks.catalog)),
    GetRuntime: vi.fn(() => Promise.resolve(mocks.runtime)),
    Activate: vi.fn(() => Promise.resolve(mocks.catalog)),
    SetTheme: vi.fn(() => Promise.resolve(mocks.catalog)),
    SetThirdPartyDisabled: vi.fn(() => Promise.resolve(mocks.catalog)),
    Reset: vi.fn(() => Promise.resolve(mocks.catalog)),
    Uninstall: vi.fn(() => Promise.resolve(mocks.catalog)),
    InstallZipDialog: vi.fn(() => Promise.resolve({ canceled: true })),
    InstallDirectoryDialog: vi.fn(() => Promise.resolve({ canceled: true })),
    ReportRuntimeReady: vi.fn(() => Promise.resolve()),
    ReportRuntimeFailure: vi.fn(() => Promise.resolve(mocks.catalog)),
  },
}));

import App from "./App.svelte";

describe("App shell", () => {
  beforeEach(() => {
    mocks.setSize.mockClear();
    mocks.player.toggle.mockClear();
    mocks.player.transfer.mockClear();
    preferences.setTheme("graphite");
    preferences.setMotion("system");
    skins.queueOpen = false;
    skins.error = null;
    skins.notice = null;
    music.error = null;
    music.notice = null;
    vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue(null);
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("expands and collapses the queue with the window", async () => {
    render(App);

    await fireEvent.click(screen.getByRole("button", { name: "Show queue" }));
    await waitFor(() => expect(mocks.setSize).toHaveBeenCalledWith(840, 920));
    expect(screen.getByLabelText("Playback queue")).not.toBeNull();

    await fireEvent.click(screen.getByRole("button", { name: "Hide queue" }));
    await waitFor(() => expect(mocks.setSize).toHaveBeenLastCalledWith(840, 440), {
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

  it("opens the skin library from appearance settings", async () => {
    render(App);

    await fireEvent.click(screen.getByRole("button", { name: "Open appearance settings" }));
    await fireEvent.click(screen.getByRole("button", { name: "Skins" }));

    expect(screen.getByRole("dialog", { name: "Skin library" })).not.toBeNull();
    expect(screen.getByRole("button", { name: "Close skin library" })).not.toBeNull();
    await waitFor(() => expect(mocks.setSize).toHaveBeenCalledWith(840, 920));
  });

  it("opens the music library from the provider badge", async () => {
    render(App);

    await fireEvent.click(screen.getByRole("button", { name: "Open music library" }));

    expect(screen.getByRole("dialog", { name: "Music library" })).not.toBeNull();
    expect(screen.getByRole("button", { name: "Close music library" })).not.toBeNull();
    await waitFor(() => expect(mocks.setSize).toHaveBeenCalledWith(840, 920));
  });

  it("applies the reduced-motion preference to the shell", async () => {
    preferences.setMotion("reduced");
    render(App);

    const shell = document.querySelector(".clarity");
    expect(shell?.classList.contains("reduce-motion")).toBe(true);
  });
});
