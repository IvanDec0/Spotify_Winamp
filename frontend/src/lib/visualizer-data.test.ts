import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => {
  const unsubscribe = vi.fn();
  const onVisualizerFrame = vi.fn(() => unsubscribe);
  const setActive = vi.fn(() => Promise.resolve());
  return { onVisualizerFrame, setActive, unsubscribe };
});

vi.mock("../shared/api", () => ({
  onVisualizerFrame: mocks.onVisualizerFrame,
  visualizer: { setActive: mocks.setActive },
}));

import { VISUALIZER_FRESHNESS_MS, VisualizerDataStore } from "./visualizer-data.svelte";

function frame(at: number) {
  return { rms: 0.4, bands: [0.1, 0.2], waveform: [0, 0.1], at };
}

describe("visualizer data store", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("deduplicates activation and releases the client once", () => {
    const store = new VisualizerDataStore();

    store.setActive(true);
    store.setActive(true);
    expect(mocks.setActive).toHaveBeenCalledOnce();
    expect(mocks.setActive).toHaveBeenCalledWith(store.clientId, true);

    store.setActive(false);
    store.setActive(false);
    expect(mocks.setActive).toHaveBeenCalledTimes(2);
    expect(mocks.setActive).toHaveBeenLastCalledWith(store.clientId, false);
  });

  it("uses a unique client id per store", () => {
    const first = new VisualizerDataStore();
    const second = new VisualizerDataStore();

    expect(first.clientId).not.toBe("");
    expect(first.clientId).not.toBe(second.clientId);
  });

  it("reports freshness inside the window only", () => {
    const store = new VisualizerDataStore();
    expect(store.isFresh()).toBe(false);

    store.frame = frame(10_000);
    expect(store.isFresh(10_000)).toBe(true);
    expect(store.isFresh(10_000 + VISUALIZER_FRESHNESS_MS)).toBe(true);
    expect(store.isFresh(10_000 + VISUALIZER_FRESHNESS_MS + 1)).toBe(false);
  });

  it("unsubscribes and clears state on stop", () => {
    const store = new VisualizerDataStore();
    store.setActive(true);
    store.frame = frame(Date.now());
    expect(mocks.onVisualizerFrame).toHaveBeenCalledOnce();

    store.stop();

    expect(mocks.unsubscribe).toHaveBeenCalledOnce();
    expect(store.frame).toBeNull();
    expect(mocks.setActive).toHaveBeenLastCalledWith(store.clientId, false);

    store.setActive(true);
    expect(mocks.onVisualizerFrame).toHaveBeenCalledTimes(2);
  });

  it("releases the window on pagehide", () => {
    const store = new VisualizerDataStore();
    store.setActive(true);
    store.frame = frame(Date.now());
    mocks.setActive.mockClear();

    window.dispatchEvent(new Event("pagehide"));

    expect(mocks.setActive).toHaveBeenCalledWith(store.clientId, false);
    expect(store.frame).toBeNull();
  });
});
