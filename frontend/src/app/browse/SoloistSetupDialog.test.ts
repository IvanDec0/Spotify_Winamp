import { fireEvent, render, screen, waitFor } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import SoloistSetupDialog from "./SoloistSetupDialog.svelte";

const mocks = vi.hoisted(() => {
  const music = {
    audio: { playbackBackend: "soloist" },
    soloist: null as Record<string, unknown> | null,
    get soloistMode(): boolean {
      return this.audio.playbackBackend === "soloist";
    },
  };
  return {
    openSoloistDashboard: vi.fn(),
    saveSoloistAPIKey: vi.fn(),
    startSoloist: vi.fn(),
    getState: vi.fn(),
    installSoloist: vi.fn(),
    setPlaybackSettings: vi.fn(),
    music,
  };
});

vi.mock("../../shared/api", () => ({
  audio: {
    openSoloistDashboard: mocks.openSoloistDashboard,
    saveSoloistAPIKey: mocks.saveSoloistAPIKey,
    startSoloist: mocks.startSoloist,
    getState: mocks.getState,
    installSoloist: mocks.installSoloist,
    setPlaybackSettings: mocks.setPlaybackSettings,
  },
  toApiError: (error: unknown) => (error instanceof Error ? error : new Error(String(error))),
}));

vi.mock("../../lib/music.svelte", () => ({ music: mocks.music }));

function soloist(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    installed: true,
    installing: false,
    installStage: "",
    downloaded: 0,
    total: 0,
    loggedIn: false,
    needsPairing: false,
    apiKeySet: false,
    error: "",
    errorCode: "",
    ...overrides,
  };
}

function renderDialog(
  overrides: Partial<{ onclose: () => void; ondismiss: () => void; onstarted: () => void }> = {},
) {
  const onclose = overrides.onclose ?? vi.fn();
  const ondismiss = overrides.ondismiss ?? vi.fn();
  const onstarted = overrides.onstarted ?? vi.fn();
  render(SoloistSetupDialog, {
    props: { open: true, onclose, ondismiss, onstarted },
  });
  return { onclose, ondismiss, onstarted };
}

async function openPasteStep(): Promise<void> {
  await fireEvent.click(screen.getByRole("button", { name: "Generate API key" }));
  await waitFor(() => expect(screen.getByLabelText("API key")).not.toBeNull());
}

async function openPairingStep(): Promise<void> {
  mocks.startSoloist.mockResolvedValue({ soloist: { needsPairing: true } });
  await openPasteStep();
  await fireEvent.input(screen.getByLabelText("API key"), { target: { value: "key-123" } });
  await fireEvent.click(screen.getByRole("button", { name: "Save and start" }));
  await waitFor(() =>
    expect(screen.getByRole("dialog", { name: "Approve the pairing" })).not.toBeNull(),
  );
}

describe("SoloistSetupDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.music.audio.playbackBackend = "soloist";
    mocks.music.soloist = soloist();
    mocks.openSoloistDashboard.mockResolvedValue(undefined);
    mocks.saveSoloistAPIKey.mockResolvedValue({});
    mocks.startSoloist.mockResolvedValue({ soloist: { needsPairing: false } });
    mocks.getState.mockResolvedValue({ soloist: null });
    mocks.installSoloist.mockResolvedValue({});
    mocks.setPlaybackSettings.mockResolvedValue({ backend: "web" });
  });

  it("opens the dashboard and advances to the paste step", async () => {
    renderDialog();

    expect(screen.getByRole("dialog", { name: "Set up Spotify Soloist" })).not.toBeNull();
    expect(
      screen.getByText(/Soloist needs a one-time API key from your Spotify developer account/),
    ).not.toBeNull();

    await openPasteStep();

    expect(mocks.openSoloistDashboard).toHaveBeenCalledOnce();
    expect(screen.getByRole("dialog", { name: "Paste your API key" })).not.toBeNull();
  });

  it("requires a key before saving and starting", async () => {
    renderDialog();
    await openPasteStep();

    await fireEvent.click(screen.getByRole("button", { name: "Save and start" }));

    expect(screen.getByText("Paste an API key to continue.")).not.toBeNull();
    expect(mocks.saveSoloistAPIKey).not.toHaveBeenCalled();
    expect(mocks.startSoloist).not.toHaveBeenCalled();
  });

  it("saves the trimmed key, starts Soloist and closes on success", async () => {
    const onclose = vi.fn();
    const onstarted = vi.fn();
    renderDialog({ onclose, onstarted });
    await openPasteStep();

    await fireEvent.input(screen.getByLabelText("API key"), {
      target: { value: "  key-123  " },
    });
    await fireEvent.click(screen.getByRole("button", { name: "Save and start" }));

    await waitFor(() => expect(mocks.saveSoloistAPIKey).toHaveBeenCalledWith("key-123"));
    expect(mocks.startSoloist).toHaveBeenCalledOnce();
    expect(onstarted).toHaveBeenCalledOnce();
    expect(onclose).toHaveBeenCalledOnce();
  });

  it("shows the pairing step when Spotify waits for the device", async () => {
    mocks.startSoloist.mockResolvedValue({ soloist: { needsPairing: true } });
    const onclose = vi.fn();
    const onstarted = vi.fn();
    renderDialog({ onclose, onstarted });
    await openPasteStep();

    await fireEvent.input(screen.getByLabelText("API key"), { target: { value: "key-123" } });
    await fireEvent.click(screen.getByRole("button", { name: "Save and start" }));

    await waitFor(() =>
      expect(screen.getByRole("dialog", { name: "Approve the pairing" })).not.toBeNull(),
    );
    expect(onstarted).not.toHaveBeenCalled();
    expect(onclose).not.toHaveBeenCalled();

    await fireEvent.click(screen.getByRole("button", { name: "Done" }));

    expect(onstarted).toHaveBeenCalledOnce();
    expect(onclose).toHaveBeenCalledOnce();
  });

  it("finishes automatically when Spotify signs Soloist in", async () => {
    const onclose = vi.fn();
    const onstarted = vi.fn();
    renderDialog({ onclose, onstarted });
    await openPairingStep();

    (mocks.music.soloist as Record<string, unknown>).loggedIn = true;

    await waitFor(() => expect(onstarted).toHaveBeenCalledOnce());
    expect(onclose).toHaveBeenCalledOnce();
  });

  it("shows the localized rejection and stays on the paste step", async () => {
    mocks.startSoloist.mockRejectedValue(new Error("invalid_client"));
    mocks.getState.mockResolvedValue({
      soloist: { errorCode: "soloist_api_key_rejected", error: "invalid_client" },
    });
    renderDialog();
    await openPasteStep();

    await fireEvent.input(screen.getByLabelText("API key"), { target: { value: "bad-key" } });
    await fireEvent.click(screen.getByRole("button", { name: "Save and start" }));

    await waitFor(() =>
      expect(screen.getByText("Spotify rejected the Soloist API key")).not.toBeNull(),
    );
    expect(screen.getByText("invalid_client")).not.toBeNull();
    expect(screen.getByLabelText("API key")).not.toBeNull();
  });

  it("dismisses from Later", async () => {
    const ondismiss = vi.fn();
    renderDialog({ ondismiss });

    await fireEvent.click(screen.getByRole("button", { name: "Later" }));

    expect(ondismiss).toHaveBeenCalledOnce();
  });

  it("shows install progress while the build is downloading", () => {
    mocks.music.soloist = soloist({
      installed: false,
      installing: true,
      installStage: "downloading",
      downloaded: 25,
      total: 100,
    });
    renderDialog();

    expect(screen.getByText("Preparing Spotify Soloist")).not.toBeNull();
    expect(screen.getByRole("progressbar", { name: "Installing Soloist…" })).not.toBeNull();
    expect(screen.getByText(/Downloading 25%/)).not.toBeNull();
    expect(screen.queryByRole("button", { name: "Generate API key" })).toBeNull();
  });

  it("offers a retry when the install failed", async () => {
    mocks.music.soloist = soloist({
      installed: false,
      error: "network unreachable",
      errorCode: "soloist_download_failed",
    });
    renderDialog();

    expect(screen.getByText("The Spotify Soloist download failed")).not.toBeNull();

    await fireEvent.click(screen.getByRole("button", { name: "Retry download" }));

    await waitFor(() => expect(mocks.installSoloist).toHaveBeenCalledOnce());
  });

  it("persists the web backend and explains the web player", async () => {
    const onclose = vi.fn();
    renderDialog({ onclose });

    await fireEvent.click(screen.getByRole("button", { name: "Use the web player instead" }));

    await waitFor(() =>
      expect(mocks.setPlaybackSettings).toHaveBeenCalledWith({
        backend: "web",
        keepWebAvailable: false,
      }),
    );
    expect(screen.getByRole("dialog", { name: "Use the web player" })).not.toBeNull();
    expect(screen.getByText(/official Web Playback SDK/)).not.toBeNull();
    expect(screen.getByText(/Settings → Web player/)).not.toBeNull();

    await fireEvent.click(screen.getAllByRole("button", { name: "Close" })[1]);

    expect(onclose).toHaveBeenCalledOnce();
  });
});
