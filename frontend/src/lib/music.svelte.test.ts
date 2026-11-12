import { beforeEach, describe, expect, it, vi } from "vitest";
import type { AudioState, SoloistState } from "../shared/api/types";
import { MusicStore } from "./music.svelte";

const mocks = vi.hoisted(() => ({
  getState: vi.fn(),
  startEngine: vi.fn(),
  onAudioState: vi.fn(),
}));

vi.mock("../shared/api", () => ({
  audio: { getState: mocks.getState, startEngine: mocks.startEngine },
  onAudioState: mocks.onAudioState,
  toApiError: (error: unknown) => (error instanceof Error ? error : new Error(String(error))),
}));

function audioState(overrides: Partial<AudioState> = {}): AudioState {
  return {
    available: true,
    running: true,
    authenticated: true,
    engine: "go-librespot",
    engineVersion: "0.1.2",
    deviceName: "Cadence",
    deviceType: "computer",
    pairing: null,
    error: "",
    errorCode: "",
    binaryPath: "/tmp/go-librespot",
    configDirectory: "/tmp/cadence",
    audioBackend: "alsa",
    playbackBackend: "librespot",
    soloist: null,
    ...overrides,
  };
}

function soloistState(overrides: Partial<SoloistState> = {}): SoloistState {
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

describe("MusicStore", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("defaults to the web backend without Soloist state", () => {
    const store = new MusicStore();

    expect(store.audio.playbackBackend).toBe("web");
    expect(store.audio.soloist).toBeNull();
    expect(store.soloist).toBeNull();
    expect(store.soloistMode).toBe(false);
  });

  it("normalizes a missing playback backend to web", async () => {
    mocks.getState.mockResolvedValue(
      audioState({ playbackBackend: null as unknown as string, soloist: null }),
    );
    const store = new MusicStore();

    await store.refresh();

    expect(store.audio.playbackBackend).toBe("web");
    expect(store.soloistMode).toBe(false);
  });

  it("normalizes nullable Soloist fields from the API", async () => {
    mocks.getState.mockResolvedValue(
      audioState({
        playbackBackend: "soloist",
        soloist: soloistState({
          accounts: null,
          installing: null as unknown as boolean,
          downloaded: null as unknown as number,
          apiKeySet: null as unknown as boolean,
          version: null as unknown as string,
        }),
      }),
    );
    const store = new MusicStore();

    await store.refresh();

    expect(store.soloistMode).toBe(true);
    expect(store.soloist?.installed).toBe(true);
    expect(store.soloist?.accounts).toEqual([]);
    expect(store.soloist?.installing).toBe(false);
    expect(store.soloist?.downloaded).toBe(0);
    expect(store.soloist?.apiKeySet).toBe(false);
    expect(store.soloist?.version).toBe("");
    expect(store.soloist?.daysLeft).toBe(76);
  });

  it("normalizes audio:state events for both backends", async () => {
    mocks.getState.mockResolvedValue(audioState());
    const store = new MusicStore();
    await store.start();

    const listener = mocks.onAudioState.mock.calls[0]?.[0] as (state: AudioState) => void;
    expect(listener).toBeTypeOf("function");

    listener(
      audioState({
        playbackBackend: "soloist",
        soloist: soloistState({ needsPairing: true, loggedIn: false }),
      }),
    );

    expect(store.soloistMode).toBe(true);
    expect(store.soloist?.needsPairing).toBe(true);
    expect(store.soloist?.accounts).toEqual(["listener@example.com"]);

    listener(audioState({ playbackBackend: "librespot", soloist: null }));

    expect(store.soloistMode).toBe(false);
    expect(store.soloist).toBeNull();
    store.stop();
  });
});

describe("MusicStore engine start", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("starts the catalog engine when Browse asks for it", async () => {
    mocks.startEngine.mockResolvedValue(audioState({ authenticated: false, running: true }));
    mocks.getState.mockResolvedValue(audioState({ authenticated: false, running: true }));
    const store = new MusicStore();

    await store.start({ ensureEngine: true });

    expect(mocks.startEngine).toHaveBeenCalledOnce();
    expect(store.audio.running).toBe(true);
    store.stop();
  });

  it("ensureEngine starts the engine and clears the busy flag", async () => {
    mocks.startEngine.mockResolvedValue(audioState({ authenticated: false, running: true }));
    mocks.getState.mockResolvedValue(audioState({ authenticated: false, running: true }));
    const store = new MusicStore();

    await store.ensureEngine();

    expect(mocks.startEngine).toHaveBeenCalledOnce();
    expect(store.busy).toBe(false);
  });
});
