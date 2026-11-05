import type { PlayerError } from "../shared/api/types";
import type { MessageKey } from "../shared/i18n";

interface ErrorCopy {
  title: MessageKey;
  hint?: MessageKey;
}

/**
 * Known engine/player codes get translated copy. Anything else is reported as
 * a generic playback error; the raw message stays available as a detail.
 */
const ERROR_COPY: Record<string, ErrorCopy> = {
  not_authenticated: {
    title: "errors.notAuthenticated",
    hint: "errors.notAuthenticatedHint",
  },
  audio_unavailable: {
    title: "errors.audioUnavailable",
  },
  no_active_device: {
    title: "errors.noActiveDevice",
    hint: "errors.noActiveDeviceHint",
  },
};

export interface PlayerErrorCopy {
  /** Headline for a known code; null for unknown errors and no error. */
  titleKey: MessageKey | null;
  /** Status-line label: the known headline or a generic playback error. */
  statusKey: MessageKey | null;
  /** Secondary line (artist slot) for a known code. */
  hintKey: MessageKey | null;
  detail: string;
}

export function playerErrorCopy(error: PlayerError | null | undefined): PlayerErrorCopy {
  if (!error) return { titleKey: null, statusKey: null, hintKey: null, detail: "" };
  const known = ERROR_COPY[error.code];
  return {
    titleKey: known?.title ?? null,
    statusKey: known?.title ?? "errors.playback",
    hintKey: known?.hint ?? known?.title ?? null,
    detail: error.message,
  };
}
