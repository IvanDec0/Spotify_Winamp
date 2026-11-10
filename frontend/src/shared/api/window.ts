import { Screens, System, Window } from "@wailsio/runtime";
import { WindowService } from "../../../bindings/cadence/internal/platform";
import { call } from "./client";

export type BrowseSection = "nowplaying" | "search" | "library" | "podcasts" | "settings";

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
  setBrowseSize: (width: number, height: number) =>
    call(() => WindowService.SetBrowseSize(width, height)),
  openBrowse: (section: BrowseSection = "library") => call(() => WindowService.OpenBrowse(section)),
  closeBrowse: () => call(() => WindowService.CloseBrowse()),
  browseOpen: () => call(() => WindowService.BrowseOpen()),
};

export const screens = {
  getCurrent: () => call(() => Screens.GetCurrent()),
};

export const system = {
  /**
   * Fire-and-forget native command (for example `wails:drag`). Intentionally
   * not wrapped with `call`: the binding returns void and has no failure path.
   */
  invoke: (message: string): void => System.invoke(message),
};
