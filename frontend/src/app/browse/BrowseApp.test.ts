import { fireEvent, render, screen, waitFor } from "@testing-library/svelte";
import { tick } from "svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { playerMock } from "../../lib/testing/player-mock.svelte";
import { preferences } from "../../lib/preferences.svelte";
import { clampBrowseSize } from "./browse-window";

const mocks = vi.hoisted(() => ({
  runtime: {
    id: "clarity",
    name: "Clarity",
    version: "1.0.0",
    author: "Cadence",
    license: "CC BY-SA 4.0",
    description: "Built-in skin",
    builtin: true,
    themes: ["graphite", "ice"],
    defaultTheme: "graphite",
    visualizerModes: ["spectrum"],
    defaultVisualizer: "spectrum",
    intensity: 0.7,
    layout: { schemaVersion: 1, root: { type: "column", children: [] } },
    stylesheetUrl: "",
    browseStylesheetUrl: "",
    themeStylesheets: {},
    baseHeight: 220,
    queueHeight: 460,
  },
  loading: false,
  onBrowseNavigate: vi.fn(),
  closeBrowse: vi.fn(() => Promise.resolve()),
  minimise: vi.fn(() => Promise.resolve()),
  setBrowseSize: vi.fn(() => Promise.resolve()),
  size: vi.fn(() => Promise.resolve({ width: 960, height: 640 })),
  navigateHandler: null as ((section: string) => void) | null,
  music: {
    authenticated: true,
    loading: false,
    busy: false,
    error: null as string | null,
    audio: { available: true, binaryPath: "/tmp/go-librespot" },
    pairing: null as { code: string; url: string } | null,
    openPairing: vi.fn(() => Promise.resolve()),
    resetSession: vi.fn(() => Promise.resolve()),
    restart: vi.fn(() => Promise.resolve()),
    refresh: vi.fn(() => Promise.resolve()),
  },
  audio: {
    search: vi.fn(),
    playlists: vi.fn(),
    albums: vi.fn(),
    artists: vi.fn(),
    contextTracks: vi.fn(),
    yourEpisodes: vi.fn(),
    startRadio: vi.fn(),
    playContext: vi.fn(),
    playTrack: vi.fn(),
    addToQueue: vi.fn(),
    setLiked: vi.fn(),
    isLiked: vi.fn(),
    saveAlbum: vi.fn(),
    isAlbumSaved: vi.fn(),
    saveShow: vi.fn(),
    isShowSaved: vi.fn(),
    followArtist: vi.fn(),
    isArtistFollowed: vi.fn(),
    addToPlaylist: vi.fn(),
    removeFromPlaylist: vi.fn(),
    getEngineSettings: vi.fn(),
    setEngineSettings: vi.fn(),
    restart: vi.fn(),
  },
}));

vi.mock("../../shared/api", () => ({
  appWindow: {
    closeBrowse: mocks.closeBrowse,
    minimise: mocks.minimise,
    setBrowseSize: mocks.setBrowseSize,
    size: mocks.size,
  },
  audio: mocks.audio,
  onBrowseNavigate: mocks.onBrowseNavigate,
  visualizer: { setActive: vi.fn(() => Promise.resolve()) },
  onVisualizerFrame: vi.fn(() => () => {}),
  toApiError: (error: unknown) => (error instanceof Error ? error : new Error(String(error))),
}));

vi.mock("../../lib/music.svelte", () => ({ music: mocks.music }));

vi.mock("../../lib/player.svelte", async () => {
  const { playerMock: player } = await import("../../lib/testing/player-mock.svelte");
  return { player };
});

vi.mock("../../lib/skins.svelte", () => ({
  skins: {
    runtime: mocks.runtime,
    catalog: { activeId: "clarity", activeTheme: "graphite" },
    theme: "graphite",
    loading: mocks.loading,
    error: null,
  },
}));

import BrowseApp from "./BrowseApp.svelte";
import { browse } from "./browse.svelte";
import { browseSettings } from "./settings.svelte";

const engineSettings = {
  bitrate: 320,
  crossfadeDurationMs: 0,
  normalisationDisabled: false,
  normalisationUseAlbumGain: false,
  normalisationPregain: 0,
  cacheEnabled: true,
  cacheDir: "/tmp/cadence-cache",
  cacheSizeLimit: "1GB",
  flacEnabled: false,
  mprisEnabled: true,
  deviceIsGroup: false,
  imageSize: "default",
};

describe("Browse shell", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.audio.isLiked.mockResolvedValue(false);
    mocks.audio.getEngineSettings.mockResolvedValue(engineSettings);
    mocks.audio.playlists.mockResolvedValue({ total: 0, offset: 0, limit: 50, items: [] });
    mocks.audio.albums.mockResolvedValue({ total: 0, offset: 0, limit: 50, items: [] });
    mocks.audio.artists.mockResolvedValue({ total: 0, offset: 0, limit: 50, items: [] });
    mocks.audio.search.mockResolvedValue({
      tracks: { hits: [], total: 0 },
      albums: { hits: [], total: 0 },
      artists: { hits: [], total: 0 },
      playlists: { hits: [], total: 0 },
      shows: { hits: [], total: 0 },
      episodes: { hits: [], total: 0 },
    });
    mocks.audio.yourEpisodes.mockResolvedValue({
      uri: "",
      ready: true,
      length: 0,
      cached: 0,
      items: [],
    });
    mocks.navigateHandler = null;
    mocks.onBrowseNavigate.mockImplementation((handler: (section: string) => void) => {
      mocks.navigateHandler = handler;
      return () => {};
    });
    mocks.music.authenticated = true;
    mocks.music.pairing = null;
    browse.reset();
    browseSettings.reset();
    preferences.setTheme("graphite");
    playerMock.state = { ...playerMock.state, canControl: true };
    playerMock.toggle.mockClear();
    playerMock.previous.mockClear();
    playerMock.next.mockClear();
    playerMock.setVolume.mockClear();
    playerMock.loadQueue.mockClear();
    playerMock.queue = [];
    playerMock.queueLoading = false;
  });

  it("renders the sidebar, search field, theme label and now-playing bar", () => {
    render(BrowseApp);

    expect(screen.getByRole("navigation", { name: "Browse sections" })).not.toBeNull();
    expect(screen.getByRole("button", { name: "Search" })).not.toBeNull();
    expect(screen.getByRole("button", { name: "Library" })).not.toBeNull();
    expect(screen.getAllByRole("button", { name: "Podcasts" }).length).toBeGreaterThan(0);
    expect(screen.getByRole("button", { name: "Queue" })).not.toBeNull();
    expect(screen.getByRole("button", { name: "Settings" })).not.toBeNull();
    expect(
      screen.getByRole("searchbox", {
        name: "Search tracks, albums, artists, playlists, shows and episodes",
      }),
    ).not.toBeNull();

    expect(screen.getByText("Clarity · graphite")).not.toBeNull();
    expect(screen.getByText("Neon Meridian")).not.toBeNull();
    expect(screen.getByText("Aster Vale")).not.toBeNull();
    expect(screen.getByRole("button", { name: "Pause" })).not.toBeNull();
    expect(screen.getByRole("slider", { name: "Volume level" })).not.toBeNull();
  });

  it("opens the queue section and loads the queue", async () => {
    render(BrowseApp);

    await fireEvent.click(screen.getByRole("button", { name: "Queue" }));

    expect(screen.getByRole("heading", { name: "Playback queue" })).not.toBeNull();
    await waitFor(() => expect(playerMock.loadQueue).toHaveBeenCalled());
  });

  it("minimises and closes from the custom window controls", async () => {
    render(BrowseApp);

    await fireEvent.click(screen.getByRole("button", { name: "Minimise window" }));
    expect(mocks.minimise).toHaveBeenCalledOnce();

    await fireEvent.click(screen.getByRole("button", { name: "Close window" }));
    await waitFor(() => expect(mocks.closeBrowse).toHaveBeenCalledOnce());
  });

  it("marks the topbar as draggable and interactive chrome as no-drag", () => {
    const { container } = render(BrowseApp);

    expect(container.querySelector(".topbar")?.classList.contains("drag-region")).toBe(true);
    expect(container.querySelector(".search-field")?.classList.contains("no-drag")).toBe(true);
    expect(container.querySelector(".window-controls")?.classList.contains("no-drag")).toBe(true);
    expect(container.querySelector(".resize-grip")?.classList.contains("no-drag")).toBe(true);
  });

  it("clamps the custom resize grip to the minimum size", () => {
    expect(clampBrowseSize(960, 640, -400, -300)).toEqual({ width: 720, height: 480 });
    expect(clampBrowseSize(960, 640, 40, 20)).toEqual({ width: 1000, height: 660 });
  });

  it("switches to settings and forwards transport controls", async () => {
    render(BrowseApp);

    await fireEvent.click(screen.getByRole("button", { name: "Settings" }));
    expect(screen.getByRole("heading", { name: "Settings" })).not.toBeNull();
    expect(screen.getByRole("button", { name: "Save settings" })).not.toBeNull();

    await fireEvent.click(screen.getByRole("button", { name: "Pause" }));
    expect(playerMock.toggle).toHaveBeenCalledOnce();

    await fireEvent.click(screen.getByRole("button", { name: "Previous" }));
    await fireEvent.click(screen.getByRole("button", { name: "Next" }));
    expect(playerMock.previous).toHaveBeenCalledOnce();
    expect(playerMock.next).toHaveBeenCalledOnce();
  });

  it("focuses the entity back button and returns focus to the view heading", async () => {
    const entity = { kind: "album" as const, uri: "spotify:album:1", title: "Album" };
    browse.stack = [{ name: "root" }, { name: "entity", entity }];
    browse.entity = entity;
    browse.context = { uri: entity.uri, ready: true, length: 0, cached: 0, items: [] };

    render(BrowseApp);
    const back = screen.getByRole("button", { name: "Back" });
    await waitFor(() => expect(document.activeElement).toBe(back));

    await fireEvent.click(back);
    await waitFor(() =>
      expect(document.activeElement).toBe(
        screen.getByRole("heading", { name: "Search the catalog" }),
      ),
    );
  });

  it("uses the initial section and follows browse:navigate events", async () => {
    browse.setInitialSection("library");
    render(BrowseApp);

    await waitFor(() =>
      expect(screen.getByRole("heading", { name: "Your library" })).not.toBeNull(),
    );

    mocks.navigateHandler?.("podcasts");
    await tick();

    expect(browse.section).toBe("podcasts");
    await waitFor(() =>
      expect(screen.getByRole("heading", { name: "Your Episodes" })).not.toBeNull(),
    );
  });

  it("shows the engine gate for catalog sections but keeps settings reachable", async () => {
    mocks.music.authenticated = false;
    mocks.music.pairing = { code: "ABCD-1234", url: "https://spotify.example/pair" };

    render(BrowseApp);
    expect(screen.getByRole("heading", { name: "Pair Cadence with Spotify" })).not.toBeNull();

    await fireEvent.click(screen.getByRole("button", { name: "Settings" }));
    expect(screen.getByRole("heading", { name: "Settings" })).not.toBeNull();
    expect(screen.queryByText("Pair Cadence with Spotify")).toBeNull();
  });

  it("closes Browse from the back-to-player action", async () => {
    render(BrowseApp);

    await fireEvent.click(screen.getByRole("button", { name: "Back to player" }));

    await waitFor(() => expect(mocks.closeBrowse).toHaveBeenCalledOnce());
  });

  it("focuses the search field when switching back to Search", async () => {
    render(BrowseApp);

    await fireEvent.click(screen.getByRole("button", { name: "Library" }));
    await fireEvent.click(screen.getByRole("button", { name: "Search" }));

    await waitFor(() =>
      expect(document.activeElement).toBe(
        screen.getByRole("searchbox", {
          name: "Search tracks, albums, artists, playlists, shows and episodes",
        }),
      ),
    );
  });
});
