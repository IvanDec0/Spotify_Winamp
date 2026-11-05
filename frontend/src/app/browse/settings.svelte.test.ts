import { beforeEach, describe, expect, it, vi } from "vitest";
import type { EngineSettings } from "../../shared/api/types";
import { BrowseSettingsStore } from "./settings.svelte";

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

describe("BrowseSettingsStore", () => {
  let store: BrowseSettingsStore;

  beforeEach(() => {
    vi.clearAllMocks();
    store = new BrowseSettingsStore();
  });

  it("loads engine settings and caches the result", async () => {
    mocks.getEngineSettings.mockResolvedValue(settings({ bitrate: 160 }));

    await store.load();
    expect(store.settings?.bitrate).toBe(160);
    expect(store.loading).toBe(false);
    expect(store.error).toBeNull();

    await store.load();
    expect(mocks.getEngineSettings).toHaveBeenCalledTimes(1);
  });

  it("saves the form and marks a restart as required", async () => {
    mocks.getEngineSettings.mockResolvedValue(settings());
    mocks.setEngineSettings.mockResolvedValue(settings({ bitrate: 160 }));
    await store.load();
    store.update("bitrate", 160);

    await store.save();

    expect(mocks.setEngineSettings).toHaveBeenCalledWith(settings({ bitrate: 160 }));
    expect(store.settings?.bitrate).toBe(160);
    expect(store.restartRequired).toBe(true);
    expect(store.notice).toBe("browse.settingsSaved");
  });

  it("restarts the engine explicitly and clears the pending flag", async () => {
    mocks.restart.mockResolvedValue({});
    store.restartRequired = true;

    await store.restart();

    expect(mocks.restart).toHaveBeenCalledOnce();
    expect(store.restartRequired).toBe(false);
    expect(store.notice).toBe("browse.settingsRestarted");
  });

  it("keeps the form and reports errors when saving fails", async () => {
    mocks.getEngineSettings.mockResolvedValue(settings());
    mocks.setEngineSettings.mockRejectedValue(new Error("engine offline"));
    await store.load();
    store.update("bitrate", 160);

    await store.save();

    expect(store.settings?.bitrate).toBe(160);
    expect(store.restartRequired).toBe(false);
    expect(store.error).toBe("engine offline");
  });

  it("reports load errors and allows a forced retry", async () => {
    mocks.getEngineSettings
      .mockRejectedValueOnce(new Error("settings unavailable"))
      .mockResolvedValueOnce(settings());

    await store.load();
    expect(store.error).toBe("settings unavailable");
    expect(store.settings).toBeNull();

    await store.load(true);
    expect(store.settings?.bitrate).toBe(320);
    expect(store.error).toBeNull();
  });
});
