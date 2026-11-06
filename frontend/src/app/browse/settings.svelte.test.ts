import { beforeEach, describe, expect, it, vi } from "vitest";
import type { EngineSettings, WebPlayerStatus } from "../../shared/api/types";
import { BrowseSettingsStore } from "./settings.svelte";

const mocks = vi.hoisted(() => ({
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
  getWebPlayerStatus: vi.fn(),
  connectWebPlayer: vi.fn(),
  disconnectWebPlayer: vi.fn(),
  installWebPlayerRuntime: vi.fn(),
  startWebPlayer: vi.fn(),
  stopWebPlayer: vi.fn(),
  configureWebPlayerApp: vi.fn(),
  generateSoloistAPIKey: vi.fn(),
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

function webStatus(overrides: Partial<WebPlayerStatus> = {}): WebPlayerStatus {
  return {
    available: true,
    loggedIn: false,
    running: false,
    runtimeInstalled: false,
    runtimeVersion: "",
    deviceId: "",
    account: "",
    lastError: "",
    clientId: "7fe450922d8b4d31ac8bda4a1767d903",
    restartRequired: false,
    ...overrides,
  };
}

describe("BrowseSettingsStore", () => {
  let store: BrowseSettingsStore;

  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getPlaybackSettings.mockResolvedValue({ backend: "librespot" });
    mocks.getWebPlayerStatus.mockResolvedValue(webStatus());
    store = new BrowseSettingsStore();
  });

  it("loads engine and playback settings in parallel", async () => {
    mocks.getEngineSettings.mockResolvedValue(settings({ bitrate: 160 }));
    mocks.getPlaybackSettings.mockResolvedValue({ backend: "soloist" });
    mocks.getWebPlayerStatus.mockResolvedValue(webStatus({ loggedIn: true, account: "Listener" }));

    await store.load();
    expect(store.settings?.bitrate).toBe(160);
    expect(store.playback?.backend).toBe("soloist");
    expect(store.webStatus?.loggedIn).toBe(true);
    expect(store.webStatus?.account).toBe("Listener");
    expect(store.loading).toBe(false);
    expect(store.error).toBeNull();

    await store.load();
    expect(mocks.getEngineSettings).toHaveBeenCalledTimes(1);
    expect(mocks.getPlaybackSettings).toHaveBeenCalledTimes(1);
    expect(mocks.getWebPlayerStatus).toHaveBeenCalledTimes(1);
  });

  it("keeps the engine form when the web status fails to load", async () => {
    mocks.getEngineSettings.mockResolvedValue(settings());
    mocks.getWebPlayerStatus.mockRejectedValue(new Error("web player unavailable"));

    await store.load();

    expect(store.settings?.bitrate).toBe(320);
    expect(store.playback?.backend).toBe("librespot");
    expect(store.webStatus).toBeNull();
    expect(store.error).toBeNull();
  });

  it("refreshes the web status silently", async () => {
    mocks.getWebPlayerStatus.mockResolvedValueOnce(webStatus({ running: true }));
    await store.refreshWebStatus();
    expect(store.webStatus?.running).toBe(true);

    mocks.getWebPlayerStatus.mockRejectedValueOnce(new Error("network down"));
    await store.refreshWebStatus();
    expect(store.webStatus?.running).toBe(true);
    expect(store.error).toBeNull();
  });

  it("keeps the engine form when playback settings fail to load", async () => {
    mocks.getEngineSettings.mockResolvedValue(settings());
    mocks.getPlaybackSettings.mockRejectedValue(new Error("playback unavailable"));

    await store.load();

    expect(store.settings?.bitrate).toBe(320);
    expect(store.playback).toBeNull();
    expect(store.error).toBe("playback unavailable");
    expect(store.loading).toBe(false);
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

  it("updates and saves the audio backend", async () => {
    mocks.getEngineSettings.mockResolvedValue(settings());
    mocks.setPlaybackSettings.mockResolvedValue({ backend: "soloist" });
    await store.load();

    store.updateBackend("soloist");
    expect(store.playback?.backend).toBe("soloist");

    await store.saveBackend();

    expect(mocks.setPlaybackSettings).toHaveBeenCalledWith({ backend: "soloist" });
    expect(store.playback?.backend).toBe("soloist");
    expect(store.savingBackend).toBe(false);
    expect(store.error).toBeNull();
    expect(store.notice).toBe("browse.settingsBackendSaved");
  });

  it("keeps the selection and reports backend save errors", async () => {
    mocks.getEngineSettings.mockResolvedValue(settings());
    mocks.setPlaybackSettings.mockRejectedValue(new Error("settings store unavailable"));
    await store.load();
    store.updateBackend("soloist");

    await store.saveBackend();

    expect(store.playback?.backend).toBe("soloist");
    expect(store.savingBackend).toBe(false);
    expect(store.error).toBe("settings store unavailable");
    expect(store.notice).toBeNull();
  });

  it("runs every Soloist action and sets a notice", async () => {
    mocks.installSoloist.mockResolvedValue({});
    mocks.saveSoloistAPIKey.mockResolvedValue({});
    mocks.clearSoloistAPIKey.mockResolvedValue({});
    mocks.startSoloist.mockResolvedValue({});
    mocks.stopSoloist.mockResolvedValue({});
    mocks.restartSoloist.mockResolvedValue({});
    mocks.openSoloistDashboard.mockResolvedValue(undefined);
    mocks.openSoloistDownloads.mockResolvedValue(undefined);

    await store.installSoloist();
    expect(store.notice).toBe("browse.soloistInstallDone");

    await store.saveSoloistKey("secret");
    expect(mocks.saveSoloistAPIKey).toHaveBeenCalledWith("secret");
    expect(store.notice).toBe("browse.soloistKeySaved");

    await store.clearSoloistKey();
    expect(store.notice).toBe("browse.soloistKeyCleared");

    await store.startSoloist();
    expect(store.notice).toBe("browse.soloistStartDone");

    await store.stopSoloist();
    expect(store.notice).toBe("browse.soloistStopDone");

    await store.restartSoloist();
    expect(store.notice).toBe("browse.soloistRestartDone");

    await store.openSoloistDashboard();
    expect(store.notice).toBe("browse.soloistLinkOpened");

    await store.openSoloistDownloads();
    expect(store.notice).toBe("browse.soloistLinkOpened");

    expect(mocks.installSoloist).toHaveBeenCalledOnce();
    expect(mocks.clearSoloistAPIKey).toHaveBeenCalledOnce();
    expect(mocks.startSoloist).toHaveBeenCalledOnce();
    expect(mocks.stopSoloist).toHaveBeenCalledOnce();
    expect(mocks.restartSoloist).toHaveBeenCalledOnce();
    expect(mocks.openSoloistDashboard).toHaveBeenCalledOnce();
    expect(mocks.openSoloistDownloads).toHaveBeenCalledOnce();
    expect(store.soloistBusy).toBe(false);
  });

  it("reports Soloist action errors and clears the busy flag", async () => {
    mocks.startSoloist.mockRejectedValue(new Error("Soloist is not installed"));

    await store.startSoloist();

    expect(store.error).toBe("Soloist is not installed");
    expect(store.notice).toBeNull();
    expect(store.soloistBusy).toBe(false);
  });

  it("runs every web player action and stores the returned status", async () => {
    const connected = webStatus({ loggedIn: true, account: "Listener" });
    mocks.connectWebPlayer.mockResolvedValue(connected);
    mocks.disconnectWebPlayer.mockResolvedValue(webStatus());
    mocks.installWebPlayerRuntime.mockResolvedValue(
      webStatus({ runtimeInstalled: true, runtimeVersion: "44.9.3" }),
    );
    mocks.startWebPlayer.mockResolvedValue(connected);
    mocks.stopWebPlayer.mockResolvedValue(connected);

    await store.connectWebPlayer();
    expect(store.webStatus?.loggedIn).toBe(true);
    expect(store.notice).toBe("browse.webConnectDone");

    await store.disconnectWebPlayer();
    expect(store.webStatus?.loggedIn).toBe(false);
    expect(store.notice).toBe("browse.webDisconnectDone");

    await store.installWebRuntime();
    expect(store.webStatus?.runtimeVersion).toBe("44.9.3");
    expect(store.notice).toBe("browse.webInstallDone");

    await store.startWebPlayer();
    expect(store.notice).toBe("browse.webStartDone");

    await store.stopWebPlayer();
    expect(store.notice).toBe("browse.webStopDone");

    expect(mocks.connectWebPlayer).toHaveBeenCalledOnce();
    expect(mocks.disconnectWebPlayer).toHaveBeenCalledOnce();
    expect(mocks.installWebPlayerRuntime).toHaveBeenCalledOnce();
    expect(mocks.startWebPlayer).toHaveBeenCalledOnce();
    expect(mocks.stopWebPlayer).toHaveBeenCalledOnce();
    expect(store.webBusy).toBe(false);
  });

  it("reports web action errors and refreshes the stored web status", async () => {
    mocks.connectWebPlayer.mockRejectedValue(new Error("login timed out"));
    mocks.getWebPlayerStatus.mockResolvedValue(webStatus({ lastError: "webplayer_login_timeout" }));

    await store.connectWebPlayer();

    expect(store.error).toBe("login timed out");
    expect(store.notice).toBeNull();
    expect(store.webStatus?.lastError).toBe("webplayer_login_timeout");
    expect(store.webBusy).toBe(false);
  });

  it("configures the web app automatically and stores the new client id", async () => {
    const newID = "0123456789abcdef0123456789abcdef";
    mocks.configureWebPlayerApp.mockResolvedValue(
      webStatus({ clientId: newID, restartRequired: true }),
    );

    await store.configureWebPlayerApp();

    expect(mocks.configureWebPlayerApp).toHaveBeenCalledOnce();
    expect(store.webStatus?.clientId).toBe(newID);
    expect(store.webStatus?.restartRequired).toBe(true);
    expect(store.notice).toBe("browse.webConfigureDone");
    expect(store.developerBusy).toBe(false);
  });

  it("reports developer failures and refreshes the web status", async () => {
    mocks.configureWebPlayerApp.mockRejectedValue(
      new Error("sign in to the Spotify Developer dashboard first"),
    );
    mocks.getWebPlayerStatus.mockResolvedValue(webStatus({ restartRequired: true }));

    await store.configureWebPlayerApp();

    expect(store.error).toBe("sign in to the Spotify Developer dashboard first");
    expect(store.notice).toBeNull();
    expect(store.webStatus?.restartRequired).toBe(true);
    expect(store.developerBusy).toBe(false);
  });

  it("generates and rotates the Soloist API key automatically", async () => {
    mocks.generateSoloistAPIKey.mockResolvedValue({});

    await store.generateSoloistAPIKey(false);
    expect(mocks.generateSoloistAPIKey).toHaveBeenCalledWith(false);
    expect(store.notice).toBe("browse.soloistKeyGenerated");

    await store.generateSoloistAPIKey(true);
    expect(mocks.generateSoloistAPIKey).toHaveBeenLastCalledWith(true);
    expect(store.notice).toBe("browse.soloistKeyGenerated");
    expect(store.developerBusy).toBe(false);
  });

  it("reports Soloist key automation failures", async () => {
    mocks.generateSoloistAPIKey.mockRejectedValue(new Error("Spotify Premium is required"));

    await store.generateSoloistAPIKey(false);

    expect(store.error).toBe("Spotify Premium is required");
    expect(store.notice).toBeNull();
    expect(store.developerBusy).toBe(false);
  });

  it("resets the playback and Soloist state", async () => {
    mocks.getEngineSettings.mockResolvedValue(settings());
    mocks.getPlaybackSettings.mockResolvedValue({ backend: "soloist" });
    await store.load();

    store.reset();

    expect(store.settings).toBeNull();
    expect(store.playback).toBeNull();
    expect(store.webStatus).toBeNull();
    expect(store.savingBackend).toBe(false);
    expect(store.soloistBusy).toBe(false);
    expect(store.webBusy).toBe(false);
    expect(store.developerBusy).toBe(false);
    expect(store.error).toBeNull();
    expect(store.notice).toBeNull();
  });
});
