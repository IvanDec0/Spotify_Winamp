import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  setActive: vi.fn(() => Promise.resolve()),
}));

vi.mock("../../../bindings/cadence/internal/wailsapi/visualizer", () => ({
  Service: { SetActive: mocks.setActive },
}));

import { visualizer } from "./visualizer";

describe("visualizer api", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("forwards the client id and active flag", async () => {
    await visualizer.setActive("window-client", true);
    expect(mocks.setActive).toHaveBeenCalledWith("window-client", true);

    await visualizer.setActive("window-client", false);
    expect(mocks.setActive).toHaveBeenLastCalledWith("window-client", false);
  });
});
