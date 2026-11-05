import { fireEvent, render, screen } from "@testing-library/svelte";
import { createRawSnippet } from "svelte";
import { describe, expect, it, vi } from "vitest";
import Button from "./Button.svelte";

const children = createRawSnippet(() => ({
  render: () => "<span>Save changes</span>",
}));

describe("Button", () => {
  it("renders children and reports clicks", async () => {
    const onclick = vi.fn();
    render(Button, { props: { onclick, children } });

    const button = screen.getByRole("button", { name: "Save changes" });
    await fireEvent.click(button);
    expect(onclick).toHaveBeenCalledOnce();
  });

  it("does not fire when disabled", async () => {
    const onclick = vi.fn();
    render(Button, { props: { onclick, disabled: true, children } });

    const button = screen.getByRole("button", { name: "Save changes" });
    expect((button as HTMLButtonElement).disabled).toBe(true);
    await fireEvent.click(button);
    expect(onclick).not.toHaveBeenCalled();
  });

  it("forwards native button attributes", async () => {
    const onkeydown = vi.fn();
    render(Button, {
      props: {
        "aria-pressed": true,
        "aria-expanded": false,
        "data-test": "action",
        style: "width: 120px",
        form: "settings-form",
        onkeydown,
        children,
      },
    });

    const button = screen.getByRole("button", { name: "Save changes" });
    expect(button.getAttribute("aria-pressed")).toBe("true");
    expect(button.getAttribute("aria-expanded")).toBe("false");
    expect(button.getAttribute("data-test")).toBe("action");
    expect(button.getAttribute("style")).toContain("width: 120px");
    expect(button.getAttribute("form")).toBe("settings-form");

    await fireEvent.keyDown(button, { key: "Enter" });
    expect(onkeydown).toHaveBeenCalledOnce();
  });

  it("applies variant, size, type, label and title", () => {
    render(Button, {
      props: {
        variant: "primary",
        size: "lg",
        type: "submit",
        ariaLabel: "Save",
        title: "Save the changes",
        class: "custom",
        children,
      },
    });

    const button = screen.getByRole("button", { name: "Save" });
    expect(button.className).toContain("ui-button--primary");
    expect(button.className).toContain("ui-button--lg");
    expect(button.className).toContain("custom");
    expect(button.getAttribute("type")).toBe("submit");
    expect(button.getAttribute("title")).toBe("Save the changes");
  });
});
