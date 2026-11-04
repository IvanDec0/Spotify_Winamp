import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ToastStore } from "./toasts.svelte";

describe("ToastStore", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("pushes and auto-dismisses entries after their timeout", () => {
    const store = new ToastStore();
    store.push("Saved", "success", 1000);
    store.push("Still here", "info", 5000);

    expect(store.toasts).toHaveLength(2);

    vi.advanceTimersByTime(1000);
    expect(store.toasts.map((toast) => toast.message)).toEqual(["Still here"]);

    vi.advanceTimersByTime(4000);
    expect(store.toasts).toHaveLength(0);
  });

  it("clears the timer when an entry is dismissed manually", () => {
    const store = new ToastStore();
    const clear = vi.spyOn(globalThis, "clearTimeout");
    const id = store.push("Retry later", "error", 1000);

    store.dismiss(id);

    expect(store.toasts).toHaveLength(0);
    expect(clear).toHaveBeenCalledOnce();

    vi.advanceTimersByTime(2000);
    expect(store.toasts).toHaveLength(0);
  });

  it("keeps entries without a timeout until dismissed", () => {
    const store = new ToastStore();
    const id = store.push("Sticky", "info", 0);

    vi.advanceTimersByTime(60_000);
    expect(store.toasts).toHaveLength(1);

    store.dismiss(id);
    expect(store.toasts).toHaveLength(0);
  });

  it("clears every timer on clear", () => {
    const store = new ToastStore();
    const clear = vi.spyOn(globalThis, "clearTimeout");
    store.push("One", "info", 1000);
    store.push("Two", "info", 2000);

    store.clear();

    expect(store.toasts).toHaveLength(0);
    expect(clear).toHaveBeenCalledTimes(2);
  });

  it("gives every entry a unique id", () => {
    const store = new ToastStore();
    const first = store.push("First");
    const second = store.push("Second");
    expect(first).not.toBe(second);
  });
});
