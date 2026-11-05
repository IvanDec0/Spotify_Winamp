import { audio, toApiError } from "../../shared/api";
import type { EngineSettings } from "../../shared/api/types";
import type { MessageKey } from "../../shared/i18n";

/**
 * Engine settings form state. Saving only persists the values; the engine is
 * restarted exclusively from the explicit confirmation flow in SettingsView.
 */
export class BrowseSettingsStore {
  settings = $state<EngineSettings | null>(null);
  loading = $state(false);
  saving = $state(false);
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
      this.settings = await audio.getEngineSettings();
      this.#loaded = true;
    } catch (error) {
      this.error = toApiError(error).message;
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

  dismissError(): void {
    this.error = null;
  }

  dismissNotice(): void {
    this.notice = null;
  }

  reset(): void {
    this.#loaded = false;
    this.settings = null;
    this.loading = false;
    this.saving = false;
    this.restarting = false;
    this.restartRequired = false;
    this.error = null;
    this.notice = null;
  }
}

export const browseSettings = new BrowseSettingsStore();
