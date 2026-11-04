import { vi } from "vitest";
import { RepeatMode, Status, type PlayerState, type QueueItem } from "../../shared/api/types";

/**
 * Reactive player-store stand-in for component tests. Unlike a plain object it
 * notifies Svelte effects when `state` or `queue` change, which is required to
 * exercise derived values such as the queue track-URI gate.
 */
export function createPlayerState(): PlayerState {
  return {
    provider: "demo",
    status: Status.StatusPlaying,
    track: {
      id: "neon-meridian",
      title: "Neon Meridian",
      artists: ["Aster Vale"],
      album: "Night Cartography",
      cover: "meridian",
      uri: "spotify:track:1",
      durationMs: 252_000,
    },
    progressMs: 38_000,
    durationMs: 252_000,
    volume: 64,
    shuffle: false,
    repeat: RepeatMode.RepeatOff,
    device: {
      id: "studio",
      name: "Studio monitors",
      type: "speaker",
      active: true,
      volumePercent: 64,
      restricted: false,
    },
    devices: [
      {
        id: "studio",
        name: "Studio monitors",
        type: "speaker",
        active: true,
        volumePercent: 64,
        restricted: false,
      },
      {
        id: "living-room",
        name: "Living room",
        type: "speaker",
        active: false,
        volumePercent: 38,
        restricted: false,
      },
    ],
    queue: [],
    canSeek: true,
    canControl: true,
    revision: 3,
    capturedAt: 0,
  };
}

export const playerMock = $state({
  state: createPlayerState(),
  queue: [] as QueueItem[],
  queueLoading: false,
  get positionMs(): number {
    const { status, progressMs, capturedAt, durationMs } = this.state;
    let position = progressMs;
    if (status === Status.StatusPlaying && capturedAt > 0) {
      position += Math.max(0, Date.now() - capturedAt);
    }
    return durationMs > 0 ? Math.min(position, durationMs) : position;
  },
  error: null as string | null,
  start: vi.fn(() => Promise.resolve()),
  stop: vi.fn(),
  loadQueue: vi.fn(() => Promise.resolve()),
  toggle: vi.fn(() => Promise.resolve()),
  play: vi.fn(() => Promise.resolve()),
  pause: vi.fn(() => Promise.resolve()),
  next: vi.fn(() => Promise.resolve()),
  previous: vi.fn(() => Promise.resolve()),
  seek: vi.fn(() => Promise.resolve()),
  setVolume: vi.fn(() => Promise.resolve()),
  toggleShuffle: vi.fn(() => Promise.resolve()),
  cycleRepeat: vi.fn(() => Promise.resolve()),
  transfer: vi.fn(() => Promise.resolve()),
  dismissError: vi.fn(),
});
