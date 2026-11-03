const interactiveSelector =
  "button, input, textarea, select, a, [role='slider'], [role='button'], [data-no-drag]";

export function isInteractiveSkinElement(element: Element): boolean {
  return Boolean(element.closest(interactiveSelector));
}

/**
 * Returns the skin titlebar that should start a native drag, or null when the
 * composed event path starts on an interactive control. Shadow DOM retargets
 * mouse events to the host before Wails sees them, so this app-owned fallback
 * inspects the full composed path instead of relying on CSS variables.
 */
export function titlebarDragTarget(path: EventTarget[]): HTMLElement | null {
  for (const item of path) {
    if (!(item instanceof Element)) continue;
    if (isInteractiveSkinElement(item)) return null;
    if (item.classList.contains("cad-titlebar")) {
      return item instanceof HTMLElement ? item : null;
    }
  }
  return null;
}
