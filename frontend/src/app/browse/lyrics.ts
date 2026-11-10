export interface TimedLine {
  timeMs: number;
}

/**
 * Returns the last line whose timestamp is at or before the playback position.
 * Lines without a timestamp (`timeMs < 0`) are ignored. A track before its
 * first timestamp returns -1 so the UI can keep the first line muted.
 */
export function activeLyricIndex(lines: readonly TimedLine[], positionMs: number): number {
  let index = -1;
  for (let line = 0; line < lines.length; line += 1) {
    const time = lines[line].timeMs;
    if (time >= 0 && time <= positionMs) {
      index = line;
    }
  }
  return index;
}
