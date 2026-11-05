import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { LibraryContext, LibraryPlaylist, SearchResults } from "../../shared/api/types";
import { BrowseStore, normalizeSection, type EntityRef, type LibraryEntry } from "./browse.svelte";

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
    localStorage.clear();
    const emptyPage = { total: 0, offset: 0, limit: 50, items: [] };
    mocks.search.mockResolvedValue(emptyResults());
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

  it("persists recent entities with dedupe and a limit", () => {
    for (let index = 0; index < 10; index += 1) {
      store.recordEntity({
        kind: "album",
        uri: `spotify:album:${index}`,
        title: `Album ${index}`,
      });
    }

    expect(store.recentEntities).toHaveLength(8);
    expect(store.recentEntities[0].uri).toBe("spotify:album:9");
    expect(store.recentEntities[7].uri).toBe("spotify:album:2");

    store.recordEntity({ kind: "album", uri: "spotify:album:5", title: "Album 5 updated" });
    expect(store.recentEntities[0].title).toBe("Album 5 updated");
    expect(store.recentEntities).toHaveLength(8);

    const reloaded = new BrowseStore();
    expect(reloaded.recentEntities.map((entity) => entity.uri)).toEqual(
      store.recentEntities.map((entity) => entity.uri),
    );
    expect(JSON.parse(localStorage.getItem("cadence.browse.recentEntities") ?? "[]")).toHaveLength(
      8,
    );
  });

  it("tolerates corrupt recent entity storage", () => {
    localStorage.setItem("cadence.browse.recentEntities", "{not json");
    expect(new BrowseStore().recentEntities).toEqual([]);

    localStorage.setItem(
      "cadence.browse.recentEntities",
      JSON.stringify([
        { kind: "bogus", uri: "spotify:album:1", title: "Bad kind" },
        { kind: "album", uri: "", title: "No URI" },
        { kind: "album", uri: "spotify:album:2", title: "Valid" },
        { kind: "album", uri: "spotify:album:2", title: "Duplicate" },
      ]),
    );
    const loaded = new BrowseStore();
    expect(loaded.recentEntities).toEqual([
      { kind: "album", uri: "spotify:album:2", title: "Valid" },
    ]);
  });

  it("records searches only on explicit calls and keeps the newest six", async () => {
    await store.search("typed query");
    expect(store.recentSearches).toEqual([]);

    for (let index = 0; index < 7; index += 1) {
      store.recordSearch(`query ${index}`);
    }
    expect(store.recentSearches).toHaveLength(6);
    expect(store.recentSearches[0]).toBe("query 6");

    store.recordSearch("query 3");
    expect(store.recentSearches[0]).toBe("query 3");
    expect(store.recentSearches).toHaveLength(6);

    const reloaded = new BrowseStore();
    expect(reloaded.recentSearches).toEqual(store.recentSearches);

    store.clearRecentSearches();
    expect(store.recentSearches).toEqual([]);
    expect(JSON.parse(localStorage.getItem("cadence.browse.recentSearches") ?? "null")).toEqual([]);
  });

  it("loads and caches the discover library slice", async () => {
    mocks.playlists.mockResolvedValue({
      total: 1,
      offset: 0,
      limit: 12,
      items: [playlist("spotify:playlist:1")],
    });
    mocks.albums.mockResolvedValue({
      total: 1,
      offset: 0,
      limit: 12,
      items: [
        {
          uri: "spotify:album:1",
          name: "Album",
          artistNames: ["Artist"],
          year: 2026,
          imageUrl: "/art/album",
        },
      ],
    });
    mocks.artists.mockResolvedValue({
      total: 1,
      offset: 0,
      limit: 12,
      items: [{ uri: "spotify:artist:1", name: "Artist", imageUrl: "/art/artist" }],
    });
    mocks.yourEpisodes.mockResolvedValue(context(true, 2));

    await store.ensureDiscover();
    await store.ensureDiscover();

    expect(mocks.playlists).toHaveBeenCalledTimes(1);
    expect(mocks.playlists).toHaveBeenCalledWith(0, 12);
    expect(mocks.albums).toHaveBeenCalledWith(0, 12);
    expect(mocks.artists).toHaveBeenCalledWith(0, 12);
    expect(mocks.yourEpisodes).toHaveBeenCalledTimes(1);
    expect(store.discover.loaded).toBe(true);
    expect(store.discover.loading).toBe(false);
    expect(store.discover.playlists[0].uri).toBe("spotify:playlist:1");
    expect(store.discover.albums[0].title).toBe("Album");
    expect(store.discover.artists[0].kind).toBe("artist");
    expect(store.discover.episodes).toHaveLength(2);
  });

  it("reports discover load errors without marking the slice loaded", async () => {
    mocks.albums.mockRejectedValue(new Error("engine offline"));

    await store.ensureDiscover();

    expect(store.discover.loaded).toBe(false);
    expect(store.discover.loading).toBe(false);
    expect(store.discover.error).toBe("engine offline");
  });

  it("opens Liked Songs and the Podcasts shortcut", () => {
    store.openLikedSongs();
    expect(store.entity?.uri).toBe("spotify:collection:tracks");
    expect(mocks.contextTracks).toHaveBeenCalledWith("spotify:collection:tracks");
    expect(store.recentEntities[0].uri).toBe("spotify:collection:tracks");

    store.openYourEpisodes();
    expect(store.section).toBe("podcasts");
  });

  it("plays episode entries through their show context when available", async () => {
    const episode: LibraryEntry = {
      kind: "episode",
      uri: "spotify:episode:1",
      title: "Episode",
      image: "",
      owner: "Show",
      artists: ["Host"],
      year: 0,
      length: 0,
      durationMs: 60_000,
      canEdit: false,
      showUri: "spotify:show:1",
    };

    await store.playEntry(episode);
    expect(mocks.playContext).toHaveBeenCalledWith("spotify:show:1", "spotify:episode:1");

    await store.playEntry({ ...episode, showUri: "" });
    expect(mocks.playTrack).toHaveBeenCalledWith("spotify:episode:1");
  });

  it("searches genres with the right tab and records the query", async () => {
    await store.searchGenre({ query: "Jazz", tab: "tracks" });
    expect(store.query).toBe("Jazz");
    expect(store.searchTab).toBe("tracks");
    expect(mocks.search).toHaveBeenCalledWith("Jazz", 30);
    expect(store.recentSearches[0]).toBe("Jazz");

    await store.searchGenre({ query: "Podcast", tab: "shows" });
    expect(store.query).toBe("Podcast");
    expect(store.searchTab).toBe("shows");
    expect(store.recentSearches[0]).toBe("Podcast");
  });
});
