/**
 * The UI keeps a fixed logical coordinate space and scales it to the native
 * window. This keeps skin layouts stable while allowing the player to grow or
 * shrink with the window.
 */
export const WINDOW_WIDTH = 420;
export const WINDOW_HEIGHT = 220;
export const EXPANDED_HEIGHT = 460;

export const DEFAULT_UI_SCALE = 2;
export const MIN_UI_SCALE = 1.25;
export const MAX_UI_SCALE = 3;

export function clampUiScale(scale: number): number {
  if (!Number.isFinite(scale)) return DEFAULT_UI_SCALE;
  return Math.min(MAX_UI_SCALE, Math.max(MIN_UI_SCALE, scale));
}

/**
 * Returns the largest scale that fits a native window while keeping the
 * current logical height visible.
 */
export function fitUiScale(
  nativeWidth: number,
  nativeHeight: number,
  logicalHeight: number,
): number {
  if (nativeWidth <= 0 || nativeHeight <= 0 || logicalHeight <= 0) {
    return DEFAULT_UI_SCALE;
  }
  return clampUiScale(Math.min(nativeWidth / WINDOW_WIDTH, nativeHeight / logicalHeight));
}

export function roundedUiScale(scale: number): number {
  return Math.round(clampUiScale(scale) * 100) / 100;
}
