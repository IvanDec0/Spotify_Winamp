import { render, screen } from "@testing-library/svelte";
import { tick } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { Status } from "../../../shared/api/types";
import { createPlayerState, playerMock } from "../../testing/player-mock.svelte";
import Progress from "./Progress.svelte";

vi.mock("../../player.svelte", async () => {
  const { playerMock: player } = await import("../../testing/player-mock.svelte");
  return { player };
});

describe("skin Progress", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-11-04T12:00:00Z"));
    playerMock.state = {
      ...createPlayerState(),
      status: Status.StatusPlaying,
      progressMs: 1_000,
      capturedAt: Date.now() - 1_000,
      durationMs: 180_000,
    };
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("interpolates the position while playing", async () => {
    render(Progress);
    await tick();

    const slider = screen.getByRole("slider", { name: "Playback position" });
    expect(slider.getAttribute("aria-valuetext")).toBe("0:02 of 3:00");

    await vi.advanceTimersByTimeAsync(1_000);
    expect(slider.getAttribute("aria-valuetext")).toBe("0:03 of 3:00");
  });

  it("freezes the position when paused", async () => {
    playerMock.state = {
      ...playerMock.state,
      status: Status.StatusPaused,
      progressMs: 1_000,
    };
    render(Progress);
    await tick();

    const slider = screen.getByRole("slider", { name: "Playback position" });
    expect(slider.getAttribute("aria-valuetext")).toBe("0:01 of 3:00");

    await vi.advanceTimersByTimeAsync(1_000);
    expect(slider.getAttribute("aria-valuetext")).toBe("0:01 of 3:00");
  });

  it("updates the shown position when the paused state changes", async () => {
    playerMock.state = {
      ...createPlayerState(),
      status: Status.StatusPaused,
      progressMs: 1_000,
      capturedAt: 0,
      durationMs: 180_000,
    };
    render(Progress);
    await tick();

    const slider = screen.getByRole("slider", { name: "Playback position" });
    expect(slider.getAttribute("aria-valuetext")).toBe("0:01 of 3:00");

    playerMock.state = {
      ...playerMock.state,
      progressMs: 61_000,
    };
    await tick();
    expect(slider.getAttribute("aria-valuetext")).toBe("1:01 of 3:00");
  });

  it("clears the playback interval on unmount", async () => {
    playerMock.state = {
      ...createPlayerState(),
      status: Status.StatusPlaying,
      progressMs: 1_000,
      capturedAt: Date.now() - 1_000,
      durationMs: 180_000,
    };
    const result = render(Progress);
    await tick();
    expect(vi.getTimerCount()).toBe(1);

    result.unmount();
    expect(vi.getTimerCount()).toBe(0);
  });
});
