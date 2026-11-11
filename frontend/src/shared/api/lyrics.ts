import { Get } from "../../../bindings/cadence/internal/wailsapi/lyrics/service";
import type { Track } from "../../../bindings/cadence/internal/wailsapi/lyrics/models";
import { call } from "./client";

export type {
  Line as LyricLine,
  Result as LyricsResult,
  Track as LyricsTrack,
} from "../../../bindings/cadence/internal/wailsapi/lyrics/models";

/**
 * Lyrics facade. The Go service owns the LRCLIB request and its on-disk cache;
 * the frontend only sends track metadata and renders the result.
 */
export const lyrics = {
  get: (track: Track) => call(() => Get(track)),
};

export type LyricsApi = typeof lyrics;
