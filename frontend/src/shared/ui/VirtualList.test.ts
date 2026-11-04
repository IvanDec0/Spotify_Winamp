import { fireEvent, render, waitFor } from "@testing-library/svelte";
import { createRawSnippet } from "svelte";
import { describe, expect, it } from "vitest";
import VirtualList from "./VirtualList.svelte";

const children = createRawSnippet<[unknown, number]>((item, index) => ({
  render: () => `<span data-row>${item()}:${index()}</span>`,
}));

function rows(container: HTMLElement): string[] {
  return Array.from(container.querySelectorAll("[data-row]")).map((row) => row.textContent ?? "");
}

describe("VirtualList", () => {
  it("renders only the visible window", () => {
    const items = Array.from({ length: 100 }, (_, index) => index);
    const { container } = render(VirtualList, {
      props: { items, itemHeight: 20, height: 60, overscan: 2, children },
    });

    expect(rows(container)).toEqual(["0:0", "1:1", "2:2", "3:3", "4:4"]);
  });

  it("translates the window when scrolled", async () => {
    const items = Array.from({ length: 100 }, (_, index) => index);
    const { container } = render(VirtualList, {
      props: { items, itemHeight: 20, height: 60, overscan: 2, children },
    });

    const viewport = container.querySelector<HTMLElement>(".ui-virtual-list");
    expect(viewport).not.toBeNull();
    viewport!.scrollTop = 200;
    await fireEvent.scroll(viewport!);

    expect(rows(container)).toEqual(["8:8", "9:9", "10:10", "11:11", "12:12", "13:13", "14:14"]);
    const spacer = container.querySelector<HTMLElement>(".ui-virtual-spacer");
    expect(spacer?.style.height).toBe("2000px");
  });

  it("measures the viewport when no height prop is provided", async () => {
    const original = Object.getOwnPropertyDescriptor(HTMLElement.prototype, "clientHeight");
    Object.defineProperty(HTMLElement.prototype, "clientHeight", {
      configurable: true,
      get: () => 60,
    });

    try {
      const { container } = render(VirtualList, {
        props: {
          items: Array.from({ length: 20 }, (_, index) => index),
          itemHeight: 20,
          overscan: 0,
          children,
        },
      });

      await waitFor(() => expect(rows(container)).toEqual(["0:0", "1:1", "2:2"]));
    } finally {
      if (original) {
        Object.defineProperty(HTMLElement.prototype, "clientHeight", original);
      } else {
        delete (HTMLElement.prototype as { clientHeight?: number }).clientHeight;
      }
    }
  });

  it("renders nothing for an empty list", () => {
    const { container } = render(VirtualList, {
      props: { items: [], itemHeight: 20, height: 60, children },
    });
    expect(rows(container)).toEqual([]);
  });
});
