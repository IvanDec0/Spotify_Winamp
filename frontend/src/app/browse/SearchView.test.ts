import { fireEvent, render, screen } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { SearchHit } from "../../shared/api/types";
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
}));

vi.mock("../../shared/api", () => ({
  audio: mocks.audio,
  toApiError: (error: unknown) => (error instanceof Error ? error : new Error(String(error))),
}));

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
    browse.reset();
    mocks.audio.playlists.mockResolvedValue({ total: 0, offset: 0, limit: 100, items: [] });
    mocks.audio.isLiked.mockResolvedValue(false);
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
    expect(screen.getByRole("button", { name: /Tracks/ }).textContent).toContain("1");
    expect(screen.getByText("One More Time")).not.toBeNull();
    expect(screen.getByText("Daft Punk")).not.toBeNull();

    await fireEvent.click(screen.getByRole("button", { name: /Albums/ }));
    expect(screen.getByText("Discovery")).not.toBeNull();

    await fireEvent.click(screen.getByRole("button", { name: /Shows/ }));
    expect(screen.getByText("Podcast Show")).not.toBeNull();
    expect(screen.getByText("Podcast")).not.toBeNull();
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
});
