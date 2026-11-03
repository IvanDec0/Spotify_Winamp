import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import Progress from "./Progress.svelte";

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
});
