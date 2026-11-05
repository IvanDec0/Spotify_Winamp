import type { MessageKey } from "../shared/i18n";

/**
 * Known local-engine state codes exposed by `AudioState.errorCode`. Unknown
 * codes fall back to a generic headline; the raw error stays a detail.
 */
const ENGINE_ERROR_KEYS: Record<string, MessageKey> = {
  engine_not_installed: "errors.engineNotInstalled",
  engine_config_error: "errors.engineConfigError",
  engine_start_failed: "errors.engineStartFailed",
  engine_not_ready: "errors.engineNotReady",
  engine_stopped: "errors.engineStopped",
  engine_disabled: "errors.engineDisabled",
  engine_not_running: "errors.engineNotRunning",
};

export function engineErrorKey(code: string | null | undefined): MessageKey {
  if (!code) return "errors.engineUnknown";
  return ENGINE_ERROR_KEYS[code] ?? "errors.engineUnknown";
}

export function engineErrorHelpKey(code: string | null | undefined): MessageKey {
  return code === "engine_disabled" ? "errors.engineDisabledHelp" : "errors.engineHelp";
}
