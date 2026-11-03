import { Events } from "@wailsio/runtime";
import {
  AuthStatus,
  Service as SpotifyService,
  type AuthState,
} from "../../bindings/cadence/internal/spotify";

const AUTH_EVENT = "spotify:auth";

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  return typeof error === "string" ? error : "The Spotify operation failed.";
}

function normalize(state: AuthState): AuthState {
  return {
    status: state.status ?? AuthStatus.AuthChecking,
    clientId: state.clientId ?? "",
    redirectUri: state.redirectUri ?? "http://127.0.0.1:43821/callback",
    keyringAvailable: state.keyringAvailable ?? true,
    playerMode: state.playerMode ?? "demo",
    error: state.error ?? "",
  };
}

const initialState: AuthState = {
  status: AuthStatus.AuthChecking,
  clientId: "",
  redirectUri: "http://127.0.0.1:43821/callback",
  keyringAvailable: true,
  playerMode: "demo",
  error: "",
};

class SpotifyStore {
  auth = $state<AuthState>(initialState);
  loading = $state(true);
  error = $state<string | null>(null);

  #unsubscribe: (() => void) | null = null;
  #started = false;
  #onAuth = (event: { data: unknown }): void => {
    this.auth = normalize(event.data as AuthState);
    this.error = null;
  };

  get isAuthenticated(): boolean {
    return this.auth.status === AuthStatus.AuthAuthenticated;
  }

  get isAuthenticating(): boolean {
    return this.auth.status === AuthStatus.AuthAuthenticating;
  }

  get isSpotifyPlayer(): boolean {
    return this.auth.playerMode === "spotify";
  }

  async start(): Promise<void> {
    if (this.#started) return;
    this.#started = true;
    this.#unsubscribe = Events.On(AUTH_EVENT, this.#onAuth);
    await this.refresh();
  }

  stop(): void {
    this.#unsubscribe?.();
    this.#unsubscribe = null;
    this.#started = false;
  }

  async refresh(): Promise<void> {
    this.loading = true;
    try {
      this.auth = normalize(await SpotifyService.GetAuthState());
      this.error = null;
    } catch (error) {
      this.error = errorMessage(error);
    } finally {
      this.loading = false;
    }
  }

  async setClientID(id: string): Promise<void> {
    try {
      await SpotifyService.SetClientID(id);
      await this.refresh();
      this.error = null;
    } catch (error) {
      this.error = errorMessage(error);
    }
  }

  async login(): Promise<void> {
    try {
      await SpotifyService.Login();
      await this.refresh();
      this.error = null;
    } catch (error) {
      this.error = errorMessage(error);
    }
  }

  async logout(): Promise<void> {
    try {
      await SpotifyService.Logout();
      await this.refresh();
      this.error = null;
    } catch (error) {
      this.error = errorMessage(error);
    }
  }

  async useDemo(): Promise<void> {
    try {
      await SpotifyService.UseDemo();
      await this.refresh();
      this.error = null;
    } catch (error) {
      this.error = errorMessage(error);
    }
  }

  async useSpotify(): Promise<void> {
    try {
      await SpotifyService.UseSpotify();
      await this.refresh();
      this.error = null;
    } catch (error) {
      this.error = errorMessage(error);
    }
  }

  dismissError(): void {
    this.error = null;
  }
}

export const spotify = new SpotifyStore();
