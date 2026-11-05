import { fireEvent, render, screen, waitFor } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { SearchResults } from "../../shared/api/types";
import { browse, type DiscoverState, type EntityRef, type LibraryEntry } from "./browse.svelte";
import DiscoverView from "./DiscoverView.svelte";

const mocks = vi.hoisted(() => ({
  audio: {
    search: vi.fn(),
    playlists: vi.fn(),
    albums: vi.fn(),
    artists: vi.fn(),
    yourEpisodes: vi.fn(),
    contextTracks: vi.fn(),
    playContext: vi.fn(),
    playTrack: vi.fn(),
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

function entry(
  kind: LibraryEntry["kind"],
  uri: string,
  title: string,
  overrides: Partial<LibraryEntry> = {},
): LibraryEntry {
  return {
    kind,
    uri,
    title,
    image: "",
    owner: "",
    artists: [],
    year: 0,
    length: 0,
    durationMs: 0,
    canEdit: false,
    showUri: "",
    ...overrides,
  };
}

function loadedDiscover(overrides: Partial<DiscoverState> = {}): DiscoverState {
  return {
    playlists: [],
    albums: [],
    artists: [],
    episodes: [],
    loading: false,
    loaded: true,
    error: null,
    ...overrides,
  };
}

function resetStore(): void {
  localStorage.clear();
  browse.reset();
  browse.recentEntities = [];
  browse.recentSearches = [];
  browse.discover = loadedDiscover();
}

describe("DiscoverView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    resetStore();
    mocks.audio.search.mockResolvedValue(emptyResults());
    mocks.audio.contextTracks.mockResolvedValue({
      uri: "spotify:playlist:1",
      ready: true,
      length: 0,
      cached: 0,
      items: [],
    });
    mocks.audio.playContext.mockResolvedValue(undefined);
    mocks.audio.playTrack.mockResolvedValue(undefined);
  });

  it("renders favorites, recent entities, library cards, genres and recent searches", () => {
    browse.discover = loadedDiscover({
      playlists: [entry("playlist", "spotify:playlist:1", "Night Drive", { owner: "Ivan" })],
      albums: [entry("album", "spotify:album:1", "Discovery", { artists: ["Daft Punk"] })],
      artists: [entry("artist", "spotify:artist:1", "Daft Punk")],
    });
    browse.recentEntities = [
      { kind: "album", uri: "spotify:album:9", title: "Random Access Memories" },
    ];
    browse.recentSearches = ["daft punk"];

    render(DiscoverView);

    expect(
      screen.getByRole("heading", { level: 2, name: /Good (morning|afternoon|evening)/ }),
    ).not.toBeNull();
    expect(screen.getByText("Your favorites")).not.toBeNull();
    expect(screen.getByText("Liked Songs")).not.toBeNull();
    expect(screen.getByText("Your Episodes")).not.toBeNull();
    expect(screen.getByText("Recently played")).not.toBeNull();
    expect(screen.getByText("Random Access Memories")).not.toBeNull();
    expect(screen.getByText("From your library")).not.toBeNull();
    expect(screen.getByText("Night Drive")).not.toBeNull();
    expect(screen.getByText("Discovery")).not.toBeNull();
    expect(screen.getByText("Podcasts for you")).not.toBeNull();
    expect(screen.getByText("Browse by genre")).not.toBeNull();
    expect(screen.getByRole("button", { name: "Jazz" })).not.toBeNull();
    expect(screen.getByText("Recent searches")).not.toBeNull();
    expect(screen.getByRole("button", { name: "daft punk" })).not.toBeNull();
  });

  it("searches a music genre in the tracks tab and records it", async () => {
    render(DiscoverView);

    await fireEvent.click(screen.getByRole("button", { name: "Jazz" }));

    await waitFor(() => expect(mocks.audio.search).toHaveBeenCalledWith("Jazz", 30));
    expect(browse.searchTab).toBe("tracks");
    expect(browse.query).toBe("Jazz");
    expect(browse.recentSearches[0]).toBe("Jazz");
  });

  it("searches the podcast genre in the shows tab", async () => {
    render(DiscoverView);

    await fireEvent.click(screen.getByRole("button", { name: "Podcasts" }));

    await waitFor(() => expect(mocks.audio.search).toHaveBeenCalledWith("Podcast", 30));
    expect(browse.searchTab).toBe("shows");
  });

  it("re-runs a recent search from its chip", async () => {
    browse.recentSearches = ["daft punk"];
    render(DiscoverView);

    await fireEvent.click(screen.getByRole("button", { name: "daft punk" }));

    await waitFor(() => expect(mocks.audio.search).toHaveBeenCalledWith("daft punk", 30));
    expect(browse.query).toBe("daft punk");
  });

  it("shows library and podcast CTAs when the slices are empty", async () => {
    const input = document.createElement("input");
    input.id = "browse-search-input";
    document.body.appendChild(input);

    render(DiscoverView);

    expect(screen.getByText(/Your library is empty/)).not.toBeNull();
    expect(screen.getByText(/No saved episodes yet/)).not.toBeNull();

    await fireEvent.click(screen.getByRole("button", { name: "Search the catalog" }));
    expect(document.activeElement).toBe(input);

    await fireEvent.click(screen.getByRole("button", { name: "Search podcasts" }));
    await waitFor(() => expect(mocks.audio.search).toHaveBeenCalledWith("Podcast", 30));
    expect(browse.searchTab).toBe("shows");

    input.remove();
  });

  it("hides recent sections when there are no recents", () => {
    render(DiscoverView);

    expect(screen.queryByText("Recently played")).toBeNull();
    expect(screen.queryByText("Recent searches")).toBeNull();
  });

  it("opens a recent entity", async () => {
    const ref: EntityRef = { kind: "playlist", uri: "spotify:playlist:9", title: "Saved Mix" };
    browse.recentEntities = [ref];
    render(DiscoverView);

    await fireEvent.click(screen.getByRole("button", { name: /Saved Mix/ }));

    await waitFor(() => expect(mocks.audio.contextTracks).toHaveBeenCalledWith(ref.uri));
    expect(browse.entity?.uri).toBe(ref.uri);
  });

  it("shows a retry banner when the discover slice fails", () => {
    browse.discover = {
      ...loadedDiscover({ loaded: false }),
      error: "engine offline",
    };

    render(DiscoverView);

    expect(screen.getByRole("alert").textContent).toContain("Could not load your library.");
    expect(screen.getByRole("button", { name: "Retry" })).not.toBeNull();
  });
});
