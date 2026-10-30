import { describe, expect, it } from "vitest";
import { formatTime } from "./format";

describe("formatTime", () => {
  it("formats whole seconds and minutes", () => {
    expect(formatTime(0)).toBe("0:00");
    expect(formatTime(9)).toBe("0:09");
    expect(formatTime(61)).toBe("1:01");
    expect(formatTime(600)).toBe("10:00");
  });

  it("floors fractional seconds", () => {
    expect(formatTime(59.9)).toBe("0:59");
  });

  it("coerces invalid values to zero", () => {
    expect(formatTime(-4)).toBe("0:00");
    expect(formatTime(Number.NaN)).toBe("0:00");
    expect(formatTime(Number.POSITIVE_INFINITY)).toBe("0:00");
  });
});
