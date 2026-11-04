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
  ready = $state(false);
  busy = $state(false);
  error = $state<string | null>(null);

  #unsubscribe: (() => void) | null = null;
  #starting = false;
  #pending = 0;

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
    this.state = normalizeState(state);
  }
}

export const player = new PlayerStore();
