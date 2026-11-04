import { Screens, System, Window } from "@wailsio/runtime";
import { WindowService } from "../../../bindings/cadence/internal/platform";
import { call } from "./client";

/**
 * Native window facade. `setSize` uses the platform service because Wails'
 * GTK4 path does not resize an already mapped window.
 */
export const appWindow = {
  size: () => call(() => Window.Size()),
  getZoom: () => call(() => Window.GetZoom()),
  setZoom: (zoom: number) => call(() => Window.SetZoom(zoom)),
  setMinSize: (width: number, height: number) => call(() => Window.SetMinSize(width, height)),
  minimise: () => call(() => Window.Minimise()),
  close: () => call(() => Window.Close()),
  setSize: (width: number, height: number) => call(() => WindowService.SetSize(width, height)),
};

export const screens = {
  getCurrent: () => call(() => Screens.GetCurrent()),
};

export const system = {
  invoke: (message: unknown): void => System.invoke(message),
};
