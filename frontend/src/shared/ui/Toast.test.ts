import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import Toast from "./Toast.svelte";
import type { ToastMessage } from "./toasts.svelte";

const toast: ToastMessage = { id: 7, message: "Skin installed", kind: "success" };

describe("Toast", () => {
  it("renders the message and reports dismissal", async () => {
    const ondismiss = vi.fn();
    render(Toast, { props: { toast, ondismiss } });

    expect(screen.getByText("Skin installed")).not.toBeNull();
    await fireEvent.click(screen.getByRole("button", { name: "Dismiss" }));
    expect(ondismiss).toHaveBeenCalledWith(7);
  });

  it("uses an alert role for errors", () => {
    render(Toast, { props: { toast: { ...toast, kind: "error" }, ondismiss: vi.fn() } });
    expect(screen.getByRole("alert")).not.toBeNull();
  });
});
