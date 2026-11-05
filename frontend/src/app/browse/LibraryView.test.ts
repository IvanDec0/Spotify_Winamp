import { fireEvent, render, screen, waitFor } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { LibraryEntry } from "./browse.svelte";
import { browse } from "./browse.svelte";
import LibraryView from "./LibraryView.svelte";

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

function entry(uri: string, title: string): LibraryEntry {
  return {
    kind: "playlist",
    uri,
    title,
    image: "",
    owner: "Owner",
    artists: [],
    year: 0,
    length: 3,
    durationMs: 0,
    canEdit: false,
    showUri: "",
  };
}

describe("LibraryView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    browse.reset();
    browse.libraryTab = "playlists";
    browse.pages.playlists = {
      items: [entry("spotify:playlist:1", "First")],
      total: 2,
      offset: 1,
      loading: false,
      loaded: true,
    };
    mocks.audio.playlists.mockResolvedValue({
      total: 2,
      offset: 1,
      limit: 50,
      items: [
        {
          uri: "spotify:playlist:2",
          name: "Second",
          description: "",
          owner: "Owner",
          length: 4,
          imageUrl: "",
          collaborative: false,
          canEdit: false,
          folder: null,
        },
      ],
    });
  });

  it("renders the Liked Songs card and loads the next page", async () => {
    render(LibraryView);

    expect(screen.getByRole("button", { name: /Liked Songs/ })).not.toBeNull();
    expect(screen.getByText("First")).not.toBeNull();

    await fireEvent.click(screen.getByRole("button", { name: "Load more" }));
    await waitFor(() => expect(screen.getByText("Second")).not.toBeNull());

    expect(mocks.audio.playlists).toHaveBeenCalledWith(1, 50);
    expect(browse.pages.playlists.items).toHaveLength(2);
  });
});
