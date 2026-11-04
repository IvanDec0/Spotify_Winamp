import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  seek: vi.fn(() => Promise.resolve({})),
  setVolume: vi.fn(() => Promise.resolve({})),
}));

vi.mock("../../../bindings/cadence/internal/player", () => ({
  Service: {
    Seek: mocks.seek,
    SetVolume: mocks.setVolume,
  },
}));

import { player } from "./player";

describe("player api", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("rounds seek positions before calling the binding", async () => {
    await player.seek(43_000.4);
    expect(mocks.seek).toHaveBeenCalledWith(43_000);
  });

  it("rounds volume values before calling the binding", async () => {
    await player.setVolume(42.6);
    expect(mocks.setVolume).toHaveBeenCalledWith(43);
  });

  it("keeps integer inputs exact", async () => {
    await player.seek(12_345);
    await player.setVolume(0);
    expect(mocks.seek).toHaveBeenCalledWith(12_345);
    expect(mocks.setVolume).toHaveBeenCalledWith(0);
  });
});
