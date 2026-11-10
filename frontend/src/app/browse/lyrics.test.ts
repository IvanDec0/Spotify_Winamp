import { describe, expect, it } from "vitest";
import { activeLyricIndex } from "./lyrics";

const lines = [{ timeMs: 0 }, { timeMs: 1_000 }, { timeMs: 2_500 }];

describe("activeLyricIndex", () => {
  it("returns the last line at or before the position", () => {
    expect(activeLyricIndex(lines, 0)).toBe(0);
    expect(activeLyricIndex(lines, 999)).toBe(0);
    expect(activeLyricIndex(lines, 1_000)).toBe(1);
    expect(activeLyricIndex(lines, 2_499)).toBe(1);
    expect(activeLyricIndex(lines, 2_500)).toBe(2);
    expect(activeLyricIndex(lines, 9_999)).toBe(2);
  });

  it("returns -1 before the first timestamp and for plain lines", () => {
    expect(activeLyricIndex([{ timeMs: 800 }, { timeMs: 1_200 }], 400)).toBe(-1);
    expect(activeLyricIndex([{ timeMs: -1 }, { timeMs: -1 }], 400)).toBe(-1);
    expect(activeLyricIndex([], 400)).toBe(-1);
  });

  it("ignores unsynced lines interleaved with timed lines", () => {
    expect(activeLyricIndex([{ timeMs: 100 }, { timeMs: -1 }, { timeMs: 200 }], 150)).toBe(0);
    expect(activeLyricIndex([{ timeMs: 100 }, { timeMs: -1 }, { timeMs: 200 }], 250)).toBe(2);
  });
});
