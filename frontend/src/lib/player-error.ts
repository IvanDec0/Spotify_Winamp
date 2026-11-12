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
  soloist_not_installed: {
    title: "errors.soloistNotInstalled",
    hint: "errors.soloistInstallHint",
  },
  soloist_unsupported_platform: {
    title: "errors.soloistUnsupportedPlatform",
  },
  soloist_api_key_missing: {
    title: "errors.soloistApiKeyMissing",
    hint: "errors.soloistApiKeyHint",
  },
  soloist_api_key_invalid: {
    title: "errors.soloistApiKeyInvalid",
    hint: "errors.soloistApiKeyHint",
  },
  soloist_api_key_rejected: {
    title: "errors.soloistApiKeyRejected",
    hint: "errors.soloistApiKeyHint",
  },
  soloist_not_logged_in: {
    title: "errors.soloistNotLoggedIn",
    hint: "errors.soloistPairingRequiredHint",
  },
  soloist_pairing_required: {
    title: "errors.soloistPairingRequired",
    hint: "errors.soloistPairingRequiredHint",
  },
  soloist_build_expired: {
    title: "errors.soloistBuildExpired",
    hint: "errors.soloistUpdateHint",
  },
  soloist_start_failed: {
    title: "errors.soloistStartFailed",
  },
  soloist_download_failed: {
    title: "errors.soloistDownloadFailed",
  },
  soloist_protocol_error: {
    title: "errors.soloistProtocolError",
  },
  soloist_unsupported: {
    title: "errors.soloistUnsupported",
  },
  webplayer_login_required: {
    title: "errors.webLoginRequired",
    hint: "errors.webLoginHint",
  },
  webplayer_runtime_missing: {
    title: "errors.webRuntimeMissing",
    hint: "errors.webRuntimeHint",
  },
  webplayer_rate_limited: {
    title: "errors.webRateLimited",
    hint: "errors.webRateLimitedHint",
  },
  webplayer_premium: {
    title: "errors.webPremiumRequired",
  },
  webplayer_device_missing: {
    title: "errors.webDeviceMissing",
    hint: "errors.webDeviceHint",
  },
  webplayer_widevine: {
    title: "errors.webWidevine",
    hint: "errors.webWidevineHint",
  },
  webplayer_api: {
    title: "errors.webApiError",
  },
  webplayer_forbidden: {
    title: "errors.webForbidden",
    hint: "errors.webForbiddenHint",
  },
  webplayer_unsupported: {
    title: "errors.webUnsupported",
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
