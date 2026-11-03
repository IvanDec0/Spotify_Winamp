import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import Volume from "./Volume.svelte";

describe("Volume", () => {
  it("reports slider changes", async () => {
    const onVolume = vi.fn();
    render(Volume, { volume: 40, canControl: true, onVolume });

    const slider = screen.getByRole("slider", { name: "Volume level" }) as HTMLInputElement;
    expect(slider.value).toBe("40");

    await fireEvent.input(slider, { target: { value: "73" } });

    expect(onVolume).toHaveBeenCalledWith(73);
  });

  it("mutes and restores the previous level", async () => {
    const onVolume = vi.fn();
    render(Volume, { volume: 40, canControl: true, onVolume });

    await fireEvent.click(screen.getByRole("button", { name: "Mute" }));
    expect(onVolume).toHaveBeenLastCalledWith(0);

    await fireEvent.click(screen.getByRole("button", { name: "Unmute" }));
    expect(onVolume).toHaveBeenLastCalledWith(40);
  });

  it("disables controls when playback is unavailable", () => {
    render(Volume, { volume: 0, canControl: false, onVolume: vi.fn() });

    expect((screen.getByRole("slider") as HTMLInputElement).disabled).toBe(true);
    expect((screen.getByRole("button") as HTMLButtonElement).disabled).toBe(true);
  });
});
