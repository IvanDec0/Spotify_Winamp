import { audio, toApiError } from "../../shared/api";
import type {
  AlbumPage,
  ArtistPage,
  ContextTrackInfo,
  LibraryAlbum,
  LibraryArtist,
  LibraryContext,
  LibraryPlaylist,
  PlaylistPage,
  SearchHit,
} from "../../shared/api/types";
import { t, type MessageKey, type MessageParams } from "../../shared/i18n";
import { player } from "../../lib/player.svelte";

export const BROWSE_SECTIONS = [
  "nowplaying",
  "search",
  "library",
  "podcasts",
  "queue",
  "settings",
] as const;
export type Section = (typeof BROWSE_SECTIONS)[number];

/** Accepts a URL or event value only when it is a known Browse section. */
export function normalizeSection(value: unknown, fallback: Section = "library"): Section {
  return typeof value === "string" && (BROWSE_SECTIONS as readonly string[]).includes(value)
    ? (value as Section)
    : fallback;
}

export type SearchTab = "tracks" | "albums" | "artists" | "playlists" | "shows" | "episodes";
export type LibraryTab = "playlists" | "albums" | "artists" | "episodes";
export type EntityKind = "playlist" | "album" | "artist" | "show";

export interface EntityRef {
  kind: EntityKind;
  uri: string;
  title: string;
  subtitle?: string;
  image?: string;
  owner?: string;
  canEdit?: boolean;
}

export type Route = { name: "root" } | { name: "entity"; entity: EntityRef };

export interface SearchResultsView {
  tracks: SearchHit[];
  albums: SearchHit[];
  artists: SearchHit[];
  playlists: SearchHit[];
  shows: SearchHit[];
  episodes: SearchHit[];
}

export interface LibraryEntry {
  kind: "playlist" | "album" | "artist" | "episode";
  uri: string;
  title: string;
  image: string;
  owner: string;
  artists: string[];
  year: number;
  length: number;
  durationMs: number;
  canEdit: boolean;
  showUri: string;
}

export interface LibraryPage {
  items: LibraryEntry[];
  total: number;
  loading: boolean;
  offset: number;
  loaded: boolean;
}

export interface BrowseNotice {
  key: MessageKey;
  params?: MessageParams;
}

export interface DiscoverState {
  playlists: LibraryEntry[];
  albums: LibraryEntry[];
  artists: LibraryEntry[];
  episodes: LibraryEntry[];
  loading: boolean;
  loaded: boolean;
  error: string | null;
}

export interface GenreSearch {
  query: string;
  tab: SearchTab;
}

const PAGE_SIZE = 50;
const MAX_CONTEXT_RETRIES = 10;
const MAX_RECENT_ENTITIES = 8;
const MAX_RECENT_SEARCHES = 6;
const RECENT_ENTITIES_KEY = "cadence.browse.recentEntities";
const RECENT_SEARCHES_KEY = "cadence.browse.recentSearches";
const LIKED_SONGS_URI = "spotify:collection:tracks";
const ENTITY_KINDS: readonly EntityKind[] = ["playlist", "album", "artist", "show"];

function emptyDiscover(): DiscoverState {
  return {
    playlists: [],
    albums: [],
    artists: [],
    episodes: [],
    loading: false,
    loaded: false,
    error: null,
  };
}

function readStoredJSON(key: string): unknown {
  try {
    const raw = globalThis.localStorage?.getItem(key);
    return raw ? (JSON.parse(raw) as unknown) : null;
  } catch {
    return null;
  }
}

function writeStoredJSON(key: string, value: unknown): void {
  try {
    globalThis.localStorage?.setItem(key, JSON.stringify(value));
  } catch {
    // Recents stay in memory when storage is unavailable.
  }
}

function isEntityKind(value: unknown): value is EntityKind {
  return typeof value === "string" && (ENTITY_KINDS as readonly string[]).includes(value);
}

function sanitizeEntity(value: unknown): EntityRef | null {
  if (!value || typeof value !== "object") return null;
  const record = value as Record<string, unknown>;
  if (!isEntityKind(record.kind)) return null;
  if (typeof record.uri !== "string" || !record.uri.trim()) return null;
  if (typeof record.title !== "string" || !record.title.trim()) return null;

  const ref: EntityRef = {
    kind: record.kind,
    uri: record.uri.trim(),
    title: record.title.trim(),
  };
  if (typeof record.subtitle === "string" && record.subtitle) ref.subtitle = record.subtitle;
  if (typeof record.image === "string" && record.image) ref.image = record.image;
  if (typeof record.owner === "string" && record.owner) ref.owner = record.owner;
  if (typeof record.canEdit === "boolean") ref.canEdit = record.canEdit;
  return ref;
}

function loadRecentEntities(): EntityRef[] {
  const stored = readStoredJSON(RECENT_ENTITIES_KEY);
  if (!Array.isArray(stored)) return [];
  const entities: EntityRef[] = [];
  for (const value of stored) {
    const ref = sanitizeEntity(value);
    if (!ref || entities.some((entity) => entity.uri === ref.uri)) continue;
    entities.push(ref);
    if (entities.length >= MAX_RECENT_ENTITIES) break;
  }
  return entities;
}

function loadRecentSearches(): string[] {
  const stored = readStoredJSON(RECENT_SEARCHES_KEY);
  if (!Array.isArray(stored)) return [];
  const searches: string[] = [];
  for (const value of stored) {
    if (typeof value !== "string") continue;
    const query = value.trim();
    if (!query || searches.includes(query)) continue;
    searches.push(query);
    if (searches.length >= MAX_RECENT_SEARCHES) break;
  }
  return searches;
}

function emptyPage(): LibraryPage {
  return { items: [], total: 0, loading: false, offset: 0, loaded: false };
}

export function emptyPages(): Record<LibraryTab, LibraryPage> {
  return {
    playlists: emptyPage(),
    albums: emptyPage(),
    artists: emptyPage(),
    episodes: emptyPage(),
  };
}

function artistsOf(hit: SearchHit): string {
  return (hit.artists ?? []).map((artist) => artist.name).join(", ");
}

export function entityFromSearchHit(hit: SearchHit, kind: SearchTab): EntityRef | null {
  switch (kind) {
    case "albums":
      return {
        kind: "album",
        uri: hit.uri,
        title: hit.name,
        subtitle: artistsOf(hit),
        image: hit.image,
      };
    case "artists":
      return {
        kind: "artist",
        uri: hit.uri,
        title: hit.name,
        image: hit.image,
      };
    case "playlists":
      return {
        kind: "playlist",
        uri: hit.uri,
        title: hit.name,
        subtitle: hit.author,
        image: hit.image,
        owner: hit.author,
        canEdit: false,
      };
    case "shows":
      return {
        kind: "show",
        uri: hit.uri,
        title: hit.name,
        subtitle: hit.showType,
        image: hit.image,
      };
    default:
      return null;
  }
}

export function entityFromLibraryEntry(entry: LibraryEntry): EntityRef | null {
  if (entry.kind === "episode") return null;
  return {
    kind: entry.kind,
    uri: entry.uri,
    title: entry.title,
    subtitle: entry.artists.join(", "),
    image: entry.image,
    owner: entry.owner,
    canEdit: entry.canEdit,
  };
}

function playlistEntry(item: LibraryPlaylist): LibraryEntry {
  return {
    kind: "playlist",
    uri: item.uri,
    title: item.name,
    image: item.imageUrl,
    owner: item.owner,
    artists: [],
    year: 0,
    length: item.length,
    durationMs: 0,
    canEdit: item.canEdit,
    showUri: "",
  };
}

function albumEntry(item: LibraryAlbum): LibraryEntry {
  return {
    kind: "album",
    uri: item.uri,
    title: item.name,
    image: item.imageUrl,
    owner: "",
    artists: item.artistNames ?? [],
    year: item.year,
    length: 0,
    durationMs: 0,
    canEdit: false,
    showUri: "",
  };
}

function artistEntry(item: LibraryArtist): LibraryEntry {
  return {
    kind: "artist",
    uri: item.uri,
    title: item.name,
    image: item.imageUrl,
    owner: "",
    artists: [],
    year: 0,
    length: 0,
    durationMs: 0,
    canEdit: false,
    showUri: "",
  };
}

function episodeEntry(item: ContextTrackInfo): LibraryEntry {
  return {
    kind: "episode",
    uri: item.uri,
    title: item.title,
    image: item.cover,
    owner: item.album,
    artists: item.artists ?? [],
    year: 0,
    length: 0,
    durationMs: item.durationMs,
    canEdit: false,
    showUri: "",
  };
}

function normalizeContext(context: LibraryContext): LibraryContext & {
  items: NonNullable<LibraryContext["items"]>;
} {
  return { ...context, items: context.items ?? [] };
}

export class BrowseStore {
  section = $state<Section>("search");
  searchTab = $state<SearchTab>("tracks");
  libraryTab = $state<LibraryTab>("playlists");
  query = $state("");
  searching = $state(false);
  searchError = $state<string | null>(null);
  results = $state<SearchResultsView | null>(null);
  searchSequence = $state(0);

  recentEntities = $state<EntityRef[]>(loadRecentEntities());
  recentSearches = $state<string[]>(loadRecentSearches());
  discover = $state<DiscoverState>(emptyDiscover());

  pages = $state<Record<LibraryTab, LibraryPage>>(emptyPages());
  libraryError = $state<string | null>(null);
  stack = $state<Route[]>([{ name: "root" }]);
  entity = $state<EntityRef | null>(null);
  context = $state<(LibraryContext & { items: NonNullable<LibraryContext["items"]> }) | null>(null);
  contextLoading = $state(false);
  contextError = $state<string | null>(null);
  entitySaved = $state(false);

  currentLiked = $state(false);
  likedLoading = $state(false);

  editablePlaylists = $state<LibraryPlaylist[]>([]);
  playlistsLoading = $state(false);

  busy = $state(false);
  notice = $state<BrowseNotice | null>(null);
  error = $state<string | null>(null);

  #discoverRequest: Promise<void> | null = null;
  #contextTimer: ReturnType<typeof setTimeout> | null = null;
  #contextRequest = 0;
  #contextAttempts = 0;
  #savedRequest = 0;
  #likedURI = "";
  #libraryRequests: Record<LibraryTab, number> = {
    playlists: 0,
    albums: 0,
    artists: 0,
    episodes: 0,
  };

  get route(): Route {
    return this.stack[this.stack.length - 1] ?? { name: "root" };
  }

  async search(query: string): Promise<void> {
    const trimmed = query.trim();
    this.query = trimmed;
    if (!trimmed) {
      this.clearSearch();
      return;
    }

    const sequence = ++this.searchSequence;
    this.searching = true;
    this.searchError = null;
    try {
      const results = await audio.search(trimmed, 30);
      if (sequence !== this.searchSequence) return;
      this.results = {
        tracks: results.tracks?.hits ?? [],
        albums: results.albums?.hits ?? [],
        artists: results.artists?.hits ?? [],
        playlists: results.playlists?.hits ?? [],
        shows: results.shows?.hits ?? [],
        episodes: results.episodes?.hits ?? [],
      };
    } catch (error) {
      if (sequence !== this.searchSequence) return;
      this.searchError = toApiError(error).message;
      this.results = null;
    } finally {
      if (sequence === this.searchSequence) this.searching = false;
    }
  }

  clearSearch(): void {
    this.searchSequence += 1;
    this.query = "";
    this.results = null;
    this.searching = false;
    this.searchError = null;
  }

  recordEntity(ref: EntityRef): void {
    const entity = sanitizeEntity(ref);
    if (!entity) return;
    this.recentEntities = [
      entity,
      ...this.recentEntities.filter((item) => item.uri !== entity.uri),
    ].slice(0, MAX_RECENT_ENTITIES);
    writeStoredJSON(RECENT_ENTITIES_KEY, this.recentEntities);
  }

  recordSearch(query: string): void {
    const trimmed = query.trim();
    if (!trimmed) return;
    this.recentSearches = [
      trimmed,
      ...this.recentSearches.filter((item) => item !== trimmed),
    ].slice(0, MAX_RECENT_SEARCHES);
    writeStoredJSON(RECENT_SEARCHES_KEY, this.recentSearches);
  }

  clearRecentSearches(): void {
    this.recentSearches = [];
    writeStoredJSON(RECENT_SEARCHES_KEY, this.recentSearches);
  }

  ensureDiscover(): Promise<void> {
    if (this.discover.loaded) return Promise.resolve();
    if (this.#discoverRequest) return this.#discoverRequest;
    this.#discoverRequest = this.#loadDiscover().finally(() => {
      this.#discoverRequest = null;
    });
    return this.#discoverRequest;
  }

  async #loadDiscover(): Promise<void> {
    this.discover = { ...this.discover, loading: true, error: null };
    try {
      const [playlists, albums, artists, episodes] = await Promise.all([
        audio.playlists(0, 12),
        audio.albums(0, 12),
        audio.artists(0, 12),
        audio.yourEpisodes(),
      ]);
      this.discover = {
        playlists: (playlists.items ?? []).map(playlistEntry),
        albums: (albums.items ?? []).map(albumEntry),
        artists: (artists.items ?? []).map(artistEntry),
        episodes: normalizeContext(episodes).items.map(episodeEntry),
        loading: false,
        loaded: true,
        error: null,
      };
    } catch (error) {
      this.discover = {
        ...this.discover,
        loading: false,
        error: toApiError(error).message,
      };
    }
  }

  async playEntry(entry: LibraryEntry): Promise<void> {
    if (entry.kind === "episode") {
      if (entry.showUri) {
        await this.#run(() => audio.playContext(entry.showUri, entry.uri));
      } else {
        await this.playEpisode(entry.uri);
      }
      return;
    }
    const entity = entityFromLibraryEntry(entry);
    if (entity) await this.openEntity(entity);
  }

  openLikedSongs(): void {
    void this.openEntity({
      kind: "playlist",
      uri: LIKED_SONGS_URI,
      title: t("browse.likedSongs"),
      canEdit: false,
    });
  }

  openYourEpisodes(): void {
    this.setSection("podcasts");
  }

  async searchGenre(genre: GenreSearch): Promise<void> {
    if (this.section !== "search" || this.route.name !== "root") {
      this.setSection("search");
    }
    this.searchTab = genre.tab;
    this.recordSearch(genre.query);
    await this.search(genre.query);
  }

  async loadLibrary(tab: LibraryTab): Promise<void> {
    const request = ++this.#libraryRequests[tab];
    this.libraryError = null;
    this.pages[tab] = { ...emptyPage(), loading: true };
    try {
      const page = await this.#fetchPage(tab, 0);
      if (request !== this.#libraryRequests[tab]) return;
      this.pages[tab] = { ...page, loading: false, loaded: true };
      this.libraryError = null;
    } catch (error) {
      if (request !== this.#libraryRequests[tab]) return;
      this.pages[tab] = { ...this.pages[tab], loading: false };
      this.libraryError = toApiError(error).message;
    }
  }

  async loadMore(tab: LibraryTab): Promise<void> {
    const page = this.pages[tab];
    if (page.loading || !page.loaded || tab === "episodes") return;
    if (page.offset >= page.total) return;

    const request = ++this.#libraryRequests[tab];
    this.libraryError = null;
    this.pages[tab] = { ...page, loading: true };
    try {
      const next = await this.#fetchPage(tab, page.offset);
      if (request !== this.#libraryRequests[tab]) return;
      this.pages[tab] = {
        items: [...page.items, ...next.items],
        total: next.total,
        offset: next.offset,
        loading: false,
        loaded: true,
      };
      this.libraryError = null;
    } catch (error) {
      if (request !== this.#libraryRequests[tab]) return;
      this.pages[tab] = { ...this.pages[tab], loading: false };
      this.libraryError = toApiError(error).message;
    }
  }

  async #fetchPage(tab: LibraryTab, offset: number): Promise<LibraryPage> {
    if (tab === "episodes") {
      const context = normalizeContext(await audio.yourEpisodes());
      const items = context.items.map(episodeEntry);
      return { items, total: items.length, offset: items.length, loading: false, loaded: true };
    }

    if (tab === "playlists") {
      const page: PlaylistPage = await audio.playlists(offset, PAGE_SIZE);
      const items = (page.items ?? []).map(playlistEntry);
      return {
        items,
        total: page.total,
        offset: page.offset + items.length,
        loading: false,
        loaded: true,
      };
    }
    if (tab === "albums") {
      const page: AlbumPage = await audio.albums(offset, PAGE_SIZE);
      const items = (page.items ?? []).map(albumEntry);
      return {
        items,
        total: page.total,
        offset: page.offset + items.length,
        loading: false,
        loaded: true,
      };
    }
    const page: ArtistPage = await audio.artists(offset, PAGE_SIZE);
    const items = (page.items ?? []).map(artistEntry);
    return {
      items,
      total: page.total,
      offset: page.offset + items.length,
      loading: false,
      loaded: true,
    };
  }

  async openEntity(ref: EntityRef): Promise<void> {
    this.recordEntity(ref);
    this.#cancelEntityContext();
    this.stack = [...this.stack, { name: "entity", entity: ref }];
    this.#beginEntity(ref);
    const request = this.#contextRequest;
    void this.#loadEntitySaved(request, ref);
    await this.#loadEntityContext(request, ref);
  }

  async reloadEntity(): Promise<void> {
    const entity = this.entity;
    if (!entity) return;
    this.#cancelEntityContext();
    this.#beginEntity(entity);
    const request = this.#contextRequest;
    void this.#loadEntitySaved(request, entity);
    await this.#loadEntityContext(request, entity);
  }

  back(): void {
    if (this.stack.length <= 1) return;
    this.#cancelEntityContext();
    const nextStack = this.stack.slice(0, -1);
    const next = nextStack[nextStack.length - 1] ?? { name: "root" };
    this.stack = nextStack;
    if (next.name === "entity") {
      this.#beginEntity(next.entity);
      const request = this.#contextRequest;
      void this.#loadEntityContext(request, next.entity);
      void this.#loadEntitySaved(request, next.entity);
    } else {
      this.#clearEntity();
    }
  }

  async playEntity(): Promise<void> {
    const entity = this.entity;
    if (!entity) return;
    this.recordEntity(entity);
    await this.#run(() => audio.playContext(entity.uri, ""));
  }

  async playItem(item: ContextTrackInfo): Promise<void> {
    const entity = this.entity;
    if (!entity) return;
    this.recordEntity(entity);
    await this.#run(() => audio.playContext(entity.uri, item.uri));
  }

  async playEpisode(uri: string): Promise<void> {
    await this.#run(() => audio.playTrack(uri));
  }

  async playSearchHit(hit: SearchHit, kind: SearchTab): Promise<void> {
    if (kind === "tracks" || kind === "episodes") {
      await this.playEpisode(hit.uri);
      return;
    }
    const entity = entityFromSearchHit(hit, kind);
    if (entity) await this.openEntity(entity);
  }

  async addToQueue(uri: string): Promise<void> {
    await this.#run(() => audio.addToQueue(uri), { key: "browse.noticeQueued" });
  }

  async startRadio(uri: string): Promise<void> {
    await this.#run(() => audio.startRadio(uri), { key: "browse.noticeRadioStarted" });
  }

  async toggleSave(): Promise<void> {
    const entity = this.entity;
    if (!entity) return;
    const kind = entity.kind;
    if (kind !== "album" && kind !== "show" && kind !== "artist") return;

    this.#savedRequest += 1;
    const next = !this.entitySaved;
    this.entitySaved = next;
    this.busy = true;
    try {
      if (kind === "album") {
        await audio.saveAlbum(entity.uri, next);
      } else if (kind === "show") {
        await audio.saveShow(entity.uri, next);
      } else {
        await audio.followArtist(entity.uri, next);
      }
      this.error = null;
      if (kind === "artist") {
        this.notice = { key: next ? "browse.noticeFollowed" : "browse.noticeUnfollowed" };
      } else {
        this.notice = { key: next ? "browse.noticeSaved" : "browse.noticeRemoved" };
      }
    } catch (error) {
      this.entitySaved = !next;
      this.error = toApiError(error).message;
    } finally {
      this.busy = false;
    }
  }

  async syncTrackLike(uri: string): Promise<void> {
    if (!uri) {
      this.#likedURI = "";
      this.currentLiked = false;
      this.likedLoading = false;
      return;
    }
    if (uri === this.#likedURI) return;

    this.#likedURI = uri;
    this.currentLiked = false;
    this.likedLoading = true;
    try {
      const liked = await audio.isLiked(uri);
      if (this.#likedURI === uri) this.currentLiked = liked;
    } catch {
      if (this.#likedURI === uri) this.currentLiked = false;
    } finally {
      if (this.#likedURI === uri) this.likedLoading = false;
    }
  }

  async toggleCurrentLike(): Promise<void> {
    const uri = player.state.track.uri;
    if (!uri) return;
    const next = !this.currentLiked;
    this.currentLiked = next;
    try {
      await audio.setLiked(uri, next);
      this.error = null;
      this.notice = { key: next ? "browse.noticeLiked" : "browse.noticeUnliked" };
    } catch (error) {
      this.currentLiked = !next;
      this.error = toApiError(error).message;
    }
  }

  async loadEditablePlaylists(): Promise<void> {
    if (this.playlistsLoading) return;
    this.playlistsLoading = true;
    try {
      const page = await audio.playlists(0, 100);
      this.editablePlaylists = (page.items ?? []).filter((playlist) => playlist.canEdit);
      this.error = null;
    } catch (error) {
      this.error = toApiError(error).message;
    } finally {
      this.playlistsLoading = false;
    }
  }

  async addToPlaylist(trackUri: string, playlistUri: string): Promise<void> {
    if (!trackUri || !playlistUri) return;
    await this.#run(() => audio.addToPlaylist(playlistUri, trackUri), {
      key: "browse.noticeAddedToPlaylist",
    });
  }

  async removeFromPlaylist(trackUri: string): Promise<void> {
    const entity = this.entity;
    if (!trackUri || !entity || entity.kind !== "playlist" || !entity.canEdit) return;

    this.busy = true;
    try {
      await audio.removeFromPlaylist(entity.uri, trackUri);
      if (this.context) {
        const items = this.context.items.filter((item) => item.uri !== trackUri);
        this.context = {
          ...this.context,
          items,
          length: Math.max(0, this.context.length - 1),
          cached: Math.max(0, this.context.cached - 1),
        };
      }
      this.error = null;
      this.notice = { key: "browse.noticeRemovedFromPlaylist" };
    } catch (error) {
      this.error = toApiError(error).message;
    } finally {
      this.busy = false;
    }
  }

  setInitialSection(section: Section): void {
    this.#resetRoute();
    this.section = section;
  }

  setSection(section: Section): void {
    this.section = section;
    this.#resetRoute();
    if (section === "library") {
      const page = this.pages[this.libraryTab];
      if (!page.loaded && !page.loading) void this.loadLibrary(this.libraryTab);
    }
    if (section === "podcasts") {
      const page = this.pages.episodes;
      if (!page.loaded && !page.loading) void this.loadLibrary("episodes");
    }
  }

  setSearchTab(tab: SearchTab): void {
    this.searchTab = tab;
  }

  setLibraryTab(tab: LibraryTab): void {
    if (this.libraryTab === tab) return;
    this.libraryTab = tab;
    void this.loadLibrary(tab);
  }

  dismissError(): void {
    this.error = null;
  }

  dismissNotice(): void {
    this.notice = null;
  }

  reset(): void {
    this.#cancelEntityContext();
    this.searchSequence += 1;
    this.#libraryRequests = { playlists: 0, albums: 0, artists: 0, episodes: 0 };
    this.section = "search";
    this.searchTab = "tracks";
    this.libraryTab = "playlists";
    this.query = "";
    this.searching = false;
    this.searchError = null;
    this.results = null;
    this.discover = emptyDiscover();
    this.#discoverRequest = null;
    this.pages = emptyPages();
    this.libraryError = null;
    this.stack = [{ name: "root" }];
    this.#clearEntity();
    this.currentLiked = false;
    this.likedLoading = false;
    this.#likedURI = "";
    this.editablePlaylists = [];
    this.playlistsLoading = false;
    this.busy = false;
    this.notice = null;
    this.error = null;
  }

  #beginEntity(ref: EntityRef): void {
    this.entity = ref;
    this.context = null;
    this.contextLoading = true;
    this.contextError = null;
    this.entitySaved = false;
    this.#contextAttempts = 0;
  }

  #clearEntity(): void {
    this.entity = null;
    this.context = null;
    this.contextLoading = false;
    this.contextError = null;
    this.entitySaved = false;
  }

  #resetRoute(): void {
    this.#cancelEntityContext();
    this.stack = [{ name: "root" }];
    this.#clearEntity();
  }

  #cancelEntityContext(): void {
    this.#contextRequest += 1;
    if (this.#contextTimer) {
      clearTimeout(this.#contextTimer);
      this.#contextTimer = null;
    }
  }

  async #loadEntityContext(request: number, ref: EntityRef): Promise<void> {
    this.#contextAttempts += 1;
    try {
      const context = normalizeContext(await audio.contextTracks(ref.uri));
      if (request !== this.#contextRequest) return;
      this.context = context;
      this.contextLoading = false;
      this.contextError = null;
      if (!context.ready && this.#contextAttempts <= MAX_CONTEXT_RETRIES) {
        this.#contextTimer = setTimeout(() => {
          this.#contextTimer = null;
          void this.#loadEntityContext(request, ref);
        }, 1000);
      }
    } catch (error) {
      if (request !== this.#contextRequest) return;
      this.contextLoading = false;
      this.contextError = toApiError(error).message;
    }
  }

  async #loadEntitySaved(request: number, ref: EntityRef): Promise<void> {
    const savedRequest = ++this.#savedRequest;
    try {
      let saved = false;
      if (ref.kind === "album") {
        saved = await audio.isAlbumSaved(ref.uri);
      } else if (ref.kind === "show") {
        saved = await audio.isShowSaved(ref.uri);
      } else if (ref.kind === "artist") {
        saved = await audio.isArtistFollowed(ref.uri);
      } else {
        return;
      }
      if (request === this.#contextRequest && savedRequest === this.#savedRequest) {
        this.entitySaved = saved;
      }
    } catch {
      // Saved state is supplementary; the entity page still works without it.
    }
  }

  async #run(action: () => Promise<void>, notice?: BrowseNotice): Promise<void> {
    this.busy = true;
    try {
      await action();
      this.error = null;
      if (notice) this.notice = notice;
    } catch (error) {
      this.error = toApiError(error).message;
    } finally {
      this.busy = false;
    }
  }
}

export const browse = new BrowseStore();
