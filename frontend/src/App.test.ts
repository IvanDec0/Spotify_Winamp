import { fireEvent, render, screen, waitFor } from "@testing-library/svelte";
import { tick } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { preferences } from "./lib/preferences.svelte";
import { skins } from "./lib/skins.svelte";
import { createPlayerState, playerMock } from "./lib/testing/player-mock.svelte";

const mocks = vi.hoisted(() => {
  const setSize = vi.fn(() => Promise.resolve());
  const setMinSize = vi.fn(() => Promise.resolve());
  const getZoom = vi.fn(() => Promise.resolve(2));
  const setZoom = vi.fn(() => Promise.resolve());
  const size = vi.fn(() => Promise.resolve({ width: 840, height: 440 }));
  const minimise = vi.fn(() => Promise.resolve());
  const close = vi.fn(() => Promise.resolve());
  const openBrowse = vi.fn(() => Promise.resolve());

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

  const on = vi.fn(() => () => {});

  const music = {
    loading: false,
    audio: {
      playbackBackend: "librespot",
      soloist: null as Record<string, unknown> | null,
    },
    soloistMode: false,
    soloist: null as Record<string, unknown> | null,
    start: vi.fn(() => Promise.resolve()),
    stop: vi.fn(),
  };

  return {
    setSize,
    setMinSize,
    getZoom,
    setZoom,
    size,
    minimise,
    close,
    openBrowse,
    catalog,
    runtime,
    on,
    music,
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

vi.mock("./lib/player.svelte", async () => {
  const { playerMock: player } = await import("./lib/testing/player-mock.svelte");
  return { player };
});

vi.mock("./lib/music.svelte", () => ({ music: mocks.music }));

vi.mock("../bindings/cadence/internal/platform", () => ({
  WindowService: { SetSize: mocks.setSize, OpenBrowse: mocks.openBrowse },
}));

vi.mock("../bindings/cadence/internal/wailsapi/skins", () => ({
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
    playerMock.state = createPlayerState();
    playerMock.queue = [];
    playerMock.queueLoading = false;
    playerMock.loadQueue.mockClear();
    playerMock.toggle.mockClear();
    playerMock.transfer.mockClear();
    preferences.setTheme("graphite");
    preferences.setMotion("system");
    skins.queueOpen = false;
    skins.error = null;
    skins.errorKey = null;
    skins.notice = null;
    mocks.openBrowse.mockClear();
    mocks.music.loading = false;
    mocks.music.audio = { playbackBackend: "librespot", soloist: null };
    mocks.music.soloistMode = false;
    mocks.music.soloist = null;
    mocks.music.start.mockClear();
    mocks.music.stop.mockClear();
    localStorage.clear();
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

  it("loads the queue on open and only again when the track URI changes", async () => {
    render(App);
    await tick();
    expect(playerMock.loadQueue).not.toHaveBeenCalled();

    await fireEvent.click(screen.getByRole("button", { name: "Show queue" }));
    await waitFor(() => expect(playerMock.loadQueue).toHaveBeenCalledTimes(1));

    // Applied states with the same track must not reload the queue.
    playerMock.state = { ...playerMock.state, volume: 70 };
    await tick();
    playerMock.state = { ...playerMock.state, progressMs: 40_000 };
    await tick();
    expect(playerMock.loadQueue).toHaveBeenCalledTimes(1);

    playerMock.state = {
      ...playerMock.state,
      track: { ...playerMock.state.track, uri: "spotify:track:2" },
    };
    await waitFor(() => expect(playerMock.loadQueue).toHaveBeenCalledTimes(2));
  });

  it("opens Browse Settings from the compact settings button", async () => {
    render(App);

    await fireEvent.click(screen.getByRole("button", { name: "Settings" }));

    await waitFor(() => expect(mocks.openBrowse).toHaveBeenCalledWith("settings"));
    expect(screen.queryByRole("dialog", { name: "Appearance settings" })).toBeNull();
    expect(screen.queryByRole("dialog", { name: "Skin library" })).toBeNull();
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
    expect(playerMock.transfer).toHaveBeenCalledWith("living-room");
    expect(screen.queryByRole("dialog", { name: "Playback devices" })).toBeNull();
  });

  it("maps global keyboard shortcuts and ignores editable targets", async () => {
    render(App);

    await fireEvent.keyDown(window, { key: " " });
    expect(playerMock.toggle).toHaveBeenCalledOnce();

    await fireEvent.keyDown(window, { key: "ArrowRight" });
    expect(playerMock.seek).toHaveBeenCalledWith(43_000);

    playerMock.toggle.mockClear();
    await fireEvent.keyDown(screen.getByRole("slider", { name: "Playback position" }), {
      key: " ",
    });
    expect(playerMock.toggle).not.toHaveBeenCalled();
  });

  it("ignores shortcuts when another handler already prevented the default", () => {
    render(App);
    playerMock.toggle.mockClear();

    const event = new KeyboardEvent("keydown", {
      key: " ",
      bubbles: true,
      cancelable: true,
    });
    event.preventDefault();
    window.dispatchEvent(event);

    expect(playerMock.toggle).not.toHaveBeenCalled();
  });

  it("opens the Browse window directly from the provider badge", async () => {
    render(App);

    const browseBadge = screen.getByRole("button", { name: "Open Browse" });
    expect(browseBadge.textContent).toContain("Browse");
    await fireEvent.click(browseBadge);

    await waitFor(() => expect(mocks.openBrowse).toHaveBeenCalledWith("library"));
    expect(screen.queryByRole("dialog", { name: "Music library" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Close music library" })).toBeNull();
  });

  it("applies the reduced-motion preference to the shell", async () => {
    preferences.setMotion("reduced");
    render(App);

    const shell = document.querySelector(".clarity");
    expect(shell?.classList.contains("reduce-motion")).toBe(true);
  });

  it("prompts for the Soloist API key at launch when the build has none", () => {
    const soloist = { installed: true, apiKeySet: false, installing: false };
    mocks.music.soloistMode = true;
    mocks.music.audio = { playbackBackend: "soloist", soloist };
    mocks.music.soloist = soloist;

    render(App);

    expect(screen.getByRole("dialog", { name: "Set up Spotify Soloist" })).not.toBeNull();
    expect(screen.getByText(/Soloist needs a one-time API key/)).not.toBeNull();
    expect(mocks.music.start).toHaveBeenCalledOnce();
  });
});
