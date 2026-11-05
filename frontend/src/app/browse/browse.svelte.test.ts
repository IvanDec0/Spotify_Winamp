import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { LibraryContext, LibraryPlaylist, SearchResults } from "../../shared/api/types";
import { BrowseStore, normalizeSection, type EntityRef } from "./browse.svelte";

const mocks = vi.hoisted(() => ({
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
}));

vi.mock("../../shared/api", () => ({
  audio: mocks,
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

function hit(uri: string, name: string) {
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
  };
}

function playlist(uri: string): LibraryPlaylist {
  return {
    uri,
    name: uri,
    description: "",
    owner: "Owner",
    length: 1,
    imageUrl: `/art/${uri}`,
    collaborative: false,
    canEdit: false,
    folder: null,
  };
}

function context(ready: boolean, count: number): LibraryContext {
  const items = Array.from({ length: count }, (_, index) => ({
    uri: `spotify:track:${index + 1}`,
    title: `Track ${index + 1}`,
    artists: ["Artist"],
    album: "Album",
    cover: `/art/${index + 1}`,
    durationMs: 180_000,
  }));
  return { uri: "spotify:album:1", ready, length: count, cached: count, items };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

const albumRef: EntityRef = {
  kind: "album",
  uri: "spotify:album:1",
  title: "Album",
};

describe("BrowseStore", () => {
  let store: BrowseStore;

  beforeEach(() => {
    vi.clearAllMocks();
    const emptyPage = { total: 0, offset: 0, limit: 50, items: [] };
    mocks.playlists.mockResolvedValue(emptyPage);
    mocks.albums.mockResolvedValue(emptyPage);
    mocks.artists.mockResolvedValue(emptyPage);
    mocks.contextTracks.mockResolvedValue(context(true, 0));
    mocks.yourEpisodes.mockResolvedValue(context(true, 0));
    mocks.isAlbumSaved.mockResolvedValue(false);
    mocks.isShowSaved.mockResolvedValue(false);
    mocks.isArtistFollowed.mockResolvedValue(false);
    mocks.isLiked.mockResolvedValue(false);
    store = new BrowseStore();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("accepts a validated initial section", () => {
    expect(normalizeSection("podcasts")).toBe("podcasts");
    expect(normalizeSection("bogus")).toBe("library");
    expect(normalizeSection(null)).toBe("library");
    expect(normalizeSection("settings", "search")).toBe("settings");

    store.setInitialSection("library");
    expect(store.section).toBe("library");
    expect(store.route.name).toBe("root");
  });

  it("keeps the latest search response and maps every group", async () => {
    const first = deferred<SearchResults>();
    const second = deferred<SearchResults>();
    mocks.search.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise);

    const firstRun = store.search("first");
    const secondRun = store.search("second");

    const secondResults = emptyResults();
    secondResults.tracks.hits = [hit("spotify:track:2", "Second")];
    secondResults.shows.hits = [hit("spotify:show:2", "Second Show")];
    second.resolve(secondResults);
    await secondRun;

    const firstResults = emptyResults();
    firstResults.tracks.hits = [hit("spotify:track:1", "First")];
    first.resolve(firstResults);
    await firstRun;

    expect(store.searchSequence).toBe(2);
    expect(store.results?.tracks).toEqual([secondResults.tracks.hits[0]]);
    expect(store.results?.shows).toEqual([secondResults.shows.hits[0]]);
    expect(store.results?.episodes).toEqual([]);
  });

  it("appends library pages and keeps the total", async () => {
    mocks.playlists
      .mockResolvedValueOnce({ total: 2, offset: 0, limit: 50, items: [playlist("one")] })
      .mockResolvedValueOnce({ total: 2, offset: 1, limit: 50, items: [playlist("two")] });

    await store.loadLibrary("playlists");
    expect(store.pages.playlists.items.map((item) => item.uri)).toEqual(["one"]);
    expect(store.pages.playlists.total).toBe(2);

    await store.loadMore("playlists");
    expect(store.pages.playlists.items.map((item) => item.uri)).toEqual(["one", "two"]);
    expect(mocks.playlists).toHaveBeenNthCalledWith(2, 1, 50);
    expect(store.pages.playlists.offset).toBe(2);
  });

  it("retries an unready entity context until it is ready", async () => {
    vi.useFakeTimers();
    mocks.contextTracks
      .mockResolvedValueOnce(context(false, 0))
      .mockResolvedValueOnce(context(true, 1));
    mocks.isAlbumSaved.mockResolvedValue(false);

    await store.openEntity(albumRef);
    expect(mocks.contextTracks).toHaveBeenCalledTimes(1);
    expect(store.context?.ready).toBe(false);
    expect(store.contextLoading).toBe(false);

    await vi.advanceTimersByTimeAsync(1000);
    expect(mocks.contextTracks).toHaveBeenCalledTimes(2);
    expect(store.context?.ready).toBe(true);
    expect(store.context?.items).toHaveLength(1);
  });

  it("cancels context retries when the route changes", async () => {
    vi.useFakeTimers();
    mocks.contextTracks.mockResolvedValue(context(false, 0));

    await store.openEntity(albumRef);
    store.setSection("library");
    await vi.advanceTimersByTimeAsync(2500);

    expect(mocks.contextTracks).toHaveBeenCalledTimes(1);
    expect(store.entity).toBeNull();
    expect(store.stack).toHaveLength(1);
  });

  it("toggles album save and artist follow states optimistically", async () => {
    mocks.isAlbumSaved.mockResolvedValue(false);
    mocks.saveAlbum.mockResolvedValue(undefined);

    await store.openEntity(albumRef);
    expect(store.entitySaved).toBe(false);
    await store.toggleSave();
    expect(mocks.saveAlbum).toHaveBeenCalledWith("spotify:album:1", true);
    expect(store.entitySaved).toBe(true);
    expect(store.notice?.key).toBe("browse.noticeSaved");

    mocks.isArtistFollowed.mockResolvedValue(false);
    mocks.followArtist.mockResolvedValue(undefined);
    await store.openEntity({ kind: "artist", uri: "spotify:artist:1", title: "Artist" });
    await store.toggleSave();
    expect(mocks.followArtist).toHaveBeenCalledWith("spotify:artist:1", true);
    expect(store.notice?.key).toBe("browse.noticeFollowed");
  });

  it("rolls the save state back when the write fails", async () => {
    mocks.isAlbumSaved.mockResolvedValue(false);
    mocks.saveAlbum.mockRejectedValue(new Error("engine offline"));

    await store.openEntity(albumRef);
    await store.toggleSave();

    expect(store.entitySaved).toBe(false);
    expect(store.error).toBe("engine offline");
  });

  it("routes search hits to playback or entity pages", async () => {
    mocks.playTrack.mockResolvedValue(undefined);
    mocks.contextTracks.mockResolvedValue(context(true, 0));
    mocks.isAlbumSaved.mockResolvedValue(false);

    await store.playSearchHit(hit("spotify:track:1", "Track"), "tracks");
    expect(mocks.playTrack).toHaveBeenCalledWith("spotify:track:1");

    await store.playSearchHit(hit("spotify:album:1", "Album"), "albums");
    expect(mocks.contextTracks).toHaveBeenCalledWith("spotify:album:1");
    expect(store.entity?.kind).toBe("album");
    expect(store.entity?.title).toBe("Album");
  });

  it("adds to and removes from an editable playlist", async () => {
    mocks.addToPlaylist.mockResolvedValue(undefined);
    await store.addToPlaylist("spotify:track:1", "spotify:playlist:9");
    expect(mocks.addToPlaylist).toHaveBeenCalledWith("spotify:playlist:9", "spotify:track:1");
    expect(store.notice?.key).toBe("browse.noticeAddedToPlaylist");

    mocks.contextTracks.mockResolvedValue(context(true, 2));
    await store.openEntity({
      kind: "playlist",
      uri: "spotify:playlist:9",
      title: "Editable",
      canEdit: true,
    });
    mocks.removeFromPlaylist.mockResolvedValue(undefined);
    await store.removeFromPlaylist("spotify:track:1");

    expect(mocks.removeFromPlaylist).toHaveBeenCalledWith("spotify:playlist:9", "spotify:track:1");
    expect(store.context?.items.map((item) => item.uri)).toEqual(["spotify:track:2"]);
    expect(store.notice?.key).toBe("browse.noticeRemovedFromPlaylist");
  });
});
