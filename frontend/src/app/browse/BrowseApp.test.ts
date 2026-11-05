import { fireEvent, render, screen } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { playerMock } from "../../lib/testing/player-mock.svelte";
import { preferences } from "../../lib/preferences.svelte";

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
  },
}));

vi.mock("../../shared/api", () => ({
  audio: mocks.audio,
  toApiError: (error: unknown) => (error instanceof Error ? error : new Error(String(error))),
}));

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

describe("Browse shell", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.audio.isLiked.mockResolvedValue(false);
    browse.reset();
    preferences.setTheme("graphite");
    playerMock.state = { ...playerMock.state, canControl: true };
    playerMock.toggle.mockClear();
    playerMock.previous.mockClear();
    playerMock.next.mockClear();
    playerMock.setVolume.mockClear();
  });

  it("renders the sidebar, search field, theme label and now-playing bar", () => {
    render(BrowseApp);

    expect(screen.getByRole("navigation", { name: "Browse sections" })).not.toBeNull();
    expect(screen.getByRole("button", { name: "Search" })).not.toBeNull();
    expect(screen.getByRole("button", { name: "Library" })).not.toBeNull();
    expect(screen.getByRole("button", { name: "Podcasts" })).not.toBeNull();
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
    expect(screen.getByRole("slider", { name: "Volume" })).not.toBeNull();
  });

  it("switches to settings and forwards transport controls", async () => {
    render(BrowseApp);

    await fireEvent.click(screen.getByRole("button", { name: "Settings" }));
    expect(screen.getByRole("heading", { name: "Settings" })).not.toBeNull();

    await fireEvent.click(screen.getByRole("button", { name: "Pause" }));
    expect(playerMock.toggle).toHaveBeenCalledOnce();

    await fireEvent.click(screen.getByRole("button", { name: "Previous" }));
    await fireEvent.click(screen.getByRole("button", { name: "Next" }));
    expect(playerMock.previous).toHaveBeenCalledOnce();
    expect(playerMock.next).toHaveBeenCalledOnce();
  });
});
