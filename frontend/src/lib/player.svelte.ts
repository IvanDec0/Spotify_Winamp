import { onPlayerState, player as playerApi, toApiError } from "../shared/api";
import {
  RepeatMode,
  Status,
  type PlayerState,
  type QueueItem,
  type Track,
} from "../shared/api/types";

const fallbackTrack: Track = {
  id: "loading",
  title: "Loading playback",
  artists: [],
  album: "",
  cover: "meridian",
  uri: "",
  durationMs: 1,
};

const fallbackState: PlayerState = {
  provider: "demo",
  status: Status.StatusPaused,
  track: fallbackTrack,
  progressMs: 0,
  durationMs: 1,
  volume: 0,
  shuffle: false,
  repeat: RepeatMode.RepeatOff,
  device: {
    id: "",
    name: "No device",
    type: "",
    active: true,
    volumePercent: 0,
    restricted: false,
  },
  devices: [],
  queue: [],
  canSeek: false,
  canControl: false,
  revision: 0,
  capturedAt: 0,
};

function normalizeTrack(track: Track): Track {
  return { ...track, artists: track.artists ?? [] };
}

function normalizeQueue(queue: QueueItem[] | null): QueueItem[] {
  return (queue ?? []).map((item) => ({ ...item, track: normalizeTrack(item.track) }));
}

function normalizeState(state: PlayerState): PlayerState {
  return {
    ...state,
    track: normalizeTrack(state.track),
    devices: state.devices ?? [],
    queue: normalizeQueue(state.queue),
  };
}

class PlayerStore {
  state = $state<PlayerState>(fallbackState);
  queue = $state<QueueItem[]>([]);
  ready = $state(false);
  busy = $state(false);
  queueLoading = $state(false);
  error = $state<string | null>(null);

  #unsubscribe: (() => void) | null = null;
  #starting = false;
  #pending = 0;
  #queueRequest: Promise<void> | null = null;

  /**
   * Interpolated playback position. The engine only sends progress on state
   * changes, so the frontend advances it from the capture timestamp.
   */
  get positionMs(): number {
    const { status, progressMs, capturedAt, durationMs } = this.state;
    let position = progressMs;
    if (status === Status.StatusPlaying && capturedAt > 0) {
      position += Math.max(0, Date.now() - capturedAt);
    }
    return durationMs > 0 ? Math.min(position, durationMs) : position;
  }

  async start(): Promise<void> {
    if (this.ready || this.#starting) return;
    this.#starting = true;
    try {
      this.apply(await playerApi.getState());
      this.#unsubscribe = onPlayerState((state) => {
        this.apply(state);
      });
      this.ready = true;
      this.error = null;
    } catch (error) {
      this.error = toApiError(error).message;
    } finally {
      this.#starting = false;
    }
  }

  stop(): void {
    this.#unsubscribe?.();
    this.#unsubscribe = null;
    this.ready = false;
  }

  dismissError(): void {
    this.error = null;
  }

  async play(): Promise<void> {
    await this.#action(() => playerApi.play());
  }

  async pause(): Promise<void> {
    await this.#action(() => playerApi.pause());
  }

  async toggle(): Promise<void> {
    await this.#action(() => playerApi.togglePlay());
  }

  async next(): Promise<void> {
    await this.#action(() => playerApi.next());
  }

  async previous(): Promise<void> {
    await this.#action(() => playerApi.previous());
  }

  async seek(positionMs: number): Promise<void> {
    await this.#action(() => playerApi.seek(positionMs));
  }

  async setVolume(volume: number): Promise<void> {
    await this.#action(() => playerApi.setVolume(volume));
  }

  async toggleShuffle(): Promise<void> {
    const next = !this.state.shuffle;
    await this.#action(() => playerApi.setShuffle(next));
  }

  async cycleRepeat(): Promise<void> {
    const next =
      this.state.repeat === RepeatMode.RepeatOff
        ? RepeatMode.RepeatContext
        : this.state.repeat === RepeatMode.RepeatContext
          ? RepeatMode.RepeatTrack
          : RepeatMode.RepeatOff;
    await this.#action(() => playerApi.setRepeat(next));
  }

  async transfer(deviceId: string): Promise<void> {
    await this.#action(() => playerApi.transfer(deviceId));
  }

  /**
   * Loads the playback queue on demand. Concurrent callers share one request
   * so opening the queue twice does not hit the engine twice.
   */
  loadQueue(): Promise<void> {
    if (this.#queueRequest) return this.#queueRequest;
    this.queueLoading = true;
    this.#queueRequest = this.#fetchQueue().finally(() => {
      this.#queueRequest = null;
      this.queueLoading = false;
    });
    return this.#queueRequest;
  }

  async #fetchQueue(): Promise<void> {
    try {
      this.queue = normalizeQueue(await playerApi.getQueue());
      this.error = null;
    } catch (error) {
      this.error = toApiError(error).message;
    }
  }

  async #action(action: () => Promise<PlayerState>): Promise<void> {
    this.#pending += 1;
    this.busy = true;
    try {
      this.apply(await action());
      this.error = null;
    } catch (error) {
      this.error = toApiError(error).message;
    } finally {
      this.#pending -= 1;
      this.busy = this.#pending > 0;
    }
  }

  apply(state: PlayerState): void {
    // States arrive from the action response and the engine stream; a slower
    // response must not overwrite a newer snapshot.
    if (state.revision > 0 && state.revision < this.state.revision) return;
    this.state = normalizeState(state);
  }
}

export const player = new PlayerStore();
