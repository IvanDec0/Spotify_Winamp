import { audio as audioApi, onAudioState, toApiError } from "../shared/api";
import type {
  AlbumPage,
  ArtistPage,
  AudioState,
  LibraryContext,
  PlaylistPage,
  SearchHit,
} from "../shared/api/types";

export type SearchResultsView = {
  tracks: SearchHit[];
  albums: SearchHit[];
  artists: SearchHit[];
  playlists: SearchHit[];
  shows: SearchHit[];
  episodes: SearchHit[];
};

function normalizeAudio(state: AudioState): AudioState {
  return {
    available: state.available ?? false,
    running: state.running ?? false,
    authenticated: state.authenticated ?? false,
    engine: state.engine ?? "go-librespot",
    engineVersion: state.engineVersion ?? "",
    deviceName: state.deviceName ?? "Cadence",
    deviceType: state.deviceType ?? "",
    pairing: state.pairing ?? null,
    error: state.error ?? "",
    binaryPath: state.binaryPath ?? "",
    configDirectory: state.configDirectory ?? "",
    audioBackend: state.audioBackend ?? "",
  };
}

const initialState: AudioState = {
  available: false,
  running: false,
  authenticated: false,
  engine: "go-librespot",
  engineVersion: "",
  deviceName: "Cadence",
  deviceType: "",
  pairing: null,
  error: "",
  binaryPath: "",
  configDirectory: "",
  audioBackend: "",
};

// The audio engine pushes state changes; polling is only needed while the
// user still has to pair the device.
const pairingPollInterval = 5000;

class MusicStore {
  audio = $state<AudioState>(initialState);
  loading = $state(true);
  busy = $state(false);
  error = $state<string | null>(null);
  notice = $state<string | null>(null);

  playlists = $state<NonNullable<PlaylistPage["items"]>>([]);
  albums = $state<NonNullable<AlbumPage["items"]>>([]);
  artists = $state<NonNullable<ArtistPage["items"]>>([]);
  context = $state<(LibraryContext & { items: NonNullable<LibraryContext["items"]> }) | null>(null);
  contextLoading = $state(false);
  searchQuery = $state("");
  searchResults = $state<SearchResultsView | null>(null);
  searching = $state(false);
  searchError = $state<string | null>(null);

  #unsubscribe: (() => void) | null = null;
  #started = false;
  #pollTimer: ReturnType<typeof setInterval> | null = null;
  #contextTimer: ReturnType<typeof setTimeout> | null = null;
  #contextURI = "";

  get authenticated(): boolean {
    return this.audio.authenticated;
  }

  get pairing() {
    return this.audio.pairing;
  }

  async start(): Promise<void> {
    if (this.#started) return;
    this.#started = true;
    this.#unsubscribe = onAudioState((state) => {
      this.audio = normalizeAudio(state);
      this.#syncPoll();
    });
    await this.refresh();
  }

  stop(): void {
    this.#unsubscribe?.();
    this.#unsubscribe = null;
    this.#started = false;
    this.#stopPoll();
    if (this.#contextTimer) {
      clearTimeout(this.#contextTimer);
      this.#contextTimer = null;
    }
  }

  async refresh(): Promise<void> {
    try {
      this.audio = normalizeAudio(await audioApi.getState());
      this.loading = false;
      this.#syncPoll();
    } catch (error) {
      this.error = toApiError(error).message;
      this.loading = false;
      this.#syncPoll();
    }
  }

  #syncPoll(): void {
    if (!this.#started) return;
    if (this.audio.authenticated) {
      this.#stopPoll();
      return;
    }
    if (this.#pollTimer) return;
    this.#pollTimer = setInterval(() => {
      void this.refresh();
    }, pairingPollInterval);
  }

  #stopPoll(): void {
    if (!this.#pollTimer) return;
    clearInterval(this.#pollTimer);
    this.#pollTimer = null;
  }

  async openPairing(): Promise<void> {
    try {
      await audioApi.openPairing();
      this.error = null;
    } catch (error) {
      this.error = toApiError(error).message;
    }
  }

  async resetSession(): Promise<void> {
    this.busy = true;
    try {
      this.audio = normalizeAudio(await audioApi.resetSession());
      this.#syncPoll();
      this.clearLibrary();
      this.error = null;
      this.notice = "Pairing reset. Use the new code to connect Spotify.";
    } catch (error) {
      this.error = toApiError(error).message;
    } finally {
      this.busy = false;
    }
  }

  async restart(): Promise<void> {
    this.busy = true;
    try {
      this.audio = normalizeAudio(await audioApi.restart());
      this.#syncPoll();
      this.error = null;
    } catch (error) {
      this.error = toApiError(error).message;
    } finally {
      this.busy = false;
    }
  }

  async loadPlaylists(): Promise<void> {
    await this.#loadLibrary(async () => {
      const page = await audioApi.playlists(0, 100);
      this.playlists = page.items ?? [];
    });
  }

  async loadAlbums(): Promise<void> {
    await this.#loadLibrary(async () => {
      const page = await audioApi.albums(0, 100);
      this.albums = page.items ?? [];
    });
  }

  async loadArtists(): Promise<void> {
    await this.#loadLibrary(async () => {
      const page = await audioApi.artists(0, 100);
      this.artists = page.items ?? [];
    });
  }

  async #loadLibrary(action: () => Promise<void>): Promise<void> {
    this.busy = true;
    try {
      await action();
      this.error = null;
    } catch (error) {
      this.error = toApiError(error).message;
    } finally {
      this.busy = false;
    }
  }

  async loadContext(uri: string): Promise<void> {
    if (this.#contextTimer) {
      clearTimeout(this.#contextTimer);
      this.#contextTimer = null;
    }
    this.#contextURI = uri;
    this.contextLoading = true;
    try {
      const context = await audioApi.contextTracks(uri);
      if (this.#contextURI !== uri) return;
      this.context = {
        ...context,
        items: context.items ?? [],
      };
      this.error = null;
      if (!context.ready) {
        this.#contextTimer = setTimeout(() => {
          this.#contextTimer = null;
          void this.loadContext(uri);
        }, 1000);
      }
    } catch (error) {
      this.error = toApiError(error).message;
    } finally {
      this.contextLoading = false;
    }
  }

  clearContext(): void {
    if (this.#contextTimer) {
      clearTimeout(this.#contextTimer);
      this.#contextTimer = null;
    }
    this.#contextURI = "";
    this.context = null;
  }

  clearLibrary(): void {
    this.playlists = [];
    this.albums = [];
    this.artists = [];
    this.clearContext();
    this.clearSearch();
  }

  async search(query: string): Promise<void> {
    const trimmed = query.trim();
    if (!trimmed) {
      this.clearSearch();
      return;
    }
    this.searchQuery = trimmed;
    this.searching = true;
    this.searchError = null;
    try {
      const results = await audioApi.search(trimmed, 20);
      this.searchResults = {
        tracks: results.tracks?.hits ?? [],
        albums: results.albums?.hits ?? [],
        artists: results.artists?.hits ?? [],
        playlists: results.playlists?.hits ?? [],
        shows: results.shows?.hits ?? [],
        episodes: results.episodes?.hits ?? [],
      };
    } catch (error) {
      this.searchError = toApiError(error).message;
      this.searchResults = null;
    } finally {
      this.searching = false;
    }
  }

  clearSearch(): void {
    this.searchQuery = "";
    this.searchResults = null;
    this.searching = false;
    this.searchError = null;
  }

  async startRadio(uri: string): Promise<void> {
    if (!uri) return;
    this.busy = true;
    try {
      await audioApi.startRadio(uri);
      this.notice = "Radio started.";
      this.error = null;
    } catch (error) {
      this.error = toApiError(error).message;
    } finally {
      this.busy = false;
    }
  }

  async playContext(uri: string, skipToURI = ""): Promise<void> {
    this.busy = true;
    try {
      await audioApi.playContext(uri, skipToURI);
      this.error = null;
    } catch (error) {
      this.error = toApiError(error).message;
    } finally {
      this.busy = false;
    }
  }

  async playTrack(uri: string): Promise<void> {
    this.busy = true;
    try {
      await audioApi.playTrack(uri);
      this.error = null;
    } catch (error) {
      this.error = toApiError(error).message;
    } finally {
      this.busy = false;
    }
  }

  async addToQueue(uri: string): Promise<void> {
    this.busy = true;
    try {
      await audioApi.addToQueue(uri);
      this.notice = "Added to the play queue.";
      this.error = null;
    } catch (error) {
      this.error = toApiError(error).message;
    } finally {
      this.busy = false;
    }
  }

  dismissError(): void {
    this.error = null;
  }

  dismissNotice(): void {
    this.notice = null;
  }
}

export const music = new MusicStore();
