import { fireEvent, render, screen } from "@testing-library/svelte";
import { tick } from "svelte";
import { afterEach, describe, expect, it, vi } from "vitest";
import Progress from "./Progress.svelte";

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

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("Progress", () => {
  it("shows position labels and exposes an accessible range", async () => {
    render(Progress, {
      props: {
        positionMs: 30_000,
        durationMs: 180_000,
        playing: false,
        canSeek: true,
        onSeek: vi.fn(),
      },
    });

    await new Promise((resolve) => requestAnimationFrame(resolve));

    const slider = screen.getByRole("slider", { name: "Playback position" });
    expect(slider.getAttribute("aria-valuetext")).toBe("0:30 of 3:00");
    expect((slider as HTMLInputElement).value).toBe("30000");
  });

  it("commits a seek when the range changes", async () => {
    const onSeek = vi.fn();
    render(Progress, {
      props: {
        positionMs: 30_000,
        durationMs: 180_000,
        playing: false,
        canSeek: true,
        onSeek,
      },
    });

    const slider = screen.getByRole("slider", { name: "Playback position" }) as HTMLInputElement;
    await fireEvent.input(slider, { target: { value: "90000" } });
    await fireEvent.change(slider, { target: { value: "90000" } });

    expect(onSeek).toHaveBeenCalledWith(90_000);
  });

  it("does not seek when the provider cannot seek", async () => {
    const onSeek = vi.fn();
    render(Progress, {
      props: {
        positionMs: 0,
        durationMs: 180_000,
        playing: false,
        canSeek: false,
        onSeek,
      },
    });

    const slider = screen.getByRole("slider", { name: "Playback position" }) as HTMLInputElement;
    expect(slider.disabled).toBe(true);
    await fireEvent.change(slider, { target: { value: "90000" } });
    expect(onSeek).not.toHaveBeenCalled();
  });

  it("does not schedule animation frames while paused", async () => {
    const { raf, cancel } = stubFrames();

    render(Progress, {
      props: {
        positionMs: 30_000,
        durationMs: 180_000,
        playing: false,
        canSeek: true,
        onSeek: vi.fn(),
      },
    });
    await tick();

    expect(raf).not.toHaveBeenCalled();
    expect(cancel).not.toHaveBeenCalled();
    const slider = screen.getByRole("slider", { name: "Playback position" }) as HTMLInputElement;
    expect(slider.value).toBe("30000");
  });

  it("cancels the frame loop when playback pauses", async () => {
    const { raf, cancel, scheduled } = stubFrames();

    const result = render(Progress, {
      props: {
        positionMs: 30_000,
        durationMs: 180_000,
        playing: true,
        canSeek: true,
        onSeek: vi.fn(),
      },
    });
    await tick();
    expect(raf).toHaveBeenCalledTimes(1);
    const pending = scheduled[0];

    await result.rerender({ playing: false });
    await tick();
    expect(cancel).toHaveBeenCalledTimes(1);

    // A frame already queued before the pause must not reschedule itself.
    pending(performance.now());
    expect(raf).toHaveBeenCalledTimes(1);
  });

  it("stops the loop when the document is hidden", async () => {
    const { cancel } = stubFrames();
    render(Progress, {
      props: {
        positionMs: 30_000,
        durationMs: 180_000,
        playing: true,
        canSeek: true,
        onSeek: vi.fn(),
      },
    });
    await tick();
    cancel.mockClear();

    const hidden = vi.spyOn(document, "hidden", "get").mockReturnValue(true);
    document.dispatchEvent(new Event("visibilitychange"));
    await tick();
    hidden.mockRestore();

    expect(cancel).toHaveBeenCalled();
  });
});
