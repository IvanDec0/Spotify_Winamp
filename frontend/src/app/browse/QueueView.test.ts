import { render, screen, waitFor } from "@testing-library/svelte";
import { tick } from "svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { QueueItem } from "../../shared/api/types";
import { createPlayerState, playerMock } from "../../lib/testing/player-mock.svelte";
import QueueView from "./QueueView.svelte";

vi.mock("../../lib/player.svelte", async () => {
  const { playerMock: player } = await import("../../lib/testing/player-mock.svelte");
  return { player };
});

function queueItem(index: number, title: string, overrides: Partial<QueueItem> = {}): QueueItem {
  return {
    track: {
      id: `track-${index}`,
      title,
      artists: ["Aster Vale"],
      album: "Night Cartography",
      cover: "meridian",
      uri: `spotify:track:${index}`,
      durationMs: 180_000,
    },
    index,
    isCurrent: false,
    isNext: false,
    ...overrides,
  };
}

describe("QueueView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    playerMock.state = createPlayerState();
    playerMock.queue = [];
    playerMock.queueLoading = false;
    playerMock.loadQueue.mockClear();
  });

  it("loads the queue on mount and shows the loading state", async () => {
    playerMock.queueLoading = true;
    render(QueueView);

    expect(screen.getByRole("heading", { name: "Playback queue" })).not.toBeNull();
    expect(screen.getByRole("status").textContent).toContain("Loading queue");
    await waitFor(() => expect(playerMock.loadQueue).toHaveBeenCalled());
  });

  it("shows an empty state when nothing is queued", async () => {
    render(QueueView);

    expect(screen.getByText(/Nothing queued/)).not.toBeNull();
    await waitFor(() => expect(playerMock.loadQueue).toHaveBeenCalled());
  });

  it("renders current, next and queued tracks", async () => {
    playerMock.queue = [
      queueItem(0, "Neon Meridian", { isCurrent: true }),
      queueItem(1, "Glass Tides", { isNext: true }),
    ];
    render(QueueView);

    expect(screen.getByText("2 tracks")).not.toBeNull();
    expect(screen.getByText("Neon Meridian")).not.toBeNull();
    expect(screen.getByText("Glass Tides")).not.toBeNull();
    expect(screen.getAllByText("Aster Vale")).toHaveLength(2);
    expect(screen.getByText("Now playing")).not.toBeNull();
    expect(screen.getByText("Next")).not.toBeNull();
    expect(screen.getAllByText("3:00")).toHaveLength(2);
    expect(screen.getByLabelText("Currently playing")).not.toBeNull();
  });

  it("reloads the queue when the current track changes", async () => {
    render(QueueView);
    await waitFor(() => expect(playerMock.loadQueue).toHaveBeenCalledTimes(1));

    playerMock.state = {
      ...playerMock.state,
      track: { ...playerMock.state.track, uri: "spotify:track:2" },
    };
    await tick();

    await waitFor(() => expect(playerMock.loadQueue).toHaveBeenCalledTimes(2));
  });
});
