import { fireEvent, render, screen, waitFor } from "@testing-library/svelte";
import { createRawSnippet } from "svelte";
import { describe, expect, it, vi } from "vitest";
import Dialog from "./Dialog.svelte";

const children = createRawSnippet(() => ({
  render: () => `
    <div>
      <input aria-label="Name" />
      <button type="button">Confirm</button>
    </div>
  `,
}));

const hiddenOnly = createRawSnippet(() => ({
  render: () => `
    <div>
      <input type="hidden" aria-label="Hidden field" />
    </div>
  `,
}));

const hiddenBeforeButton = createRawSnippet(() => ({
  render: () => `
    <div>
      <input type="hidden" aria-label="Hidden field" />
      <button type="button">Visible action</button>
    </div>
  `,
}));

const negativeTabindexBeforeButton = createRawSnippet(() => ({
  render: () => `
    <div>
      <button type="button" tabindex="-1">Programmatic</button>
      <button type="button">Visible action</button>
    </div>
  `,
}));

const autofocusBeforeClose = createRawSnippet(() => ({
  render: () => `
    <div>
      <button type="button">Visible action</button>
      <button type="button" data-autofocus>Generate API key</button>
    </div>
  `,
}));

function props(open: boolean, onclose = vi.fn()) {
  return { open, title: "Test dialog", onclose, children };
}

describe("Dialog", () => {
  it("exposes a modal dialog labelled by its title", () => {
    render(Dialog, { props: props(true) });

    const dialog = screen.getByRole("dialog", { name: "Test dialog" });
    expect(dialog.getAttribute("aria-modal")).toBe("true");
    expect(screen.getByRole("button", { name: "Close" })).not.toBeNull();
    expect(screen.getByRole("button", { name: "Dismiss" })).not.toBeNull();
  });

  it("focuses the first focusable element and restores focus on close", async () => {
    const trigger = document.createElement("button");
    trigger.textContent = "Open dialog";
    document.body.appendChild(trigger);
    trigger.focus();

    const { rerender } = render(Dialog, { props: props(true) });

    await waitFor(() =>
      expect(document.activeElement).toBe(screen.getByRole("button", { name: "Close" })),
    );

    await rerender({ open: false });
    await waitFor(() => expect(document.activeElement).toBe(trigger));

    trigger.remove();
  });

  it("closes on Escape and on backdrop click", async () => {
    const onclose = vi.fn();
    render(Dialog, { props: props(true, onclose) });

    await fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
    expect(onclose).toHaveBeenCalledOnce();

    await fireEvent.click(screen.getByRole("button", { name: "Dismiss" }));
    expect(onclose).toHaveBeenCalledTimes(2);
  });

  it("traps Tab inside the dialog", async () => {
    render(Dialog, { props: props(true) });
    const dialog = screen.getByRole("dialog");
    const close = screen.getByRole("button", { name: "Close" });
    const confirm = screen.getByRole("button", { name: "Confirm" });

    confirm.focus();
    await fireEvent.keyDown(dialog, { key: "Tab" });
    expect(document.activeElement).toBe(close);

    close.focus();
    await fireEvent.keyDown(dialog, { key: "Tab", shiftKey: true });
    expect(document.activeElement).toBe(confirm);
  });

  it("skips hidden inputs when looking for the first focusable element", async () => {
    render(Dialog, {
      props: { open: true, title: "Test dialog", onclose: vi.fn(), children: hiddenBeforeButton },
    });
    screen.getByRole("button", { name: "Close" }).setAttribute("disabled", "");

    await waitFor(() =>
      expect(document.activeElement).toBe(screen.getByRole("button", { name: "Visible action" })),
    );
  });

  it("skips tabindex=-1 elements when looking for the first focusable element", async () => {
    render(Dialog, {
      props: {
        open: true,
        title: "Test dialog",
        onclose: vi.fn(),
        children: negativeTabindexBeforeButton,
      },
    });
    screen.getByRole("button", { name: "Close" }).setAttribute("disabled", "");

    await waitFor(() =>
      expect(document.activeElement).toBe(screen.getByRole("button", { name: "Visible action" })),
    );
  });

  it("prefers a body element marked data-autofocus over the close button", async () => {
    render(Dialog, {
      props: {
        open: true,
        title: "Test dialog",
        onclose: vi.fn(),
        children: autofocusBeforeClose,
      },
    });

    await waitFor(() =>
      expect(document.activeElement).toBe(screen.getByRole("button", { name: "Generate API key" })),
    );
  });

  it("focuses the dialog itself when there are no focusable children", async () => {
    const onclose = vi.fn();
    render(Dialog, {
      props: { open: true, title: "Test dialog", onclose, children: hiddenOnly },
    });
    screen.getByRole("button", { name: "Close" }).setAttribute("disabled", "");

    const dialog = screen.getByRole("dialog");
    await waitFor(() => expect(document.activeElement).toBe(dialog));

    await fireEvent.keyDown(dialog, { key: "Escape" });
    expect(onclose).toHaveBeenCalledOnce();
  });

  it("stops Escape propagation out of the dialog", async () => {
    const onclose = vi.fn();
    const bubble = vi.fn();
    document.body.addEventListener("keydown", bubble);
    render(Dialog, { props: props(true, onclose) });

    await fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });

    expect(onclose).toHaveBeenCalledOnce();
    expect(bubble).not.toHaveBeenCalled();
    document.body.removeEventListener("keydown", bubble);
  });

  it("renders nothing while closed", () => {
    render(Dialog, { props: props(false) });
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});
