export interface VisibleRange {
  /** First rendered index, inclusive. */
  start: number;
  /** Last rendered index, exclusive. */
  end: number;
}

/**
 * Computes the translated window for a fixed-height virtual list. The first
 * visible row is clamped to the last item so a stale scroll offset cannot
 * produce an empty window past the end of the list.
 */
export function visibleRange(
  scrollTop: number,
  viewportHeight: number,
  itemHeight: number,
  count: number,
  overscan = 4,
): VisibleRange {
  if (count <= 0 || itemHeight <= 0 || viewportHeight <= 0) {
    return { start: 0, end: 0 };
  }

  const padding = Math.max(0, Math.floor(overscan));
  const rawFirst = scrollTop / itemHeight;
  const first = Math.min(count - 1, Math.max(0, Math.floor(rawFirst)));
  const offset = Math.max(0, scrollTop - first * itemHeight);
  const visible = Math.max(1, Math.ceil((offset + viewportHeight) / itemHeight));
  const start = Math.max(0, first - padding);
  const end = Math.min(count, first + visible + padding);

  return { start, end };
}
