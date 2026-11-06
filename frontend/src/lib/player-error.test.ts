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
