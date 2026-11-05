import { describe, expect, it } from "vitest";
import {
  bandValue,
  beatEnvelope,
  gridPoint,
  hashString,
  seededUnit,
  signalEnergy,
  spectrumBar,
  trackSeed,
  trackTempo,
  waveSample,
  waveformGain,
  waveformValue,
} from "./visualizer";

describe("deterministic visualizer signals", () => {
  it("hashes strings stably and differently", () => {
    expect(hashString("neon-meridian")).toBe(hashString("neon-meridian"));
    expect(hashString("neon-meridian")).not.toBe(hashString("glass-tides"));
    expect(hashString("")).toBeGreaterThanOrEqual(0);
  });

  it("returns stable unit values in range", () => {
    for (let step = 0; step < 100; step += 1) {
      const value = seededUnit(0.42, step);
      expect(value).toBeGreaterThanOrEqual(0);
      expect(value).toBeLessThan(1);
      expect(seededUnit(0.42, step)).toBe(value);
    }
  });

  it("derives a stable tempo from the track id", () => {
    expect(trackSeed("neon-meridian")).toBe(trackSeed("neon-meridian"));
    expect(trackTempo("neon-meridian")).toBeGreaterThanOrEqual(78);
    expect(trackTempo("neon-meridian")).toBeLessThanOrEqual(122);
  });

  it("accentuates the first beat of each bar", () => {
    expect(beatEnvelope(0, 120)).toBeGreaterThan(beatEnvelope(0.25, 120));
    expect(beatEnvelope(0.075, 120)).toBeGreaterThan(beatEnvelope(0.575, 120));
    expect(beatEnvelope(0, 120)).toBeLessThanOrEqual(1);
  });

  it("uses less energy while paused", () => {
    for (const time of [0, 1, 8, 42.5]) {
      expect(signalEnergy(time, 0.3, false)).toBeLessThan(signalEnergy(time, 0.3, true));
    }
  });

  it("keeps spectrum bars and oscilloscope samples in range", () => {
    for (let index = 0; index < 48; index += 1) {
      const bar = spectrumBar(index, 48, 12.25, 0.61, true);
      expect(bar).toBeGreaterThanOrEqual(0);
      expect(bar).toBeLessThanOrEqual(1);
      expect(spectrumBar(index, 48, 12.25, 0.61, true)).toBe(bar);
    }

    for (let step = 0; step <= 80; step += 1) {
      const sample = waveSample(step / 80, 6.5, 0.61, true);
      expect(sample).toBeGreaterThanOrEqual(-1);
      expect(sample).toBeLessThanOrEqual(1);
    }
  });

  it("keeps grid brightness in range", () => {
    for (let x = 0; x <= 10; x += 1) {
      for (let y = 0; y <= 10; y += 1) {
        const value = gridPoint(x / 10, y / 10, 3.4, 0.2, true);
        expect(value).toBeGreaterThanOrEqual(0);
        expect(value).toBeLessThanOrEqual(1);
      }
    }
  });

  it("tolerates invalid numbers", () => {
    expect(Number.isFinite(spectrumBar(Number.NaN, 0, Number.NaN, Number.NaN, true))).toBe(true);
    expect(Number.isFinite(waveSample(Number.NaN, Number.NaN, Number.NaN, false))).toBe(true);
    expect(Number.isFinite(gridPoint(Number.NaN, Number.NaN, Number.NaN, Number.NaN, false))).toBe(
      true,
    );
    expect(Number.isFinite(beatEnvelope(Number.NaN, Number.NaN))).toBe(true);
  });

  it("maps real bands across any bar count", () => {
    const bands = [0, 0.25, 0.5, 0.75, 1];
    expect(bandValue(bands, 0, 5)).toBe(0);
    expect(bandValue(bands, 4, 5)).toBe(1);
    expect(bandValue(bands, 4, 9)).toBeCloseTo(0.5);
    expect(bandValue([], 0, 10)).toBe(0);

    for (let index = 0; index < 48; index += 1) {
      const value = bandValue(bands, index, 48);
      expect(value).toBeGreaterThanOrEqual(0);
      expect(value).toBeLessThanOrEqual(1);
    }
  });

  it("interpolates real waveform samples and scales quiet frames", () => {
    expect(waveformValue([-1, 1], 0)).toBe(-1);
    expect(waveformValue([-1, 1], 1)).toBe(1);
    expect(waveformValue([-1, 1], 0.5)).toBeCloseTo(0);
    expect(waveformValue([], 0.5)).toBe(0);
    expect(waveformGain([0, 0.01])).toBe(0);
    expect(waveformGain([0, 0.5])).toBeCloseTo(2);
    expect(waveformGain([0, 0.1])).toBeLessThanOrEqual(6);
  });
});
