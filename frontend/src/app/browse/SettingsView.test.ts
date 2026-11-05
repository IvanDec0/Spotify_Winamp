import { fireEvent, render, screen, waitFor } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { EngineSettings } from "../../shared/api/types";
import { browseSettings } from "./settings.svelte";
import SettingsView from "./SettingsView.svelte";

const mocks = vi.hoisted(() => ({
  getEngineSettings: vi.fn(),
  setEngineSettings: vi.fn(),
  restart: vi.fn(),
}));

vi.mock("../../shared/api", () => ({
  audio: mocks,
  toApiError: (error: unknown) => (error instanceof Error ? error : new Error(String(error))),
}));

function settings(overrides: Partial<EngineSettings> = {}): EngineSettings {
  return {
    bitrate: 320,
    crossfadeDurationMs: 0,
    normalisationDisabled: false,
    normalisationUseAlbumGain: false,
    normalisationPregain: 0,
    cacheEnabled: true,
    cacheDir: "/tmp/cadence-cache",
    cacheSizeLimit: "1GB",
    flacEnabled: false,
    mprisEnabled: true,
    deviceIsGroup: false,
    imageSize: "default",
    ...overrides,
  };
}

async function renderSettings(): Promise<void> {
  render(SettingsView);
  await waitFor(() => expect(screen.getByRole("button", { name: "Save settings" })).not.toBeNull());
}

describe("SettingsView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    browseSettings.reset();
    mocks.getEngineSettings.mockResolvedValue(settings());
    mocks.setEngineSettings.mockImplementation((value: EngineSettings) => Promise.resolve(value));
    mocks.restart.mockResolvedValue({});
  });

  it("renders the playback, cache and feature fields", async () => {
    await renderSettings();

    expect(screen.getByLabelText("Bitrate")).not.toBeNull();
    expect(screen.getByLabelText("Crossfade")).not.toBeNull();
    expect(screen.getByLabelText("Disable normalisation")).not.toBeNull();
    expect(screen.getByLabelText("Use album gain")).not.toBeNull();
    expect(screen.getByLabelText("Enable disk cache")).not.toBeNull();
    expect(screen.getByLabelText("Size limit")).not.toBeNull();
    expect(screen.getByText("/tmp/cadence-cache")).not.toBeNull();
    expect(screen.getByLabelText("FLAC")).not.toBeNull();
    expect(screen.getByLabelText("MPRIS")).not.toBeNull();
    expect(screen.getByLabelText("Advertise as a speaker group")).not.toBeNull();
    expect(screen.getByLabelText("Image size")).not.toBeNull();
    expect(screen.getByRole("button", { name: "Restart engine" }).hasAttribute("disabled")).toBe(
      true,
    );
  });

  it("saves the form and requires a confirmed restart", async () => {
    await renderSettings();

    await fireEvent.change(screen.getByLabelText("Bitrate"), { target: { value: "160" } });
    await fireEvent.click(screen.getByRole("button", { name: "Save settings" }));

    await waitFor(() =>
      expect(mocks.setEngineSettings).toHaveBeenCalledWith(settings({ bitrate: 160 })),
    );
    expect(screen.getByText("Settings saved. Restart the engine to apply them.")).not.toBeNull();

    const restart = screen.getByRole("button", { name: "Restart engine" });
    expect(restart.hasAttribute("disabled")).toBe(false);
    await fireEvent.click(restart);

    expect(screen.getByRole("dialog", { name: "Restart the audio engine?" })).not.toBeNull();
    expect(mocks.restart).not.toHaveBeenCalled();

    await fireEvent.click(screen.getByRole("button", { name: "Restart now" }));
    await waitFor(() => expect(mocks.restart).toHaveBeenCalledOnce());
    expect(browseSettings.restartRequired).toBe(false);
    expect(screen.getByText("Audio engine restarted.")).not.toBeNull();
  });

  it("shows a retry path when loading fails", async () => {
    mocks.getEngineSettings
      .mockRejectedValueOnce(new Error("settings unavailable"))
      .mockResolvedValueOnce(settings());

    render(SettingsView);
    await waitFor(() =>
      expect(screen.getByText("Could not load the engine settings.")).not.toBeNull(),
    );
    expect(screen.getByText("settings unavailable")).not.toBeNull();

    await fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Save settings" })).not.toBeNull(),
    );
  });

  it("keeps the form and announces save failures", async () => {
    mocks.setEngineSettings.mockRejectedValue(new Error("engine offline"));
    await renderSettings();

    await fireEvent.change(screen.getByLabelText("Bitrate"), { target: { value: "96" } });
    await fireEvent.click(screen.getByRole("button", { name: "Save settings" }));

    await waitFor(() => expect(screen.getByRole("alert").textContent).toContain("engine offline"));
    expect(browseSettings.settings?.bitrate).toBe(96);
    expect(screen.getByLabelText("Bitrate")).not.toBeNull();
  });
});
