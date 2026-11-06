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
    errorKey: null as string | null,
    notice: null as { key: string; params?: Record<string, string | number> } | null,
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
  const music = {
    audio: { playbackBackend: "librespot" },
    soloist: null as unknown,
    get soloistMode(): boolean {
      return this.audio.playbackBackend === "soloist";
    },
  };
  return {
    getEngineSettings: vi.fn(),
    setEngineSettings: vi.fn(),
    getPlaybackSettings: vi.fn(),
    setPlaybackSettings: vi.fn(),
    installSoloist: vi.fn(),
    saveSoloistAPIKey: vi.fn(),
    clearSoloistAPIKey: vi.fn(),
    startSoloist: vi.fn(),
    stopSoloist: vi.fn(),
    restartSoloist: vi.fn(),
    openSoloistDashboard: vi.fn(),
    openSoloistDownloads: vi.fn(),
    restart: vi.fn(),
    music,
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
    getPlaybackSettings: mocks.getPlaybackSettings,
    setPlaybackSettings: mocks.setPlaybackSettings,
    installSoloist: mocks.installSoloist,
    saveSoloistAPIKey: mocks.saveSoloistAPIKey,
    clearSoloistAPIKey: mocks.clearSoloistAPIKey,
    startSoloist: mocks.startSoloist,
    stopSoloist: mocks.stopSoloist,
    restartSoloist: mocks.restartSoloist,
    openSoloistDashboard: mocks.openSoloistDashboard,
    openSoloistDownloads: mocks.openSoloistDownloads,
    restart: mocks.restart,
  },
  toApiError: (error: unknown) => (error instanceof Error ? error : new Error(String(error))),
}));

vi.mock("../../lib/music.svelte", () => ({ music: mocks.music }));

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

function soloistState(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    installed: true,
    binaryPath: "/tmp/soloist",
    version: "1.3.10.12",
    builtAt: "2026-10-23T00:00:00Z",
    expiresAt: "2027-01-21T00:00:00Z",
    expired: false,
    daysLeft: 76,
    running: true,
    loggedIn: true,
    needsPairing: false,
    active: true,
    deviceName: "Cadence",
    accounts: ["listener@example.com"],
    installing: false,
    installStage: "",
    downloaded: 0,
    total: 0,
    apiKeySet: true,
    error: "",
    errorCode: "",
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
    mocks.getPlaybackSettings.mockResolvedValue({ backend: "librespot" });
    mocks.setPlaybackSettings.mockImplementation((value: { backend: string }) =>
      Promise.resolve(value),
    );
    mocks.installSoloist.mockResolvedValue({});
    mocks.saveSoloistAPIKey.mockResolvedValue({});
    mocks.clearSoloistAPIKey.mockResolvedValue({});
    mocks.startSoloist.mockResolvedValue({});
    mocks.stopSoloist.mockResolvedValue({});
    mocks.restartSoloist.mockResolvedValue({});
    mocks.openSoloistDashboard.mockResolvedValue(undefined);
    mocks.openSoloistDownloads.mockResolvedValue(undefined);
    mocks.restart.mockResolvedValue({});

    mocks.music.audio.playbackBackend = "librespot";
    mocks.music.soloist = null;

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
    mocks.skins.errorKey = null;
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

  it("shows the running backend and flags a saved switch as restart-pending", async () => {
    await renderSettings();

    const select = screen.getByLabelText("Backend") as HTMLSelectElement;
    expect(select.value).toBe("librespot");
    expect(screen.queryByText(/Restart Cadence to switch the audio backend/)).toBeNull();

    await fireEvent.change(select, { target: { value: "soloist" } });
    await fireEvent.click(screen.getByRole("button", { name: "Apply backend" }));

    await waitFor(() =>
      expect(mocks.setPlaybackSettings).toHaveBeenCalledWith({ backend: "soloist" }),
    );
    expect(screen.getByText(/Restart Cadence to switch the audio backend/)).not.toBeNull();
    expect(screen.getByText("Audio backend saved. Restart Cadence to apply it.")).not.toBeNull();
  });

  it("falls back to soloist when the playback settings cannot be loaded", async () => {
    mocks.getPlaybackSettings.mockRejectedValueOnce(new Error("playback unavailable"));

    await renderSettings();

    const select = screen.getByLabelText("Backend") as HTMLSelectElement;
    expect(select.value).toBe("soloist");
  });

  it("shows the Soloist install state with an install action", async () => {
    await renderSettings();

    expect(screen.getByText("Not installed")).not.toBeNull();
    expect(screen.getByRole("button", { name: "Install Soloist" })).not.toBeNull();
    expect(screen.getByText(/never bundled/)).not.toBeNull();
  });

  it("renders the pairing hint while Soloist waits for approval", async () => {
    mocks.music.soloist = soloistState({ needsPairing: true, loggedIn: false });

    await renderSettings();

    expect(screen.getByText("Pairing required")).not.toBeNull();
    expect(screen.getByText(/choose Cadence as the device/)).not.toBeNull();
  });

  it("renders a localized headline for a coded Soloist error", async () => {
    mocks.music.soloist = soloistState({
      needsPairing: true,
      loggedIn: false,
      errorCode: "soloist_pairing_required",
      error: "Spotify Soloist needs to be paired with a Spotify account",
    });

    await renderSettings();

    const alert = screen.getByRole("alert");
    expect(alert.textContent).toContain("Spotify pairing required");
    expect(alert.textContent).toContain(
      "Spotify Soloist needs to be paired with a Spotify account",
    );
  });

  it("warns when the Soloist build is close to expiry", async () => {
    mocks.music.soloist = soloistState({ daysLeft: 10 });

    await renderSettings();

    expect(screen.getByText(/Update within 10 days/)).not.toBeNull();
  });

  it("disables go-librespot-only controls in Soloist mode", async () => {
    mocks.music.audio.playbackBackend = "soloist";
    mocks.music.soloist = soloistState();

    await renderSettings();

    expect((screen.getByLabelText("Bitrate") as HTMLSelectElement).disabled).toBe(true);
    expect((screen.getByLabelText("Crossfade") as HTMLInputElement).disabled).toBe(true);
    expect((screen.getByLabelText("Disable normalisation") as HTMLInputElement).disabled).toBe(
      true,
    );
    expect((screen.getByLabelText("Enable disk cache") as HTMLInputElement).disabled).toBe(true);
    expect((screen.getByLabelText("Size limit") as HTMLSelectElement).disabled).toBe(true);
    expect((screen.getByLabelText("FLAC") as HTMLInputElement).disabled).toBe(true);
    expect((screen.getByLabelText("MPRIS") as HTMLInputElement).disabled).toBe(true);
    expect(
      (screen.getByLabelText("Advertise as a speaker group") as HTMLInputElement).disabled,
    ).toBe(true);
    expect((screen.getByLabelText("Image size") as HTMLSelectElement).disabled).toBe(false);
    expect(screen.getByText(/These options apply to go-librespot only/)).not.toBeNull();
  });

  it("hides the Soloist process controls in librespot mode", async () => {
    mocks.music.audio.playbackBackend = "librespot";
    mocks.music.soloist = soloistState({ installed: true });

    await renderSettings();

    expect(screen.queryByRole("button", { name: "Start" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Stop" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Restart" })).toBeNull();
    expect(screen.getByRole("button", { name: "Update Soloist" })).not.toBeNull();
  });

  it("shows install progress and disables the install action", async () => {
    mocks.music.soloist = soloistState({
      installed: false,
      installing: true,
      installStage: "downloading",
      downloaded: 50,
      total: 100,
    });

    await renderSettings();

    expect(screen.getByText("Installing Soloist…")).not.toBeNull();
    const progress = screen.getByRole("progressbar");
    expect(progress.getAttribute("value")).toBe("50");
    expect(progress.getAttribute("max")).toBe("100");
    expect(screen.getByText(/Downloading 50%/)).not.toBeNull();

    const install = screen.getByRole("button", { name: "Install Soloist" });
    expect(install.hasAttribute("disabled")).toBe(true);
  });
});
