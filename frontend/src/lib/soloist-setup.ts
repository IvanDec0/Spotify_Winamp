/**
 * Shared storage for the one-time Soloist setup prompt. Both the compact
 * player and Browse read it, so a dismissal made in one webview is honoured
 * by the other and survives the next launch.
 */
export const SOLOIST_PROMPT_KEY = "cadence.soloist.prompt.v1";

export function readSoloistPromptDismissed(): boolean {
  try {
    return localStorage.getItem(SOLOIST_PROMPT_KEY) === "1";
  } catch {
    return false;
  }
}

export function writeSoloistPromptDismissed(dismissed: boolean): void {
  try {
    if (dismissed) localStorage.setItem(SOLOIST_PROMPT_KEY, "1");
    else localStorage.removeItem(SOLOIST_PROMPT_KEY);
  } catch {
    // Storage can be unavailable; the prompt still respects this session.
  }
}
