import { render, screen } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  RepeatMode,
  Status,
  type PlayerState,
} from "../../../bindings/cadence/internal/player/models";
import Player from "./Player.svelte";

function state(overrides: Partial<PlayerState> = {}): PlayerState {
  return {
    provider: "spotify",
    status: Status.StatusStopped,
    track: {
      id: "",
      title: "",
      artists: [],
      album: "",
      cover: "",
      uri: "",
      durationMs: 0,
    },
    progressMs: 0,
    durationMs: 0,
    volume: 0,
    shuffle: false,
    repeat: RepeatMode.RepeatOff,
    device: {
      id: "",
      name: "",
      type: "",
      active: false,
      volumePercent: 0,
      restricted: false,
    },
    devices: [],
    queue: [],
    canSeek: false,
    canControl: false,
    revision: 1,
    ...overrides,
  } as PlayerState;
}

const noop = vi.fn();

function renderPlayer(playerState: PlayerState) {
  return render(Player, {
    state: playerState,
    visualizerMode: "spectrum",
    visualizerLabel: "Spectrum",
    accent: "#5ef0ff",
    secondary: "#7d8cff",
    reducedMotion: true,
    onToggle: noop,
    onPrevious: noop,
    onNext: noop,
    onSeek: noop,
    onVolume: noop,
    onShuffle: noop,
    onRepeat: noop,
    onCycleVisualizer: noop,
  });
}

describe("Player error states", () => {
  beforeEach(() => {
    noop.mockClear();
    vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue(null);
  });

  it("renders the no-active-device state", () => {
    renderPlayer(
      state({
        error: {
          code: "no_active_device",
          message: "No active Spotify device. Start playback in Spotify or choose a device.",
          retryable: false,
        },
      }),
    );

    expect(screen.getByText("No active device")).not.toBeNull();
    expect(screen.getByText("Choose a Spotify device")).not.toBeNull();
    expect(
      screen.getByText("No active Spotify device. Start playback in Spotify or choose a device."),
    ).not.toBeNull();
  });

  it("renders a network error without losing the controls", () => {
    renderPlayer(
      state({
        error: {
          code: "network",
          message: "Cannot reach Spotify. Check your connection; Cadence will retry.",
          retryable: true,
        },
        canControl: false,
        canSeek: false,
      }),
    );

    expect(
      screen.getByText("Cannot reach Spotify. Check your connection; Cadence will retry."),
    ).not.toBeNull();
    expect(screen.getByRole("button", { name: "Play" })).toBeTruthy();
    expect((screen.getByRole("button", { name: "Play" }) as HTMLButtonElement).disabled).toBe(true);
  });
});
