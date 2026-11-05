import { Service } from "../../../bindings/cadence/internal/wailsapi/skins";
import { call } from "./client";

/**
 * Typed skin manager facade. Dialog helpers keep their generated names in
 * camelCase.
 */
export const skins = {
  getCatalog: () => call(() => Service.GetCatalog()),
  getRuntime: () => call(() => Service.GetRuntime()),
  activate: (id: string) => call(() => Service.Activate(id)),
  install: (path: string) => call(() => Service.Install(path)),
  installDirectoryDialog: () => call(() => Service.InstallDirectoryDialog()),
  installZipDialog: () => call(() => Service.InstallZipDialog()),
  reportRuntimeFailure: (id: string, reason: string) =>
    call(() => Service.ReportRuntimeFailure(id, reason)),
  reportRuntimeReady: (id: string, theme: string) =>
    call(() => Service.ReportRuntimeReady(id, theme)),
  reset: () => call(() => Service.Reset()),
  setTheme: (theme: string) => call(() => Service.SetTheme(theme)),
  setThirdPartyDisabled: (disabled: boolean) => call(() => Service.SetThirdPartyDisabled(disabled)),
  uninstall: (id: string) => call(() => Service.Uninstall(id)),
};

export type SkinsApi = typeof skins;
