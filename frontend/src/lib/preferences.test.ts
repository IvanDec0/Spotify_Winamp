import { describe, expect, it } from "vitest";
import {
  isMotionPreference,
  isThemeId,
  isVisualizerMode,
  nextVisualizerMode,
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
});
