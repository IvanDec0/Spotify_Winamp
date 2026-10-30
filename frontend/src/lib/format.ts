/**
 * Formats a non-negative duration in seconds as `m:ss`.
 *
 * Values that are negative, infinite or NaN are treated as zero so a bad
 * provider payload can never leak `NaN` into the interface.
 */
export function formatTime(seconds: number): string {
  const safe = Number.isFinite(seconds) && seconds > 0 ? Math.floor(seconds) : 0;
  const minutes = Math.floor(safe / 60);
  const remainder = safe % 60;
  return `${minutes}:${remainder.toString().padStart(2, "0")}`;
}
