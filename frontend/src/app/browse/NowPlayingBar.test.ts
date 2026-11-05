import { fireEvent, render, screen, waitFor } from "@testing-library/svelte";
import { tick } from "svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { RepeatMode } from "../../shared/api/types";
import { createPlayerState, playerMock } from "../../lib/testing/player-mock.svelte";
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
  setVisualizerActive: vi.fn(() => Promise.resolve()),
  onVisualizerFrame: vi.fn(() => () => {}),
}));

vi.mock("../../shared/api", () => ({
  audio: mocks.audio,
  visualizer: { setActive: mocks.setVisualizerActive },
  onVisualizerFrame: mocks.onVisualizerFrame,
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
    playerMock.state = createPlayerState();
    playerMock.toggle.mockClear();
    playerMock.previous.mockClear();
    playerMock.next.mockClear();
    playerMock.seek.mockClear();
    playerMock.toggleShuffle.mockClear();
    playerMock.cycleRepeat.mockClear();
    playerMock.setVolume.mockClear();
    playerMock.setVolume.mockImplementation((volume: number) => {
      playerMock.state = { ...playerMock.state, volume };
      return Promise.resolve();
    });
    mocks.audio.setLiked.mockResolvedValue(undefined);
  });

  it("toggles shuffle and cycles repeat with active state", async () => {
    render(NowPlayingBar);

    await fireEvent.click(screen.getByRole("button", { name: "Turn shuffle on" }));
    expect(playerMock.toggleShuffle).toHaveBeenCalledOnce();

    await fireEvent.click(screen.getByRole("button", { name: "Repeat off" }));
    expect(playerMock.cycleRepeat).toHaveBeenCalledOnce();

    playerMock.state = {
      ...playerMock.state,
      shuffle: true,
      repeat: RepeatMode.RepeatTrack,
    };
    await tick();

    expect(
      screen.getByRole("button", { name: "Turn shuffle off" }).getAttribute("aria-pressed"),
    ).toBe("true");
    expect(
      screen.getByRole("button", { name: "Repeat one track" }).getAttribute("aria-pressed"),
    ).toBe("true");
  });

  it("seeks with the progress range", async () => {
    render(NowPlayingBar);

    const slider = screen.getByRole("slider", { name: "Playback position" });
    expect(slider.getAttribute("aria-valuetext")).toBe("0:38 of 4:12");

    await fireEvent.input(slider, { target: { value: "90000" } });
    await fireEvent.change(slider, { target: { value: "90000" } });

    expect(playerMock.seek).toHaveBeenCalledWith(90000);
  });

  it("mutes and restores the previous volume", async () => {
    render(NowPlayingBar);

    await fireEvent.click(screen.getByRole("button", { name: "Mute" }));
    expect(playerMock.setVolume).toHaveBeenLastCalledWith(0);

    const unmute = await screen.findByRole("button", { name: "Unmute" });
    await fireEvent.click(unmute);
    expect(playerMock.setVolume).toHaveBeenLastCalledWith(64);
  });

  it("disables transport, seek and volume controls", () => {
    playerMock.state = { ...createPlayerState(), canControl: false, canSeek: false };
    render(NowPlayingBar);

    expect(
      (screen.getByRole("button", { name: "Turn shuffle on" }) as HTMLButtonElement).disabled,
    ).toBe(true);
    expect((screen.getByRole("button", { name: "Repeat off" }) as HTMLButtonElement).disabled).toBe(
      true,
    );
    expect((screen.getByRole("button", { name: "Mute" }) as HTMLButtonElement).disabled).toBe(true);
    expect(
      (screen.getByRole("slider", { name: "Playback position" }) as HTMLInputElement).disabled,
    ).toBe(true);
    expect(
      (screen.getByRole("slider", { name: "Volume level" }) as HTMLInputElement).disabled,
    ).toBe(true);
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
