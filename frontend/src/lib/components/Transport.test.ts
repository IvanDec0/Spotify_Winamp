import { fireEvent, render, screen } from "@testing-library/svelte";
import { afterEach, describe, expect, it, vi } from "vitest";
import { RepeatMode } from "../../shared/api/types";
import { i18n } from "../../shared/i18n";
import Transport from "./Transport.svelte";

const defaults = {
  playing: false,
  shuffle: false,
  repeat: RepeatMode.RepeatOff,
  canControl: true,
  onToggle: vi.fn(),
  onPrevious: vi.fn(),
  onNext: vi.fn(),
  onShuffle: vi.fn(),
  onRepeat: vi.fn(),
};

afterEach(async () => {
  await i18n.setLocale("en");
});

describe("Transport", () => {
  it("renders Spanish controls after a locale switch", async () => {
    await i18n.setLocale("es");
    render(Transport, { props: defaults });

    expect(screen.getByRole("button", { name: "Reproducir" })).not.toBeNull();
    expect(screen.getByRole("button", { name: "Pista anterior" })).not.toBeNull();
    expect(screen.getByRole("button", { name: "Pista siguiente" })).not.toBeNull();
    expect(screen.getByRole("button", { name: "Activar aleatorio" })).not.toBeNull();
  });

  it("exposes labelled controls and reports clicks", async () => {
    const props = { ...defaults, onToggle: vi.fn(), onShuffle: vi.fn() };
    render(Transport, { props });

    expect(screen.getByRole("button", { name: "Play" })).not.toBeNull();
    expect(screen.getByRole("button", { name: "Previous track" })).not.toBeNull();
    expect(screen.getByRole("button", { name: "Next track" })).not.toBeNull();
    expect(screen.getByRole("button", { name: "Turn shuffle on" })).not.toBeNull();

    await fireEvent.click(screen.getByRole("button", { name: "Play" }));
    expect(props.onToggle).toHaveBeenCalledOnce();

    await fireEvent.click(screen.getByRole("button", { name: "Turn shuffle on" }));
    expect(props.onShuffle).toHaveBeenCalledOnce();
  });

  it("reflects shuffle, repeat and playing state", () => {
    render(Transport, {
      props: {
        ...defaults,
        playing: true,
        shuffle: true,
        repeat: RepeatMode.RepeatTrack,
      },
    });

    expect(screen.getByRole("button", { name: "Pause" })).not.toBeNull();
    expect(
      screen.getByRole("button", { name: "Turn shuffle off" }).getAttribute("aria-pressed"),
    ).toBe("true");
    expect(
      screen.getByRole("button", { name: "Repeat one track" }).getAttribute("aria-pressed"),
    ).toBe("true");
    expect(screen.getByText("1")).not.toBeNull();
  });

  it("disables transport when the provider is not controllable", () => {
    render(Transport, { props: { ...defaults, canControl: false } });
    expect((screen.getByRole("button", { name: "Play" }) as HTMLButtonElement).disabled).toBe(true);
  });
});
