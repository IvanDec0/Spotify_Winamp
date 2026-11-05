import { render } from "@testing-library/svelte";
import { tick } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { preferences } from "../../lib/preferences.svelte";
import BrowseVisualizer from "./BrowseVisualizer.svelte";

const mocks = vi.hoisted(() => {
  const setActive = vi.fn();
  const isFresh = vi.fn(() => false);
  const store = {
    frame: null as {
      rms: number;
      bands: number[];
      waveform: number[];
      at: number;
    } | null,
    isFresh,
    setActive,
  };
  return { store, setActive, isFresh };
});

vi.mock("../../lib/player.svelte", async () => {
  const { playerMock } = await import("../../lib/testing/player-mock.svelte");
  return { player: playerMock };
});

vi.mock("../../lib/visualizer-data.svelte", () => ({ visualizerData: mocks.store }));

function stubFrames(): { scheduled: FrameRequestCallback[]; cancel: ReturnType<typeof vi.fn> } {
  const scheduled: FrameRequestCallback[] = [];
  const raf = vi.fn((callback: FrameRequestCallback) => {
    scheduled.push(callback);
    return scheduled.length;
  });
  const cancel = vi.fn();
  vi.stubGlobal("requestAnimationFrame", raf);
  vi.stubGlobal("cancelAnimationFrame", cancel);
  return { scheduled, cancel };
}

function stubCanvas(): ReturnType<typeof vi.fn> {
  const fillRect = vi.fn();
  const context = {
    setTransform: vi.fn(),
    clearRect: vi.fn(),
    fillRect,
    fillStyle: "",
    globalAlpha: 1,
  };
  vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue(
    context as unknown as CanvasRenderingContext2D,
  );
  vi.spyOn(HTMLCanvasElement.prototype, "getBoundingClientRect").mockReturnValue({
    width: 72,
    height: 28,
    top: 0,
    left: 0,
    right: 72,
    bottom: 28,
    x: 0,
    y: 0,
    toJSON: () => ({}),
  } as DOMRect);
  return fillRect;
}

let frameClock = 0;

function drawNextFrame(scheduled: FrameRequestCallback[]): void {
  const callback = scheduled.at(-1);
  if (!callback) throw new Error("no scheduled frame");
  frameClock += 1000;
  callback(frameClock);
}

beforeEach(() => {
  vi.clearAllMocks();
  frameClock = 0;
  mocks.store.frame = null;
  mocks.isFresh.mockReturnValue(false);
  preferences.setMotion("system");
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("BrowseVisualizer", () => {
  it("activates only while playing, visible and motion is allowed", async () => {
    stubCanvas();
    stubFrames();

    const result = render(BrowseVisualizer);
    await tick();

    expect(mocks.setActive).toHaveBeenCalledWith(true);

    result.unmount();
    expect(mocks.setActive).toHaveBeenLastCalledWith(false);
  });

  it("draws the simulated fallback when no fresh frame exists", async () => {
    const fillRect = stubCanvas();
    const { scheduled } = stubFrames();

    render(BrowseVisualizer);
    await tick();
    drawNextFrame(scheduled);

    const heights = fillRect.mock.calls.map((call) => call[3] as number);
    expect(heights.some((height) => height > 1)).toBe(true);
  });

  it("draws the real bands when the frame is fresh", async () => {
    const fillRect = stubCanvas();
    const { scheduled } = stubFrames();

    render(BrowseVisualizer);
    await tick();
    drawNextFrame(scheduled);

    mocks.isFresh.mockReturnValue(true);
    mocks.store.frame = {
      rms: 0,
      bands: new Array(24).fill(0),
      waveform: new Array(64).fill(0),
      at: Date.now(),
    };
    fillRect.mockClear();
    drawNextFrame(scheduled);

    const heights = fillRect.mock.calls.map((call) => call[3] as number);
    expect(heights.length).toBeGreaterThan(0);
    expect(heights.every((height) => height === 1)).toBe(true);
  });

  it("does not animate with reduced motion", async () => {
    stubCanvas();
    const { scheduled } = stubFrames();
    preferences.setMotion("reduced");

    render(BrowseVisualizer);
    await tick();

    expect(scheduled).toHaveLength(0);
    expect(mocks.setActive).toHaveBeenCalledWith(false);
  });
});
