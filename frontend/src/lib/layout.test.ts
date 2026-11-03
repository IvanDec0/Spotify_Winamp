import { describe, expect, it } from "vitest";
import { DEFAULT_UI_SCALE, MAX_UI_SCALE, MIN_UI_SCALE, fitUiScale, roundedUiScale } from "./layout";

describe("UI scale", () => {
  it("matches the base and expanded design sizes", () => {
    expect(fitUiScale(840, 440, 220)).toBe(2);
    expect(fitUiScale(840, 920, 460)).toBe(2);
  });

  it("shrinks and grows with the native window", () => {
    expect(fitUiScale(630, 330, 220)).toBe(1.5);
    expect(fitUiScale(1050, 660, 220)).toBe(2.5);
    expect(fitUiScale(1260, 660, 220)).toBe(3);
  });

  it("uses the limiting dimension", () => {
    expect(fitUiScale(840, 660, 220)).toBe(2);
    expect(fitUiScale(1260, 440, 220)).toBe(2);
    expect(fitUiScale(1000, 690, 460)).toBe(1.5);
  });

  it("clamps scales outside the supported range", () => {
    expect(fitUiScale(100, 100, 220)).toBe(MIN_UI_SCALE);
    expect(fitUiScale(4000, 4000, 220)).toBe(MAX_UI_SCALE);
    expect(fitUiScale(0, 0, 220)).toBe(DEFAULT_UI_SCALE);
    expect(roundedUiScale(2.006)).toBe(2.01);
  });
});
