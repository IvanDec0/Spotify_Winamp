import { describe, expect, it } from "vitest";
import { coverLoading, coverURL } from "./artwork";

describe("coverURL", () => {
  it("maps the 2x logical size to the nearest backend variant", () => {
    expect(coverURL("/art/cover", 24)).toBe("/art/cover?size=64");
    expect(coverURL("/art/cover", 32)).toBe("/art/cover?size=64");
    expect(coverURL("/art/cover", 40)).toBe("/art/cover?size=64");
    expect(coverURL("/art/cover", 41)).toBe("/art/cover?size=128");
    expect(coverURL("/art/cover", 80)).toBe("/art/cover?size=128");
    expect(coverURL("/art/cover", 81)).toBe("/art/cover?size=240");
    expect(coverURL("/art/cover", 160)).toBe("/art/cover?size=240");
    expect(coverURL("/art/cover", 161)).toBe("/art/cover?size=640");
    expect(coverURL("/art/cover", 400)).toBe("/art/cover?size=640");
  });

  it("leaves non-backend covers untouched", () => {
    expect(coverURL("meridian", 88)).toBe("meridian");
    expect(coverURL("https://i.scdn.co/image/abc", 34)).toBe("https://i.scdn.co/image/abc");
    expect(coverURL("", 64)).toBe("");
  });
});

describe("coverLoading", () => {
  it("loads small covers lazily and visible art eagerly", () => {
    expect(coverLoading(24)).toBe("lazy");
    expect(coverLoading(40)).toBe("lazy");
    expect(coverLoading(41)).toBe("eager");
    expect(coverLoading(88)).toBe("eager");
  });
});
