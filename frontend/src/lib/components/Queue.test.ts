import { render, screen } from "@testing-library/svelte";
import { describe, expect, it } from "vitest";
import type { QueueItem } from "../../../bindings/cadence/internal/player/models";
import Queue from "./Queue.svelte";

const items: QueueItem[] = [
  {
    track: {
      id: "neon-meridian",
      title: "Neon Meridian",
      artists: ["Aster Vale"],
      album: "Night Cartography",
      cover: "meridian",
      uri: "",
      durationMs: 252_000,
    },
    index: 0,
    isCurrent: true,
    isNext: false,
  },
  {
    track: {
      id: "glass-tides",
      title: "Glass Tides",
      artists: ["Lumen Field"],
      album: "Soft Machines",
      cover: "tides",
      uri: "",
      durationMs: 228_000,
    },
    index: 1,
    isCurrent: false,
    isNext: true,
  },
];

describe("Queue", () => {
  it("renders the current item, the next item and the count", () => {
    render(Queue, { props: { items } });

    expect(screen.getByLabelText("Playback queue")).not.toBeNull();
    expect(screen.getByText("Up next")).not.toBeNull();
    expect(document.querySelector(".queue-count")?.textContent).toBe("02");
    expect(screen.getByText("Neon Meridian")).not.toBeNull();
    expect(screen.getByText("Glass Tides")).not.toBeNull();
    expect(screen.getByText("Next")).not.toBeNull();
    expect(screen.getByLabelText("Currently playing")).not.toBeNull();
    expect(document.querySelector('[aria-current="true"]')?.textContent).toContain("Neon Meridian");
    expect(screen.getByText("4:12")).not.toBeNull();
  });

  it("shows a useful empty state", () => {
    render(Queue, { props: { items: [] } });
    expect(screen.getByText(/Nothing queued/)).not.toBeNull();
  });
});
