import { render } from "@testing-library/svelte";
import { tick } from "svelte";
import { afterEach, describe, expect, it, vi } from "vitest";
import Visualizer from "./Visualizer.svelte";

function stubFrames(): {
  raf: ReturnType<typeof vi.fn>;
  cancel: ReturnType<typeof vi.fn>;
  scheduled: FrameRequestCallback[];
} {
  const scheduled: FrameRequestCallback[] = [];
  const raf = vi.fn((callback: FrameRequestCallback) => {
    scheduled.push(callback);
    return scheduled.length;
  });
  const cancel = vi.fn();
  vi.stubGlobal("requestAnimationFrame", raf);
  vi.stubGlobal("cancelAnimationFrame", cancel);
  return { raf, cancel, scheduled };
}

function stubCanvas(): void {
  const context = {
    setTransform: vi.fn(),
    clearRect: vi.fn(),
    fillRect: vi.fn(),
    beginPath: vi.fn(),
    arc: vi.fn(),
    stroke: vi.fn(),
    fill: vi.fn(),
    moveTo: vi.fn(),
    lineTo: vi.fn(),
    createRadialGradient: vi.fn(() => ({ addColorStop: vi.fn() })),
    fillStyle: "",
    strokeStyle: "",
    lineWidth: 0,
    lineJoin: "",
    lineCap: "",
    globalAlpha: 1,
  };
  vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue(
    context as unknown as CanvasRenderingContext2D,
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

const baseProps = {
  mode: "spectrum" as const,
  trackId: "phase-4-3",
  playing: true,
  reducedMotion: false,
  accent: "#5ef0ff",
  secondary: "#7d8cff",
};

describe("Visualizer", () => {
  it("runs frames only while playing and visible", async () => {
    stubCanvas();
    const { raf, cancel, scheduled } = stubFrames();

    const result = render(Visualizer, { props: baseProps });
    await tick();
    expect(raf).toHaveBeenCalledTimes(1);

    await result.rerender({ playing: false });
    await tick();
    expect(cancel).toHaveBeenCalled();
    scheduled[0](performance.now());
    expect(raf).toHaveBeenCalledTimes(1);

    const hidden = vi.spyOn(document, "hidden", "get").mockReturnValue(true);
    document.dispatchEvent(new Event("visibilitychange"));
    await tick();
    const afterHide = raf.mock.calls.length;

    await result.rerender({ playing: true });
    await tick();
    expect(raf).toHaveBeenCalledTimes(afterHide);

    hidden.mockReturnValue(false);
    document.dispatchEvent(new Event("visibilitychange"));
    await tick();
    expect(raf.mock.calls.length).toBeGreaterThan(afterHide);
  });

  it("does not schedule frames when reduced motion is requested", async () => {
    stubCanvas();
    const { raf } = stubFrames();

    render(Visualizer, { props: { ...baseProps, reducedMotion: true } });
    await tick();

    expect(raf).not.toHaveBeenCalled();
  });
});
