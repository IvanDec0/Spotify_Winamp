import { Events } from "@wailsio/runtime";
import {
  Service as AudioService,
  type AlbumPage,
  type ArtistPage,
  type AudioState,
  type LibraryContext,
  type PlaylistPage,
  type SearchHit,
} from "../../bindings/cadence/internal/golibrespot";

const AUDIO_EVENT = "audio:state";

export type SearchResultsView = {
  tracks: SearchHit[];
  albums: SearchHit[];
  artists: SearchHit[];
  playlists: SearchHit[];
};

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  return typeof error === "string" ? error : "The audio engine operation failed.";
}

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

  #onAudio = (event: { data: unknown }): void => {
    this.audio = normalizeAudio(event.data as AudioState);
  };

  get authenticated(): boolean {
    return this.audio.authenticated;
  }

  get pairing() {
    return this.audio.pairing;
  }

  async start(): Promise<void> {
    if (this.#started) return;
    this.#started = true;
    this.#unsubscribe = Events.On(AUDIO_EVENT, this.#onAudio);
    await this.refresh();
    this.#pollTimer = setInterval(() => {
      void this.refresh();
    }, 2000);
  }

  stop(): void {
    this.#unsubscribe?.();
    this.#unsubscribe = null;
    this.#started = false;
    if (this.#pollTimer) {
      clearInterval(this.#pollTimer);
      this.#pollTimer = null;
    }
    if (this.#contextTimer) {
      clearTimeout(this.#contextTimer);
      this.#contextTimer = null;
    }
  }

  async refresh(): Promise<void> {
    try {
      this.audio = normalizeAudio(await AudioService.GetState());
      this.loading = false;
    } catch (error) {
      this.error = errorMessage(error);
      this.loading = false;
    }
  }

  async openPairing(): Promise<void> {
    try {
      await AudioService.OpenPairing();
      this.error = null;
    } catch (error) {
      this.error = errorMessage(error);
    }
  }

  async resetSession(): Promise<void> {
    this.busy = true;
    try {
      this.audio = normalizeAudio(await AudioService.ResetSession());
      this.clearLibrary();
      this.error = null;
      this.notice = "Pairing reset. Use the new code to connect Spotify.";
    } catch (error) {
      this.error = errorMessage(error);
    } finally {
      this.busy = false;
    }
  }

  async restart(): Promise<void> {
    this.busy = true;
    try {
      this.audio = normalizeAudio(await AudioService.Restart());
      this.error = null;
    } catch (error) {
      this.error = errorMessage(error);
    } finally {
      this.busy = false;
    }
  }

  async loadPlaylists(): Promise<void> {
    await this.#loadLibrary(async () => {
      const page = await AudioService.Playlists(0, 100);
      this.playlists = page.items ?? [];
    });
  }

  async loadAlbums(): Promise<void> {
    await this.#loadLibrary(async () => {
      const page = await AudioService.Albums(0, 100);
      this.albums = page.items ?? [];
    });
  }

  async loadArtists(): Promise<void> {
    await this.#loadLibrary(async () => {
      const page = await AudioService.Artists(0, 100);
      this.artists = page.items ?? [];
    });
  }

  async #loadLibrary(action: () => Promise<void>): Promise<void> {
    this.busy = true;
    try {
      await action();
      this.error = null;
    } catch (error) {
      this.error = errorMessage(error);
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
      const context = await AudioService.ContextTracks(uri);
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
      this.error = errorMessage(error);
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
      const results = await AudioService.Search(trimmed, 20);
      this.searchResults = {
        tracks: results.tracks?.hits ?? [],
        albums: results.albums?.hits ?? [],
        artists: results.artists?.hits ?? [],
        playlists: results.playlists?.hits ?? [],
      };
    } catch (error) {
      this.searchError = errorMessage(error);
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
      await AudioService.StartRadio(uri);
      this.notice = "Radio started.";
      this.error = null;
    } catch (error) {
      this.error = errorMessage(error);
    } finally {
      this.busy = false;
    }
  }

  async playContext(uri: string, skipToURI = ""): Promise<void> {
    this.busy = true;
    try {
      await AudioService.PlayContext(uri, skipToURI);
      this.error = null;
    } catch (error) {
      this.error = errorMessage(error);
    } finally {
      this.busy = false;
    }
  }

  async playTrack(uri: string): Promise<void> {
    this.busy = true;
    try {
      await AudioService.PlayTrack(uri);
      this.error = null;
    } catch (error) {
      this.error = errorMessage(error);
    } finally {
      this.busy = false;
    }
  }

  async addToQueue(uri: string): Promise<void> {
    this.busy = true;
    try {
      await AudioService.AddToQueue(uri);
      this.notice = "Added to the play queue.";
      this.error = null;
    } catch (error) {
      this.error = errorMessage(error);
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
