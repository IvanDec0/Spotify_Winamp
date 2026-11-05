import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => {
  const captured: { handler: ((state: PlayerState) => void) | null } = { handler: null };
  const unsubscribe = vi.fn();
  const onPlayerState = vi.fn((handler: (state: PlayerState) => void) => {
    captured.handler = handler;
    return unsubscribe;
  });
  return {
    getState: vi.fn(),
    getQueue: vi.fn(),
    captured,
    unsubscribe,
    onPlayerState,
  };
});

vi.mock("../shared/api", () => ({
  player: {
    getState: mocks.getState,
    getQueue: mocks.getQueue,
  },
  onPlayerState: mocks.onPlayerState,
  toApiError: (error: unknown) => ({
    message: error instanceof Error ? error.message : String(error),
  }),
}));

import { RepeatMode, Status, type PlayerState, type QueueItem } from "../shared/api/types";
import { player } from "./player.svelte";

function state(overrides: Partial<PlayerState> = {}): PlayerState {
  return {
    provider: "spotify",
    status: Status.StatusPaused,
    track: {
      id: "spotify:track:1",
      title: "Song",
      artists: ["Artist"],
      album: "Album",
      cover: "",
      uri: "spotify:track:1",
      durationMs: 10_000,
    },
    progressMs: 1_000,
    durationMs: 10_000,
    volume: 50,
    shuffle: false,
    repeat: RepeatMode.RepeatOff,
    device: {
      id: "dev",
      name: "Cadence",
      type: "computer",
      active: true,
      volumePercent: 50,
      restricted: false,
    },
    devices: [],
    queue: [],
    canSeek: true,
    canControl: true,
    revision: 1,
    capturedAt: 0,
    ...overrides,
  };
}

function queueItem(index: number): QueueItem {
  return {
    track: {
      id: `spotify:track:${index}`,
      title: `Song ${index}`,
      artists: null as unknown as string[],
      album: "Album",
      cover: "",
      uri: `spotify:track:${index}`,
      durationMs: 10_000,
    },
    index,
    isCurrent: index === 0,
    isNext: index === 1,
  };
}

describe("player store", () => {
  beforeEach(() => {
    player.stop();
    player.apply(state({ revision: 0 }));
    player.queue = [];
    mocks.getQueue.mockReset();
    mocks.getState.mockReset();
    mocks.onPlayerState.mockClear();
  });

  it("drops stale states and accepts equal or newer revisions", () => {
    player.apply(state({ revision: 5, volume: 50 }));

    player.apply(state({ revision: 3, volume: 10 }));
    expect(player.state.revision).toBe(5);
    expect(player.state.volume).toBe(50);

    player.apply(state({ revision: 5, volume: 60 }));
    expect(player.state.volume).toBe(60);

    player.apply(state({ revision: 6, volume: 70 }));
    expect(player.state.revision).toBe(6);
    expect(player.state.volume).toBe(70);
  });

  it("interpolates the position while playing and freezes it when paused", () => {
    vi.useFakeTimers();
    try {
      vi.setSystemTime(new Date("2026-11-04T12:00:00Z"));

      player.apply(
        state({
          status: Status.StatusPlaying,
          progressMs: 1_000,
          capturedAt: Date.now() - 250,
          durationMs: 10_000,
        }),
      );
      expect(player.positionMs).toBe(1_250);

      player.apply(
        state({
          status: Status.StatusPaused,
          progressMs: 1_000,
          capturedAt: Date.now() - 250,
          durationMs: 10_000,
        }),
      );
      expect(player.positionMs).toBe(1_000);

      player.apply(
        state({
          status: Status.StatusPlaying,
          progressMs: 9_900,
          capturedAt: Date.now() - 1_000,
          durationMs: 10_000,
        }),
      );
      expect(player.positionMs).toBe(10_000);
    } finally {
      vi.useRealTimers();
    }
  });

  it("shares one queue request between concurrent callers", async () => {
    let resolveQueue: (items: QueueItem[]) => void = () => {};
    mocks.getQueue.mockReturnValue(
      new Promise<QueueItem[]>((resolve) => {
        resolveQueue = resolve;
      }),
    );

    const first = player.loadQueue();
    const second = player.loadQueue();
    expect(mocks.getQueue).toHaveBeenCalledTimes(1);
    expect(player.queueLoading).toBe(true);

    resolveQueue([queueItem(0), queueItem(1)]);
    await Promise.all([first, second]);

    expect(player.queueLoading).toBe(false);
    expect(player.queue).toHaveLength(2);
    expect(player.queue[0].track.artists).toEqual([]);

    mocks.getQueue.mockResolvedValueOnce([]);
    await player.loadQueue();
    expect(mocks.getQueue).toHaveBeenCalledTimes(2);
  });

  it("retries the queue after a failure", async () => {
    mocks.getQueue.mockRejectedValueOnce(new Error("engine offline"));
    await player.loadQueue();
    expect(player.error).toBe("engine offline");
    expect(player.queueLoading).toBe(false);

    mocks.getQueue.mockResolvedValueOnce([queueItem(0)]);
    await player.loadQueue();
    expect(player.queue).toHaveLength(1);
    expect(player.error).toBeNull();
  });

  it("subscribes before fetching and keeps the subscription after a failure", async () => {
    mocks.getState.mockRejectedValueOnce(new Error("offline"));

    await player.start();
    expect(mocks.onPlayerState).toHaveBeenCalledTimes(1);
    expect(player.ready).toBe(false);
    expect(player.error).toBe("offline");

    mocks.captured.handler?.(state({ revision: 9, volume: 77 }));
    expect(player.state.volume).toBe(77);

    mocks.getState.mockResolvedValueOnce(state({ revision: 10 }));
    await player.start();
    expect(mocks.onPlayerState).toHaveBeenCalledTimes(1);
    expect(player.ready).toBe(true);
  });
});
