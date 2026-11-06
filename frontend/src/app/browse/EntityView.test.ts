import { fireEvent, render, screen, waitFor } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { LibraryContext } from "../../shared/api/types";
import { browse } from "./browse.svelte";
import EntityView from "./EntityView.svelte";

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

function context(uri: string, count = 2): LibraryContext {
  return {
    uri,
    ready: true,
    length: count,
    cached: count,
    items: Array.from({ length: count }, (_, index) => ({
      uri: `spotify:track:${index + 1}`,
      title: `Track ${index + 1}`,
      artists: ["Artist"],
      album: "Album",
      cover: `/art/${index + 1}`,
      durationMs: 190_000,
    })),
  };
}

let originalClientHeight: PropertyDescriptor | undefined;

describe("EntityView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    browse.reset();
    browse.entity = { kind: "album", uri: "spotify:album:1", title: "Album" };
    browse.context = context("spotify:album:1") as NonNullable<typeof browse.context>;
    browse.entitySaved = false;
    mocks.audio.saveAlbum.mockResolvedValue(undefined);
    mocks.audio.playContext.mockResolvedValue(undefined);
    mocks.audio.playlists.mockResolvedValue({
      total: 1,
      offset: 0,
      limit: 100,
      items: [
        {
          uri: "spotify:playlist:9",
          name: "Editable",
          description: "",
          owner: "Owner",
          length: 2,
          imageUrl: "",
          collaborative: false,
          canEdit: true,
          folder: null,
        },
      ],
    });
    mocks.audio.addToPlaylist.mockResolvedValue(undefined);
    mocks.music.audio.playbackBackend = "librespot";

    originalClientHeight = Object.getOwnPropertyDescriptor(HTMLElement.prototype, "clientHeight");
    Object.defineProperty(HTMLElement.prototype, "clientHeight", {
      configurable: true,
      get: () => 120,
    });
  });

  afterEach(() => {
    if (originalClientHeight) {
      Object.defineProperty(HTMLElement.prototype, "clientHeight", originalClientHeight);
    } else {
      delete (HTMLElement.prototype as { clientHeight?: number }).clientHeight;
    }
  });

  it("renders the header, actions and virtualized rows", async () => {
    render(EntityView);

    expect(screen.getByRole("heading", { name: "Album" })).not.toBeNull();
    expect(screen.getByRole("button", { name: "Play all" })).not.toBeNull();
    expect(screen.getByRole("button", { name: "Save" })).not.toBeNull();
    await waitFor(() =>
      expect(document.activeElement).toBe(screen.getByRole("button", { name: "Back" })),
    );
    await waitFor(() => expect(screen.getByText("Track 1")).not.toBeNull());
    expect(screen.getByText("Track 2")).not.toBeNull();

    await fireEvent.click(screen.getByRole("button", { name: "Play all" }));
    expect(mocks.audio.playContext).toHaveBeenCalledWith("spotify:album:1", "");

    await fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() =>
      expect(mocks.audio.saveAlbum).toHaveBeenCalledWith("spotify:album:1", true),
    );
  });

  it("renders only the virtual window for a long context", async () => {
    browse.context = context("spotify:album:1", 100) as NonNullable<typeof browse.context>;

    render(EntityView);
    await waitFor(() => expect(screen.getByText("Track 1")).not.toBeNull());
    expect(screen.queryByText("Track 50")).toBeNull();
  });

  it("adds a row to an editable playlist through the picker", async () => {
    render(EntityView);
    await waitFor(() => expect(screen.getByText("Track 1")).not.toBeNull());

    await fireEvent.click(screen.getByRole("button", { name: "Add Track 1 to a playlist" }));
    expect(screen.getByRole("dialog", { name: "Add to playlist" })).not.toBeNull();

    await waitFor(() => expect(screen.getByRole("button", { name: /Editable/ })).not.toBeNull());
    await fireEvent.click(screen.getByRole("button", { name: /Editable/ }));

    await waitFor(() =>
      expect(mocks.audio.addToPlaylist).toHaveBeenCalledWith(
        "spotify:playlist:9",
        "spotify:track:1",
      ),
    );
  });

  it("hides the radio action in Soloist mode", () => {
    mocks.music.audio.playbackBackend = "soloist";

    render(EntityView);

    expect(screen.queryByRole("button", { name: "Radio" })).toBeNull();
  });

  it("offers the radio action with the go-librespot backend", () => {
    render(EntityView);

    expect(screen.getByRole("button", { name: "Radio" })).not.toBeNull();
  });
});
