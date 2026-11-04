import { describe, expect, it } from "vitest";
import { visibleRange } from "./virtual";

describe("visibleRange", () => {
  it("returns an empty window for an empty list", () => {
    expect(visibleRange(0, 100, 20, 0)).toEqual({ start: 0, end: 0 });
  });

  it("returns an empty window for invalid measurements", () => {
    expect(visibleRange(0, 0, 20, 10)).toEqual({ start: 0, end: 0 });
    expect(visibleRange(0, 100, 0, 10)).toEqual({ start: 0, end: 0 });
    expect(visibleRange(0, 100, 20, -1)).toEqual({ start: 0, end: 0 });
  });

  it("clamps the first page to the start", () => {
    expect(visibleRange(0, 100, 20, 50)).toEqual({ start: 0, end: 9 });
  });

  it("adds overscan around the middle", () => {
    expect(visibleRange(500, 100, 20, 50)).toEqual({ start: 21, end: 34 });
  });

  it("clamps the last page to the item count", () => {
    expect(visibleRange(980, 100, 20, 50)).toEqual({ start: 45, end: 50 });
  });

  it("clamps a stale scroll offset past the end", () => {
    expect(visibleRange(10_000, 100, 20, 50)).toEqual({ start: 45, end: 50 });
  });

  it("clamps oversized overscan", () => {
    expect(visibleRange(0, 100, 20, 5, 100)).toEqual({ start: 0, end: 5 });
  });

  it("never returns a negative start", () => {
    expect(visibleRange(-40, 100, 20, 50)).toEqual({ start: 0, end: 9 });
  });
});
