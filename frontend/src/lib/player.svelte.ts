import { Events } from "@wailsio/runtime";
import { Service as PlayerService } from "../../bindings/cadence/internal/player";
import {
  RepeatMode,
  Status,
  type PlayerState,
  type QueueItem,
  type Track,
} from "../../bindings/cadence/internal/player/models";

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

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
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
      this.apply(await PlayerService.GetState());
      this.#unsubscribe = Events.On("player:state", (event) => {
        this.apply(event.data as PlayerState);
      });
      this.ready = true;
      this.error = null;
    } catch (error) {
      this.error = errorMessage(error);
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
    await this.#action(() => PlayerService.Play());
  }

  async pause(): Promise<void> {
    await this.#action(() => PlayerService.Pause());
  }

  async toggle(): Promise<void> {
    await this.#action(() => PlayerService.TogglePlay());
  }

  async next(): Promise<void> {
    await this.#action(() => PlayerService.Next());
  }

  async previous(): Promise<void> {
    await this.#action(() => PlayerService.Previous());
  }

  async seek(positionMs: number): Promise<void> {
    await this.#action(() => PlayerService.Seek(Math.round(positionMs)));
  }

  async setVolume(volume: number): Promise<void> {
    await this.#action(() => PlayerService.SetVolume(Math.round(volume)));
  }

  async toggleShuffle(): Promise<void> {
    const next = !this.state.shuffle;
    await this.#action(() => PlayerService.SetShuffle(next));
  }

  async cycleRepeat(): Promise<void> {
    const next =
      this.state.repeat === RepeatMode.RepeatOff
        ? RepeatMode.RepeatContext
        : this.state.repeat === RepeatMode.RepeatContext
          ? RepeatMode.RepeatTrack
          : RepeatMode.RepeatOff;
    await this.#action(() => PlayerService.SetRepeat(next));
  }

  async transfer(deviceId: string): Promise<void> {
    await this.#action(() => PlayerService.Transfer(deviceId));
  }

  async #action(action: () => Promise<PlayerState>): Promise<void> {
    this.#pending += 1;
    this.busy = true;
    try {
      this.apply(await action());
      this.error = null;
    } catch (error) {
      this.error = errorMessage(error);
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
