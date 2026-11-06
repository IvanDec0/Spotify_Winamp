import { fireEvent, render, screen, waitFor } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { SearchHit, SearchResults } from "../../shared/api/types";
import type { SearchResultsView } from "./browse.svelte";
import { browse } from "./browse.svelte";
import SearchView from "./SearchView.svelte";

const mocks = vi.hoisted(() => ({
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
  music: {
    audio: { playbackBackend: "librespot" },
    get soloistMode(): boolean {
      return this.audio.playbackBackend === "soloist";
    },
  },
}));

vi.mock("../../shared/api", () => ({
  audio: mocks.audio,
  toApiError: (error: unknown) => (error instanceof Error ? error : new Error(String(error))),
}));

vi.mock("../../lib/music.svelte", () => ({ music: mocks.music }));

vi.mock("../../lib/player.svelte", async () => {
  const { playerMock } = await import("../../lib/testing/player-mock.svelte");
  return { player: playerMock };
});

function hit(uri: string, name: string, overrides: Partial<SearchHit> = {}): SearchHit {
  return {
    name,
    uri,
    image: `/art/${uri}`,
    artists: [],
    album: null,
    duration: 0,
    author: "",
    followersCount: 0,
    verified: false,
    showType: "",
    musicAndTalk: false,
    explicit: false,
    ...overrides,
  };
}

function emptyResults(): SearchResults {
  const group = () => ({ hits: [], total: 0 });
  return {
    tracks: group(),
    albums: group(),
    artists: group(),
    playlists: group(),
    shows: group(),
    episodes: group(),
  };
}

function emptyView(overrides: Partial<SearchResultsView> = {}): SearchResultsView {
  return {
    tracks: [],
    albums: [],
    artists: [],
    playlists: [],
    shows: [],
    episodes: [],
    ...overrides,
  };
}

describe("SearchView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    browse.reset();
    const emptyPage = { total: 0, offset: 0, limit: 50, items: [] };
    mocks.audio.search.mockResolvedValue(emptyResults());
    mocks.audio.playlists.mockResolvedValue(emptyPage);
    mocks.audio.albums.mockResolvedValue(emptyPage);
    mocks.audio.artists.mockResolvedValue(emptyPage);
    mocks.audio.yourEpisodes.mockResolvedValue({
      uri: "",
      ready: true,
      length: 0,
      cached: 0,
      items: [],
    });
    mocks.audio.isLiked.mockResolvedValue(false);
    mocks.music.audio.playbackBackend = "librespot";
  });

  it("renders Discover when the query is empty", async () => {
    render(SearchView);

    await waitFor(() => expect(screen.getByText("Your favorites")).not.toBeNull());
    expect(screen.getByText("Browse by genre")).not.toBeNull();
    expect(screen.queryByRole("tab")).toBeNull();
    expect(screen.queryByText("One More Time")).toBeNull();
  });

  it("shows tab counts and switches result groups", async () => {
    browse.query = "daft";
    browse.results = {
      tracks: [
        hit("spotify:track:1", "One More Time", { artists: [{ name: "Daft Punk", uri: "" }] }),
      ],
      albums: [hit("spotify:album:1", "Discovery")],
      artists: [],
      playlists: [],
      shows: [hit("spotify:show:1", "Podcast Show", { showType: "podcast" })],
      episodes: [],
    };

    render(SearchView);
    expect(screen.queryByText("Your favorites")).toBeNull();
    expect(screen.getByRole("tab", { name: /Tracks/ }).textContent).toContain("1");
    expect(screen.getByText("One More Time")).not.toBeNull();
    expect(screen.getByText("Daft Punk")).not.toBeNull();

    await fireEvent.click(screen.getByRole("tab", { name: /Albums/ }));
    expect(screen.getByText("Discovery")).not.toBeNull();

    await fireEvent.click(screen.getByRole("tab", { name: /Shows/ }));
    expect(screen.getByText("Podcast Show")).not.toBeNull();
    expect(screen.getByText("Podcast")).not.toBeNull();
  });

  it("moves the active tab with the keyboard", async () => {
    browse.query = "daft";
    browse.results = emptyView({
      tracks: [hit("spotify:track:1", "One More Time")],
      albums: [hit("spotify:album:1", "Discovery")],
    });

    render(SearchView);
    const first = screen.getAllByRole("tab")[0];
    expect(first.getAttribute("aria-selected")).toBe("true");

    await fireEvent.keyDown(first, { key: "ArrowRight" });
    expect(browse.searchTab).toBe("albums");
    await waitFor(() => {
      const tabs = screen.getAllByRole("tab");
      expect(tabs[1].getAttribute("aria-selected")).toBe("true");
      expect(document.activeElement).toBe(tabs[1]);
    });

    await fireEvent.keyDown(screen.getAllByRole("tab")[1], { key: "End" });
    expect(browse.searchTab).toBe("episodes");
    await fireEvent.keyDown(screen.getAllByRole("tab")[5], { key: "Home" });
    expect(browse.searchTab).toBe("tracks");
  });

  it("forwards track queue and radio actions", async () => {
    browse.query = "daft";
    browse.results = emptyView({
      tracks: [hit("spotify:track:1", "One More Time")],
    });
    mocks.audio.addToQueue.mockResolvedValue(undefined);
    mocks.audio.startRadio.mockResolvedValue(undefined);

    render(SearchView);
    await fireEvent.click(screen.getByRole("button", { name: "Add One More Time to the queue" }));
    await fireEvent.click(screen.getByRole("button", { name: "Start radio from One More Time" }));

    expect(mocks.audio.addToQueue).toHaveBeenCalledWith("spotify:track:1");
    expect(mocks.audio.startRadio).toHaveBeenCalledWith("spotify:track:1");
  });

  it("hides the track radio action in Soloist mode", () => {
    browse.query = "daft";
    browse.results = emptyView({
      tracks: [hit("spotify:track:1", "One More Time")],
    });
    mocks.music.audio.playbackBackend = "soloist";

    render(SearchView);

    expect(screen.queryByRole("button", { name: "Start radio from One More Time" })).toBeNull();
    expect(screen.getByRole("button", { name: "Add One More Time to the queue" })).not.toBeNull();
  });

  it("offers the track radio action with the go-librespot backend", () => {
    browse.query = "daft";
    browse.results = emptyView({
      tracks: [hit("spotify:track:1", "One More Time")],
    });

    render(SearchView);

    expect(screen.getByRole("button", { name: "Start radio from One More Time" })).not.toBeNull();
  });
});
