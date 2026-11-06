import { describe, expect, it } from "vitest";
import type { PlayerError } from "../shared/api/types";
import { playerErrorCopy } from "./player-error";

function error(code: string, message = "raw backend detail"): PlayerError {
  return { code, message, retryable: false };
}

describe("playerErrorCopy", () => {
  it("maps the Soloist pairing code to localized copy and keeps the detail", () => {
    const copy = playerErrorCopy(error("soloist_pairing_required"));

    expect(copy.titleKey).toBe("errors.soloistPairingRequired");
    expect(copy.statusKey).toBe("errors.soloistPairingRequired");
    expect(copy.hintKey).toBe("errors.soloistPairingRequiredHint");
    expect(copy.detail).toBe("raw backend detail");
  });

  it("maps a rejected API key to the API key hint", () => {
    const copy = playerErrorCopy(error("soloist_api_key_rejected"));

    expect(copy.titleKey).toBe("errors.soloistApiKeyRejected");
    expect(copy.hintKey).toBe("errors.soloistApiKeyHint");
    expect(copy.detail).toBe("raw backend detail");
  });

  it("maps an expired build to the update hint", () => {
    const copy = playerErrorCopy(error("soloist_build_expired"));

    expect(copy.titleKey).toBe("errors.soloistBuildExpired");
    expect(copy.hintKey).toBe("errors.soloistUpdateHint");
    expect(copy.detail).toBe("raw backend detail");
  });

  it("maps web player login and runtime codes to localized copy", () => {
    const login = playerErrorCopy(error("webplayer_login_required"));
    expect(login.titleKey).toBe("errors.webLoginRequired");
    expect(login.hintKey).toBe("errors.webLoginHint");

    const runtime = playerErrorCopy(error("webplayer_runtime_missing"));
    expect(runtime.titleKey).toBe("errors.webRuntimeMissing");
    expect(runtime.hintKey).toBe("errors.webRuntimeHint");
  });

  it("maps web player limits, devices and Widevine to localized copy", () => {
    const rateLimited = playerErrorCopy(error("webplayer_rate_limited"));
    expect(rateLimited.titleKey).toBe("errors.webRateLimited");
    expect(rateLimited.hintKey).toBe("errors.webRateLimitedHint");

    const premium = playerErrorCopy(error("webplayer_premium"));
    expect(premium.titleKey).toBe("errors.webPremiumRequired");

    const device = playerErrorCopy(error("webplayer_device_missing"));
    expect(device.titleKey).toBe("errors.webDeviceMissing");
    expect(device.hintKey).toBe("errors.webDeviceHint");

    const widevine = playerErrorCopy(error("webplayer_widevine"));
    expect(widevine.titleKey).toBe("errors.webWidevine");
    expect(widevine.hintKey).toBe("errors.webWidevineHint");

    const api = playerErrorCopy(error("webplayer_api"));
    expect(api.titleKey).toBe("errors.webApiError");

    const unsupported = playerErrorCopy(error("webplayer_unsupported"));
    expect(unsupported.titleKey).toBe("errors.webUnsupported");
  });

  it("falls back to the generic playback copy for unknown codes", () => {
    const copy = playerErrorCopy(error("soloist_something_new"));

    expect(copy.titleKey).toBeNull();
    expect(copy.statusKey).toBe("errors.playback");
    expect(copy.hintKey).toBeNull();
    expect(copy.detail).toBe("raw backend detail");
  });

  it("returns empty copy without an error", () => {
    expect(playerErrorCopy(null)).toEqual({
      titleKey: null,
      statusKey: null,
      hintKey: null,
      detail: "",
    });
  });
});
