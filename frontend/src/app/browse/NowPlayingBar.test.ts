import { fireEvent, render, screen, waitFor } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { browse } from "./browse.svelte";
import NowPlayingBar from "./NowPlayingBar.svelte";

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

describe("NowPlayingBar", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    browse.reset();
    mocks.audio.setLiked.mockResolvedValue(undefined);
  });

  it("likes and unlikes the current track", async () => {
    render(NowPlayingBar);

    await fireEvent.click(screen.getByRole("button", { name: "Like this track" }));
    await waitFor(() => expect(mocks.audio.setLiked).toHaveBeenCalledWith("spotify:track:1", true));
    expect(browse.currentLiked).toBe(true);

    await fireEvent.click(
      screen.getByRole("button", { name: "Remove this track from Liked Songs" }),
    );
    await waitFor(() =>
      expect(mocks.audio.setLiked).toHaveBeenLastCalledWith("spotify:track:1", false),
    );
  });
});
