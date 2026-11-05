import { describe, expect, it, vi } from "vitest";
import { ApiError, call, toApiError, UNKNOWN_ERROR_CODE } from "./client";

describe("toApiError", () => {
  it("returns an existing ApiError unchanged", () => {
    const original = new ApiError("engine offline", "network");
    expect(toApiError(original)).toBe(original);
  });

  it("normalizes strings", () => {
    const error = toApiError("pairing expired");
    expect(error).toBeInstanceOf(ApiError);
    expect(error.message).toBe("pairing expired");
    expect(error.code).toBe(UNKNOWN_ERROR_CODE);
    expect(error.cause).toBe("pairing expired");
  });

  it("normalizes Error instances and keeps a code when present", () => {
    const cause = Object.assign(new Error("token missing"), { code: "not_authenticated" });
    const error = toApiError(cause);
    expect(error).toBeInstanceOf(ApiError);
    expect(error.message).toBe("token missing");
    expect(error.code).toBe("not_authenticated");
    expect(error.cause).toBe(cause);
  });

  it("normalizes {message, code} payloads", () => {
    const error = toApiError({ message: "device is restricted", code: "restricted_device" });
    expect(error.message).toBe("device is restricted");
    expect(error.code).toBe("restricted_device");
  });

  it("keeps a code-only payload", () => {
    const error = toApiError({ code: "rate_limited" });
    expect(error.message).toBe("The operation failed.");
    expect(error.code).toBe("rate_limited");
  });

  it("falls back for unknown values", () => {
    const error = toApiError(42);
    expect(error.message).toBe("The operation failed.");
    expect(error.code).toBe(UNKNOWN_ERROR_CODE);
    expect(error.cause).toBe(42);
    expect(toApiError(null).message).toBe("The operation failed.");
  });
});

describe("call", () => {
  it("returns the resolved value", async () => {
    await expect(call(() => Promise.resolve("ok"))).resolves.toBe("ok");
  });

  it("rethrows failures as ApiError", async () => {
    const failure = Promise.reject(new Error("engine did not respond"));
    await expect(call(() => failure)).rejects.toBeInstanceOf(ApiError);
    await expect(call(() => Promise.reject("denied"))).rejects.toMatchObject({
      name: "ApiError",
      message: "denied",
      code: UNKNOWN_ERROR_CODE,
    });
  });

  it("does not double-wrap an ApiError", async () => {
    const original = new ApiError("already normalized", "network");
    let caught: unknown;
    try {
      await call(() => Promise.reject(original));
    } catch (error) {
      caught = error;
    }
    expect(caught).toBe(original);
  });

  it("runs the operation exactly once", async () => {
    const run = vi.fn(() => Promise.resolve(3));
    await call(run);
    expect(run).toHaveBeenCalledOnce();
  });
});
