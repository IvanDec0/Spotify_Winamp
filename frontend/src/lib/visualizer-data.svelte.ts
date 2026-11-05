import { onVisualizerFrame, visualizer as visualizerApi } from "../shared/api";
import type { VisualizerFrame } from "../shared/api/types";

/** A frame older than this is ignored and the simulated animation takes over. */
export const VISUALIZER_FRESHNESS_MS = 400;

function createClientID(): string {
  if (typeof globalThis.crypto?.randomUUID === "function") {
    return globalThis.crypto.randomUUID();
  }
  return `visualizer-${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
}

/**
 * Per-window visualizer data. The Go service keeps the monitor capture alive
 * while any window has an active client, so both the compact player and Browse
 * share one capture without coordinating with each other.
 */
export class VisualizerDataStore {
  frame = $state<VisualizerFrame | null>(null);
  readonly clientId = createClientID();

  #unsubscribe: (() => void) | null = null;
  #active = false;

  /** Registers or releases this window. Duplicate states are ignored. */
  setActive(active: boolean): void {
    if (active === this.#active) return;
    this.#active = active;
    if (active) {
      this.#subscribe();
    } else {
      // Do not keep drawing the last live frame while deactivated.
      this.frame = null;
    }
    this.#send(active);
  }

  /** True when a monitor frame arrived within the freshness window. */
  isFresh(now = Date.now()): boolean {
    const frame = this.frame;
    if (!frame) return false;
    return now - frame.at <= VISUALIZER_FRESHNESS_MS;
  }

  /** Releases the window and stops listening. A later setActive resubscribes. */
  stop(): void {
    const wasActive = this.#active;
    this.#active = false;
    this.#unsubscribe?.();
    this.#unsubscribe = null;
    this.frame = null;
    if (wasActive) this.#send(false);
  }

  #subscribe(): void {
    if (this.#unsubscribe || typeof onVisualizerFrame !== "function") return;
    try {
      this.#unsubscribe = onVisualizerFrame((frame) => {
        this.frame = frame;
      });
    } catch {
      // No event bridge (tests/preview): keep the simulated fallback.
      this.#unsubscribe = null;
    }
  }

  #send(active: boolean): void {
    if (typeof visualizerApi?.setActive !== "function") return;
    try {
      void Promise.resolve(visualizerApi.setActive(this.clientId, active)).catch(() => {
        // A missing parec/monitor is not fatal: the visualizer stays simulated.
      });
    } catch {
      // The Wails bridge is unavailable (preview/tests): use the simulation.
    }
  }
}

export const visualizerData = new VisualizerDataStore();
