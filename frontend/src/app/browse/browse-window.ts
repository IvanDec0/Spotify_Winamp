export const BROWSE_MIN_WIDTH = 720;
export const BROWSE_MIN_HEIGHT = 480;

export interface BrowseSize {
  width: number;
  height: number;
}

/**
 * Computes a Browse window size from the pointer delta. The native service
 * clamps too; this keeps the grip preview and throttle aligned with the floor.
 */
export function clampBrowseSize(
  startWidth: number,
  startHeight: number,
  dx: number,
  dy: number,
): BrowseSize {
  return {
    width: Math.max(BROWSE_MIN_WIDTH, Math.round(startWidth + dx)),
    height: Math.max(BROWSE_MIN_HEIGHT, Math.round(startHeight + dy)),
  };
}
