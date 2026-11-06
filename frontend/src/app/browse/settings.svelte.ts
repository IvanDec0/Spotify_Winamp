import { audio, toApiError } from "../../shared/api";
import type { EngineSettings, PlaybackSettings } from "../../shared/api/types";
import type { MessageKey } from "../../shared/i18n";

/**
 * Engine settings form state. Saving only persists the values; the engine is
 * restarted exclusively from the explicit confirmation flow in SettingsView.
 */
export class BrowseSettingsStore {
  settings = $state<EngineSettings | null>(null);
  playback = $state<PlaybackSettings | null>(null);
  loading = $state(false);
  saving = $state(false);
  savingBackend = $state(false);
  soloistBusy = $state(false);
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
      // Engine settings gate the form; a playback-settings failure must not
      // hide the engine, appearance and skin sections.
      const [engineResult, playbackResult] = await Promise.allSettled([
        audio.getEngineSettings(),
        audio.getPlaybackSettings(),
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
    this.loading = false;
    this.saving = false;
    this.savingBackend = false;
    this.soloistBusy = false;
    this.restarting = false;
    this.restartRequired = false;
    this.error = null;
    this.notice = null;
  }
}

export const browseSettings = new BrowseSettingsStore();
