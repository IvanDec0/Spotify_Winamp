import { audio, toApiError } from "../../shared/api";
import type { EngineSettings, PlaybackSettings, WebPlayerStatus } from "../../shared/api/types";
import type { MessageKey } from "../../shared/i18n";

/**
 * Engine settings form state. Saving only persists the values; the engine is
 * restarted exclusively from the explicit confirmation flow in SettingsView.
 */
export class BrowseSettingsStore {
  settings = $state<EngineSettings | null>(null);
  playback = $state<PlaybackSettings | null>(null);
  webStatus = $state<WebPlayerStatus | null>(null);
  loading = $state(false);
  saving = $state(false);
  savingBackend = $state(false);
  soloistBusy = $state(false);
  webBusy = $state(false);
  developerBusy = $state(false);
  restarting = $state(false);
  restartRequired = $state(false);
  error = $state<string | null>(null);
  notice = $state<MessageKey | null>(null);

  #loaded = false;

  async load(force = false): Promise<void> {
    if (this.#loaded && !force) return;
    this.loading = true;
    this.error = null;
    try {
      // Engine settings gate the form; a playback or web-player failure must
      // not hide the engine, appearance and skin sections.
      const [engineResult, playbackResult, webResult] = await Promise.allSettled([
        audio.getEngineSettings(),
        audio.getPlaybackSettings(),
        audio.getWebPlayerStatus(),
      ]);
      if (engineResult.status === "fulfilled") {
        this.settings = engineResult.value;
        this.#loaded = true;
      } else {
        this.error = toApiError(engineResult.reason).message;
      }
      if (playbackResult.status === "fulfilled") {
        this.playback = playbackResult.value;
      } else if (!this.error) {
        this.error = toApiError(playbackResult.reason).message;
      }
      if (webResult.status === "fulfilled") {
        this.webStatus = webResult.value;
      }
    } finally {
      this.loading = false;
    }
  }

  update<K extends keyof EngineSettings>(key: K, value: EngineSettings[K]): void {
    if (!this.settings) return;
    this.settings = { ...this.settings, [key]: value } as EngineSettings;
    this.notice = null;
    this.restartRequired = false;
  }

  async save(): Promise<void> {
    if (!this.settings || this.saving) return;
    this.saving = true;
    this.error = null;
    this.notice = null;
    try {
      this.settings = await audio.setEngineSettings(this.settings);
      this.restartRequired = true;
      this.notice = "browse.settingsSaved";
    } catch (error) {
      this.error = toApiError(error).message;
    } finally {
      this.saving = false;
    }
  }

  async restart(): Promise<void> {
    if (this.restarting || this.saving) return;
    this.restarting = true;
    this.error = null;
    this.notice = null;
    try {
      await audio.restart();
      this.restartRequired = false;
      this.notice = "browse.settingsRestarted";
    } catch (error) {
      this.error = toApiError(error).message;
    } finally {
      this.restarting = false;
    }
  }

  updateBackend(backend: string): void {
    if (!this.playback) return;
    this.playback = { ...this.playback, backend };
    this.notice = null;
    this.error = null;
  }

  updateKeepWebAvailable(value: boolean): void {
    if (!this.playback) return;
    this.playback = { ...this.playback, keepWebAvailable: value };
    this.notice = null;
    this.error = null;
  }

  async saveBackend(): Promise<void> {
    if (!this.playback || this.savingBackend) return;
    this.savingBackend = true;
    this.error = null;
    this.notice = null;
    try {
      this.playback = await audio.setPlaybackSettings(this.playback);
      this.notice = "browse.settingsBackendSaved";
    } catch (error) {
      this.error = toApiError(error).message;
    } finally {
      this.savingBackend = false;
    }
  }

  async installSoloist(): Promise<void> {
    await this.#soloistAction(() => audio.installSoloist(), "browse.soloistInstallDone");
  }

  async saveSoloistKey(key: string): Promise<void> {
    await this.#soloistAction(() => audio.saveSoloistAPIKey(key), "browse.soloistKeySaved");
  }

  async clearSoloistKey(): Promise<void> {
    await this.#soloistAction(() => audio.clearSoloistAPIKey(), "browse.soloistKeyCleared");
  }

  async startSoloist(): Promise<void> {
    await this.#soloistAction(() => audio.startSoloist(), "browse.soloistStartDone");
  }

  async stopSoloist(): Promise<void> {
    await this.#soloistAction(() => audio.stopSoloist(), "browse.soloistStopDone");
  }

  async restartSoloist(): Promise<void> {
    await this.#soloistAction(() => audio.restartSoloist(), "browse.soloistRestartDone");
  }

  async openSoloistDashboard(): Promise<void> {
    await this.#soloistAction(() => audio.openSoloistDashboard(), "browse.soloistLinkOpened");
  }

  async openSoloistDownloads(): Promise<void> {
    await this.#soloistAction(() => audio.openSoloistDownloads(), "browse.soloistLinkOpened");
  }

  /** Refreshes the web player status without surfacing transient failures. */
  async refreshWebStatus(): Promise<void> {
    try {
      this.webStatus = await audio.getWebPlayerStatus();
    } catch {
      // The status endpoint is best effort; the last known state remains.
    }
  }

  async connectWebPlayer(): Promise<void> {
    await this.#webAction(() => audio.connectWebPlayer(), "browse.webConnectDone");
  }

  async disconnectWebPlayer(): Promise<void> {
    await this.#webAction(() => audio.disconnectWebPlayer(), "browse.webDisconnectDone");
  }

  async installWebRuntime(): Promise<void> {
    await this.#webAction(() => audio.installWebPlayerRuntime(), "browse.webInstallDone");
  }

  async startWebPlayer(): Promise<void> {
    await this.#webAction(() => audio.startWebPlayer(), "browse.webStartDone");
  }

  async stopWebPlayer(): Promise<void> {
    await this.#webAction(() => audio.stopWebPlayer(), "browse.webStopDone");
  }

  /**
   * Creates or finds the Cadence Spotify Developer app and stores its Client
   * ID. The action is headless: no browser window is opened.
   */
  async configureWebPlayerApp(): Promise<void> {
    if (this.developerBusy) return;
    this.developerBusy = true;
    this.error = null;
    this.notice = null;
    try {
      this.webStatus = await audio.configureWebPlayerApp();
      this.notice = "browse.webConfigureDone";
    } catch (error) {
      this.error = toApiError(error).message;
      await this.refreshWebStatus();
    } finally {
      this.developerBusy = false;
    }
  }

  /**
   * Generates or rotates the Soloist API key through the Spotify Developer
   * dashboard and stores it in the Soloist backend. The key never reaches the
   * frontend.
   */
  async generateSoloistAPIKey(rotate: boolean): Promise<void> {
    if (this.developerBusy) return;
    this.developerBusy = true;
    this.error = null;
    this.notice = null;
    try {
      await audio.generateSoloistAPIKey(rotate);
      this.notice = "browse.soloistKeyGenerated";
    } catch (error) {
      this.error = toApiError(error).message;
    } finally {
      this.developerBusy = false;
    }
  }

  /**
   * Runs one web player action under the busy guard. The action returns the
   * full status, so account, runtime and running state update immediately; a
   * failure refreshes the status to surface the stored error code.
   */
  async #webAction(run: () => Promise<WebPlayerStatus>, notice: MessageKey): Promise<boolean> {
    if (this.webBusy) return false;
    this.webBusy = true;
    this.error = null;
    this.notice = null;
    try {
      this.webStatus = await run();
      this.notice = notice;
      return true;
    } catch (error) {
      this.error = toApiError(error).message;
      await this.refreshWebStatus();
      return false;
    } finally {
      this.webBusy = false;
    }
  }

  /**
   * Runs one Soloist action under the busy guard. Live Soloist state arrives
   * through the `audio:state` event, so the action result is not stored here.
   */
  async #soloistAction(run: () => Promise<unknown>, notice: MessageKey): Promise<boolean> {
    if (this.soloistBusy) return false;
    this.soloistBusy = true;
    this.error = null;
    this.notice = null;
    try {
      await run();
      this.notice = notice;
      return true;
    } catch (error) {
      this.error = toApiError(error).message;
      return false;
    } finally {
      this.soloistBusy = false;
    }
  }

  dismissError(): void {
    this.error = null;
  }

  dismissNotice(): void {
    this.notice = null;
  }

  reset(): void {
    this.#loaded = false;
    this.settings = null;
    this.playback = null;
    this.webStatus = null;
    this.loading = false;
    this.saving = false;
    this.savingBackend = false;
    this.soloistBusy = false;
    this.webBusy = false;
    this.developerBusy = false;
    this.restarting = false;
    this.restartRequired = false;
    this.error = null;
    this.notice = null;
  }
}

export const browseSettings = new BrowseSettingsStore();
