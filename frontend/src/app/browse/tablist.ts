/**
 * Resolves roving-focus keyboard navigation for a horizontal tab list. The
 * components use the returned index to activate and focus the next tab.
 */
export function nextTabIndex(key: string, current: number, count: number): number | null {
  if (count <= 0) return null;
  switch (key) {
    case "ArrowRight":
      return (current + 1) % count;
    case "ArrowLeft":
      return (current - 1 + count) % count;
    case "Home":
      return 0;
    case "End":
      return count - 1;
    default:
      return null;
  }
}

/** Focus and reveal a tab inside an overflowing tab strip. */
export function revealTab(id: string): void {
  const element = document.getElementById(id);
  if (!(element instanceof HTMLElement)) return;
  element.focus();
  element.scrollIntoView?.({ block: "nearest", inline: "nearest" });
}
