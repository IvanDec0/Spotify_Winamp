import { fireEvent, render, screen, waitFor } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import EngineGate from "./EngineGate.svelte";

const mocks = vi.hoisted(() => ({
  music: {
    authenticated: false,
    loading: false,
    busy: false,
    error: null as string | null,
    audio: {
      available: true,
      binaryPath: "/tmp/go-librespot",
      error: "",
      errorCode: "",
    },
    pairing: null as { code: string; url: string } | null,
    notice: null as { key: string; params?: Record<string, string | number> } | null,
    openPairing: vi.fn(() => Promise.resolve()),
    resetSession: vi.fn(() => Promise.resolve()),
    restart: vi.fn(() => Promise.resolve()),
    refresh: vi.fn(() => Promise.resolve()),
  },
}));

vi.mock("../../lib/music.svelte", () => ({ music: mocks.music }));

describe("EngineGate", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.music.authenticated = false;
    mocks.music.loading = false;
    mocks.music.busy = false;
    mocks.music.error = null;
    mocks.music.audio.available = true;
    mocks.music.audio.binaryPath = "/tmp/go-librespot";
    mocks.music.audio.error = "";
    mocks.music.audio.errorCode = "";
    mocks.music.pairing = null;
    mocks.music.notice = null;
  });

  it("offers an engine retry when the audio engine is unavailable", async () => {
    mocks.music.audio.available = false;

    render(EngineGate);

    expect(screen.getByRole("heading", { name: "Local audio engine unavailable" })).not.toBeNull();
    expect(screen.getByText("/tmp/go-librespot")).not.toBeNull();

    await fireEvent.click(screen.getByRole("button", { name: "Retry engine" }));
    expect(mocks.music.restart).toHaveBeenCalledOnce();
  });

  it("shows the pairing code and pairing actions", async () => {
    mocks.music.pairing = { code: "ABCD-1234", url: "https://spotify.example/pair" };

    const writeText = vi.fn(() => Promise.resolve());
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });

    render(EngineGate);

    expect(screen.getByText("ABCD-1234")).not.toBeNull();
    expect(screen.getByText("https://spotify.example/pair")).not.toBeNull();

    await fireEvent.click(screen.getByRole("button", { name: "Open pairing page" }));
    expect(mocks.music.openPairing).toHaveBeenCalledOnce();

    await fireEvent.click(screen.getByRole("button", { name: "Copy code" }));
    await waitFor(() => expect(writeText).toHaveBeenCalledWith("ABCD-1234"));
    expect(screen.getByText("Code copied")).not.toBeNull();

    await fireEvent.click(screen.getByRole("button", { name: "Reset pairing" }));
    expect(screen.getByRole("button", { name: "Confirm reset" })).not.toBeNull();
    await fireEvent.click(screen.getByRole("button", { name: "Confirm reset" }));
    expect(mocks.music.resetSession).toHaveBeenCalledOnce();
  });

  it("translates a known engine error code and keeps the raw detail", () => {
    mocks.music.audio.errorCode = "engine_start_failed";
    mocks.music.audio.error = "cannot start go-librespot: permission denied";

    render(EngineGate);

    expect(
      screen.getByRole("heading", { name: "The audio engine could not start" }),
    ).not.toBeNull();
    expect(screen.getByText("Technical detail")).not.toBeNull();
    expect(screen.getByText("cannot start go-librespot: permission denied")).not.toBeNull();
    expect(screen.getByRole("button", { name: "Retry engine" })).not.toBeNull();
  });

  it("falls back to a generic headline for an unknown engine code", () => {
    mocks.music.audio.errorCode = "engine_mystery";
    mocks.music.audio.error = "the engine did something unexpected";

    render(EngineGate);

    expect(
      screen.getByRole("heading", { name: "The audio engine reported an error" }),
    ).not.toBeNull();
    expect(screen.getByText("the engine did something unexpected")).not.toBeNull();
  });

  it("shows demo-mode guidance without a retry action", () => {
    mocks.music.audio.available = false;
    mocks.music.audio.errorCode = "engine_disabled";
    mocks.music.audio.error = "The local audio engine is disabled in demo mode.";

    render(EngineGate);

    expect(
      screen.getByRole("heading", { name: "The audio engine is disabled in demo mode" }),
    ).not.toBeNull();
    expect(screen.queryByRole("button", { name: "Retry engine" })).toBeNull();
    expect(
      screen.getByText(
        "Cadence is running in demo mode. Restart without --demo to use the local audio engine.",
      ),
    ).not.toBeNull();
  });

  it("translates the pairing reset notice", () => {
    mocks.music.pairing = { code: "ABCD-1234", url: "https://spotify.example/pair" };
    mocks.music.notice = { key: "music.pairingReset" };

    render(EngineGate);

    expect(screen.getByText("Pairing reset. Use the new code to connect Spotify.")).not.toBeNull();
  });

  it("renders nothing once the engine is authenticated", () => {
    mocks.music.authenticated = true;

    const { container } = render(EngineGate);

    expect(container.querySelector(".engine-gate")).toBeNull();
  });
});
