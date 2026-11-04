import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "./client";

const mocks = vi.hoisted(() => ({
  size: vi.fn(() => Promise.resolve({ width: 840, height: 440 })),
  getZoom: vi.fn(() => Promise.resolve(1.5)),
  setZoom: vi.fn(() => Promise.resolve()),
  setMinSize: vi.fn(() => Promise.resolve()),
  minimise: vi.fn(() => Promise.resolve()),
  close: vi.fn(() => Promise.resolve()),
  getCurrent: vi.fn(() => Promise.resolve({ ID: "main" })),
  invoke: vi.fn(),
  setSize: vi.fn(() => Promise.resolve()),
}));

vi.mock("@wailsio/runtime", () => ({
  Window: {
    Size: mocks.size,
    GetZoom: mocks.getZoom,
    SetZoom: mocks.setZoom,
    SetMinSize: mocks.setMinSize,
    Minimise: mocks.minimise,
    Close: mocks.close,
  },
  Screens: { GetCurrent: mocks.getCurrent },
  System: { invoke: mocks.invoke },
}));

vi.mock("../../../bindings/cadence/internal/platform", () => ({
  WindowService: { SetSize: mocks.setSize },
}));

import { appWindow, screens, system } from "./window";

describe("window api", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("forwards window, screen and system calls", async () => {
    await expect(appWindow.size()).resolves.toEqual({ width: 840, height: 440 });
    await expect(appWindow.getZoom()).resolves.toBe(1.5);
    await appWindow.setZoom(2);
    await appWindow.setMinSize(600, 300);
    await appWindow.minimise();
    await appWindow.close();
    await appWindow.setSize(840, 920);
    await expect(screens.getCurrent()).resolves.toEqual({ ID: "main" });
    system.invoke("wails:drag");

    expect(mocks.setZoom).toHaveBeenCalledWith(2);
    expect(mocks.setMinSize).toHaveBeenCalledWith(600, 300);
    expect(mocks.setSize).toHaveBeenCalledWith(840, 920);
    expect(mocks.invoke).toHaveBeenCalledWith("wails:drag");
  });

  it("normalizes native failures", async () => {
    mocks.size.mockRejectedValueOnce("window not ready");
    await expect(appWindow.size()).rejects.toBeInstanceOf(ApiError);
  });
});
