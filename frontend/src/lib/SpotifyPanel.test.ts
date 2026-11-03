import { render, screen } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AuthStatus } from "../../bindings/cadence/internal/spotify";
import { spotify } from "./spotify.svelte";
import SpotifyPanel from "./SpotifyPanel.svelte";

const onClose = vi.fn();

function authState(overrides: Partial<typeof spotify.auth> = {}): typeof spotify.auth {
  return {
    status: AuthStatus.AuthUnauthenticated,
    clientId: "client-id",
    redirectUri: "http://127.0.0.1:43821/callback",
    keyringAvailable: true,
    playerMode: "demo",
    error: "",
    ...overrides,
  };
}

describe("SpotifyPanel", () => {
  beforeEach(() => {
    onClose.mockClear();
    spotify.error = null;
    spotify.auth = authState();
  });

  it("shows reconnect copy after a revoked session", () => {
    spotify.auth = authState({
      status: AuthStatus.AuthError,
      error: "Spotify session expired. Connect again.",
    });
    render(SpotifyPanel, { onClose });

    expect(screen.getByRole("dialog", { name: "Spotify account" })).not.toBeNull();
    expect(screen.getByText("Spotify session expired. Connect again.")).not.toBeNull();
    expect(screen.getByRole("button", { name: "Reconnect Spotify" })).not.toBeNull();
  });

  it("explains an unavailable keyring and disables connecting", () => {
    spotify.auth = authState({ keyringAvailable: false });
    render(SpotifyPanel, { onClose });

    expect(screen.getByText(/The OS keyring is unavailable/)).not.toBeNull();
    const connect = screen.getByRole("button", { name: "Connect Spotify" }) as HTMLButtonElement;
    expect(connect.disabled).toBe(true);
  });

  it("shows the redirect URI and authenticated controls", () => {
    spotify.auth = authState({
      status: AuthStatus.AuthAuthenticated,
      playerMode: "spotify",
    });
    render(SpotifyPanel, { onClose });

    expect(screen.getByText("http://127.0.0.1:43821/callback")).not.toBeNull();
    expect(screen.getByText("Spotify Connect")).not.toBeNull();
    expect(screen.getByRole("button", { name: "Use demo player" })).not.toBeNull();
    expect(screen.getByRole("button", { name: "Log out" })).not.toBeNull();
  });
});
