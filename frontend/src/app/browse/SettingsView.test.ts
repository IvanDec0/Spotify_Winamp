import { fireEvent, render, screen, waitFor } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { EngineSettings } from "../../shared/api/types";
import { i18n } from "../../shared/i18n";
import { browseSettings } from "./settings.svelte";
import SettingsView from "./SettingsView.svelte";

const mocks = vi.hoisted(() => {
  const preferences = {
    theme: "graphite",
    visualizer: "spectrum",
    motion: "system",
    systemReducedMotion: false,
    setTheme: vi.fn(),
    setVisualizer: vi.fn(),
    setMotion: vi.fn(),
  };
  const skins = {
    isDeclarative: false,
    theme: "graphite",
    runtime: { themes: ["graphite", "ice"] },
    catalog: null as unknown,
    loading: false,
    error: null as string | null,
    notice: null as string | null,
    setTheme: vi.fn(() => Promise.resolve()),
    activate: vi.fn(() => Promise.resolve()),
    installZip: vi.fn(() => Promise.resolve()),
    installDirectory: vi.fn(() => Promise.resolve()),
    uninstall: vi.fn(() => Promise.resolve()),
    reset: vi.fn(() => Promise.resolve()),
    setThirdPartyDisabled: vi.fn(() => Promise.resolve()),
    dismissError: vi.fn(),
    dismissNotice: vi.fn(),
  };
  const themes = [
    { id: "graphite", label: "Graphite", accent: "#5ef0ff", secondary: "#7d8cff" },
    { id: "ice", label: "Ice", accent: "#8be9ff", secondary: "#4d9fff" },
  ];
  const visualizerModes = [
    { id: "spectrum", label: "Spectrum", glyph: "▥" },
    { id: "radial", label: "Radial", glyph: "◎" },
    { id: "oscilloscope", label: "Scope", glyph: "∿" },
    { id: "grid", label: "Grid", glyph: "⣿" },
  ];
  return {
    getEngineSettings: vi.fn(),
    setEngineSettings: vi.fn(),
    restart: vi.fn(),
    preferences,
    skins,
    themes,
    visualizerModes,
  };
});

vi.mock("../../shared/api", () => ({
  audio: {
    getEngineSettings: mocks.getEngineSettings,
    setEngineSettings: mocks.setEngineSettings,
    restart: mocks.restart,
  },
  toApiError: (error: unknown) => (error instanceof Error ? error : new Error(String(error))),
}));

vi.mock("../../lib/preferences.svelte", () => ({
  preferences: mocks.preferences,
  themes: mocks.themes,
  visualizerModes: mocks.visualizerModes,
}));

vi.mock("../../lib/skins.svelte", () => ({ skins: mocks.skins }));

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

function skinCatalog() {
  return {
    skins: [
      {
        id: "clarity",
        name: "Clarity",
        version: "1.0.0",
        description: "Built-in",
        author: "Cadence",
        license: "CC BY-SA 4.0",
        source: "builtin",
        builtin: true,
        removable: false,
        active: true,
        themes: ["graphite"],
        defaultTheme: "graphite",
        visualizerModes: ["spectrum"],
      },
      {
        id: "afterglow",
        name: "Afterglow",
        version: "2.0.0",
        description: "Warm skin",
        author: "Cadence",
        license: "MIT",
        source: "installed",
        builtin: false,
        removable: true,
        active: false,
        themes: ["ember"],
        defaultTheme: "ember",
        visualizerModes: ["spectrum"],
      },
    ],
    activeId: "clarity",
    activeTheme: "graphite",
    safeMode: false,
    thirdPartyDisabled: false,
    warnings: [],
    lastError: "",
    canInstall: true,
    canChangeActiveSkin: true,
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

    mocks.preferences.theme = "graphite";
    mocks.preferences.visualizer = "spectrum";
    mocks.preferences.motion = "system";
    mocks.preferences.systemReducedMotion = false;
    mocks.skins.isDeclarative = false;
    mocks.skins.theme = "graphite";
    mocks.skins.runtime = { themes: ["graphite", "ice"] };
    mocks.skins.catalog = skinCatalog();
    mocks.skins.loading = false;
    mocks.skins.error = null;
    mocks.skins.notice = null;
  });

  it("renders playback, cache, feature, appearance and skin fields", async () => {
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
    expect(screen.getByLabelText("Language")).not.toBeNull();
    expect(screen.getByLabelText("Theme")).not.toBeNull();
    expect(screen.getByLabelText("Visualizer")).not.toBeNull();
    expect(screen.getByLabelText("Motion")).not.toBeNull();
    expect(screen.getByText("Afterglow")).not.toBeNull();
    expect(screen.getByText("Active")).not.toBeNull();
    expect(screen.getByRole("button", { name: "Restart engine" }).hasAttribute("disabled")).toBe(
      true,
    );
  });

  it("saves engine settings and requires a confirmed restart", async () => {
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

  it("switches the language from the appearance section", async () => {
    const setLocale = vi.spyOn(i18n, "setLocale").mockResolvedValue();
    await renderSettings();

    await fireEvent.change(screen.getByLabelText("Language"), { target: { value: "es" } });

    expect(setLocale).toHaveBeenCalledWith("es");
  });

  it("applies appearance changes immediately", async () => {
    await renderSettings();

    await fireEvent.change(screen.getByLabelText("Theme"), { target: { value: "ice" } });
    expect(mocks.preferences.setTheme).toHaveBeenCalledWith("ice");

    await fireEvent.change(screen.getByLabelText("Visualizer"), { target: { value: "radial" } });
    expect(mocks.preferences.setVisualizer).toHaveBeenCalledWith("radial");

    await fireEvent.change(screen.getByLabelText("Motion"), { target: { value: "reduced" } });
    expect(mocks.preferences.setMotion).toHaveBeenCalledWith("reduced");
  });

  it("uses declarative skin themes when a skin is active", async () => {
    mocks.skins.isDeclarative = true;
    mocks.skins.theme = "azure";
    mocks.skins.runtime = { themes: ["azure", "ember"] };
    await renderSettings();

    await fireEvent.change(screen.getByLabelText("Theme"), { target: { value: "ember" } });

    expect(mocks.skins.setTheme).toHaveBeenCalledWith("ember");
    expect(mocks.preferences.setTheme).not.toHaveBeenCalled();
  });

  it("activates, installs, resets and uninstalls skins", async () => {
    await renderSettings();

    await fireEvent.click(screen.getByRole("button", { name: "Activate Afterglow" }));
    expect(mocks.skins.activate).toHaveBeenCalledWith("afterglow");

    await fireEvent.click(screen.getByRole("button", { name: "Install .zip…" }));
    expect(mocks.skins.installZip).toHaveBeenCalledOnce();

    await fireEvent.click(screen.getByRole("button", { name: "Install folder…" }));
    expect(mocks.skins.installDirectory).toHaveBeenCalledOnce();

    await fireEvent.click(screen.getByRole("button", { name: "Reset skins" }));
    expect(mocks.skins.reset).toHaveBeenCalledOnce();

    await fireEvent.click(screen.getByRole("button", { name: "Uninstall Afterglow" }));
    expect(mocks.skins.uninstall).toHaveBeenCalledWith("afterglow");

    await fireEvent.click(screen.getByLabelText("Disable third-party skins"));
    expect(mocks.skins.setThirdPartyDisabled).toHaveBeenCalledWith(true);
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
    expect(screen.getByLabelText("Theme")).not.toBeNull();
    expect(screen.getByText("Afterglow")).not.toBeNull();

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
