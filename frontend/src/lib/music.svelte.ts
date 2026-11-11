import { audio as audioApi, onAudioState, toApiError } from "../shared/api";
import type { AudioState, SoloistState } from "../shared/api/types";
import type { MessageKey, MessageParams } from "../shared/i18n";

export interface MusicNotice {
  key: MessageKey;
  params?: MessageParams;
}

/** Soloist state with every nullable generated field narrowed for the UI. */
export type SoloistSnapshot = Omit<SoloistState, "accounts"> & { accounts: string[] };

/** Audio state after normalization, with a fully populated Soloist snapshot. */
export type MusicAudioState = Omit<AudioState, "soloist"> & {
  soloist: SoloistSnapshot | null;
};

function normalizeSoloist(state: SoloistState | null | undefined): SoloistSnapshot | null {
  if (!state) return null;
  return {
    installed: state.installed ?? false,
    binaryPath: state.binaryPath ?? "",
    version: state.version ?? "",
    builtAt: state.builtAt ?? "",
    expiresAt: state.expiresAt ?? "",
    expired: state.expired ?? false,
    daysLeft: state.daysLeft ?? 0,
    running: state.running ?? false,
    loggedIn: state.loggedIn ?? false,
    needsPairing: state.needsPairing ?? false,
    active: state.active ?? false,
    deviceName: state.deviceName ?? "",
    accounts: state.accounts ?? [],
    installing: state.installing ?? false,
    installStage: state.installStage ?? "",
    downloaded: state.downloaded ?? 0,
    total: state.total ?? 0,
    apiKeySet: state.apiKeySet ?? false,
    error: state.error ?? "",
    errorCode: state.errorCode ?? "",
  };
}

function normalizeAudio(state: AudioState): MusicAudioState {
  return {
    available: state.available ?? false,
    running: state.running ?? false,
    authenticated: state.authenticated ?? false,
    engine: state.engine ?? "go-librespot",
    engineVersion: state.engineVersion ?? "",
    deviceName: state.deviceName ?? "Cadence",
    deviceType: state.deviceType ?? "",
    pairing: state.pairing ?? null,
    error: state.error ?? "",
    errorCode: state.errorCode ?? "",
    binaryPath: state.binaryPath ?? "",
    configDirectory: state.configDirectory ?? "",
    audioBackend: state.audioBackend ?? "",
    playbackBackend: state.playbackBackend ?? "web",
    soloist: normalizeSoloist(state.soloist),
  };
}

const initialState: MusicAudioState = {
  available: false,
  running: false,
  authenticated: false,
  engine: "go-librespot",
  engineVersion: "",
  deviceName: "Cadence",
  deviceType: "",
  pairing: null,
  error: "",
  errorCode: "",
  binaryPath: "",
  configDirectory: "",
  audioBackend: "",
  playbackBackend: "web",
  soloist: null,
};

// The audio engine pushes state changes; polling is only needed while the
// user still has to pair the device.
const pairingPollInterval = 5000;

/**
 * Engine and pairing state shared by Browse's `EngineGate`. Library, search
 * and context data live in the Browse store; this store only tracks whether
 * the local engine is available, running and authenticated.
 */
export class MusicStore {
  audio = $state<MusicAudioState>(initialState);
  loading = $state(true);
  busy = $state(false);
  error = $state<string | null>(null);
  notice = $state<MusicNotice | null>(null);

  #unsubscribe: (() => void) | null = null;
  #started = false;
  #pollTimer: ReturnType<typeof setInterval> | null = null;

  get authenticated(): boolean {
    return this.audio.authenticated;
  }

  get pairing() {
    return this.audio.pairing;
  }

  get soloistMode(): boolean {
    return this.audio.playbackBackend === "soloist";
  }

  get soloist(): SoloistSnapshot | null {
    return this.audio.soloist;
  }

  async start(): Promise<void> {
    if (this.#started) return;
    this.#started = true;
    this.#unsubscribe = onAudioState((state) => {
      this.audio = normalizeAudio(state);
      this.#syncPoll();
    });
    await this.refresh();
  }

  stop(): void {
    this.#unsubscribe?.();
    this.#unsubscribe = null;
    this.#started = false;
    this.#stopPoll();
  }

  async refresh(): Promise<void> {
    try {
      this.audio = normalizeAudio(await audioApi.getState());
      this.loading = false;
      this.#syncPoll();
    } catch (error) {
      this.error = toApiError(error).message;
      this.loading = false;
      this.#syncPoll();
    }
  }

  #syncPoll(): void {
    if (!this.#started) return;
    if (this.audio.authenticated) {
      this.#stopPoll();
      return;
    }
    if (this.#pollTimer) return;
    this.#pollTimer = setInterval(() => {
      void this.refresh();
    }, pairingPollInterval);
  }

  #stopPoll(): void {
    if (!this.#pollTimer) return;
    clearInterval(this.#pollTimer);
    this.#pollTimer = null;
  }

  async openPairing(): Promise<void> {
    try {
      await audioApi.openPairing();
      this.error = null;
    } catch (error) {
      this.error = toApiError(error).message;
    }
  }

  async resetSession(): Promise<void> {
    this.busy = true;
    try {
      this.audio = normalizeAudio(await audioApi.resetSession());
      this.#syncPoll();
      this.error = null;
      this.notice = { key: "music.pairingReset" };
    } catch (error) {
      this.error = toApiError(error).message;
    } finally {
      this.busy = false;
    }
  }

  async restart(): Promise<void> {
    this.busy = true;
    try {
      this.audio = normalizeAudio(await audioApi.restart());
      this.#syncPoll();
      this.error = null;
    } catch (error) {
      this.error = toApiError(error).message;
    } finally {
      this.busy = false;
    }
  }

  dismissError(): void {
    this.error = null;
  }

  dismissNotice(): void {
    this.notice = null;
  }
}

export const music = new MusicStore();
