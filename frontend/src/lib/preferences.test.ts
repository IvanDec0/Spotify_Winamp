import { describe, expect, it, vi } from "vitest";
import {
  isMotionPreference,
  isThemeId,
  isVisualizerMode,
  nextVisualizerMode,
  preferences,
  themes,
  visualizerModes,
} from "./preferences.svelte";

describe("preference guards", () => {
  it("accepts only known themes", () => {
    for (const theme of themes) {
      expect(isThemeId(theme.id)).toBe(true);
    }
    expect(isThemeId("neon")).toBe(false);
    expect(isThemeId(null)).toBe(false);
  });

  it("accepts only known visualizer modes", () => {
    for (const mode of visualizerModes) {
      expect(isVisualizerMode(mode.id)).toBe(true);
    }
    expect(isVisualizerMode("bars")).toBe(false);
  });

  it("accepts only known motion preferences", () => {
    expect(isMotionPreference("system")).toBe(true);
    expect(isMotionPreference("reduced")).toBe(true);
    expect(isMotionPreference("full")).toBe(false);
  });

  it("cycles visualizer modes and wraps", () => {
    expect(nextVisualizerMode("spectrum")).toBe("radial");
    expect(nextVisualizerMode("radial")).toBe("oscilloscope");
    expect(nextVisualizerMode("oscilloscope")).toBe("grid");
    expect(nextVisualizerMode("grid")).toBe("spectrum");
  });

  it("synchronizes storage changes without writing them back", () => {
    const setItem = vi.spyOn(Storage.prototype, "setItem");
    const stop = preferences.startStorageSync();
    try {
      window.dispatchEvent(new StorageEvent("storage", { key: "cadence.theme", newValue: "ice" }));
      window.dispatchEvent(
        new StorageEvent("storage", { key: "cadence.visualizer", newValue: "radial" }),
      );
      window.dispatchEvent(
        new StorageEvent("storage", { key: "cadence.motion", newValue: "reduced" }),
      );

      expect(preferences.theme).toBe("ice");
      expect(preferences.visualizer).toBe("radial");
      expect(preferences.motion).toBe("reduced");
      expect(setItem).not.toHaveBeenCalled();
    } finally {
      stop();
      setItem.mockRestore();
    }
  });

  it("ignores unknown storage values", () => {
    const stop = preferences.startStorageSync();
    const before = {
      theme: preferences.theme,
      visualizer: preferences.visualizer,
      motion: preferences.motion,
    };
    try {
      window.dispatchEvent(new StorageEvent("storage", { key: "cadence.theme", newValue: "neon" }));
      window.dispatchEvent(
        new StorageEvent("storage", { key: "cadence.motion", newValue: "fast" }),
      );

      expect(preferences.theme).toBe(before.theme);
      expect(preferences.visualizer).toBe(before.visualizer);
      expect(preferences.motion).toBe(before.motion);
    } finally {
      stop();
    }
  });
});
