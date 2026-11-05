import { Service } from "../../../bindings/cadence/internal/wailsapi/visualizer";
import { call } from "./client";

/**
 * Typed visualizer facade. Each window registers itself while it draws so the
 * Go capture runs only when it is needed.
 */
export const visualizer = {
  setActive: (clientId: string, active: boolean) => call(() => Service.SetActive(clientId, active)),
};

export type VisualizerApi = typeof visualizer;
