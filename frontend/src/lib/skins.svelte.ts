import { Events } from "@wailsio/runtime";
import {
  Service as SkinService,
  type InstallResult,
  type LayoutNode,
  type SkinCatalog,
  type SkinRuntime,
} from "../../bindings/cadence/internal/skins";

const VISUALIZER_KEY = "cadence.skin.visualizer";
const SKIN_EVENT = "skins:changed";

export interface SkinHandlers {
  toggleQueue: () => void;
  openDevices: () => void;
  openSettings: () => void;
  openMusic: () => void;
}

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  return typeof error === "string" ? error : "The skin operation failed.";
}

function normalizeCatalog(catalog: SkinCatalog): SkinCatalog {
  return {
    ...catalog,
    skins: catalog.skins ?? [],
    warnings: catalog.warnings ?? [],
  };
}

function normalizeRuntime(runtime: SkinRuntime): SkinRuntime {
  return {
    ...runtime,
    themes: runtime.themes ?? [],
    visualizerModes: runtime.visualizerModes ?? [],
    themeStylesheets: runtime.themeStylesheets ?? {},
    layout: {
      ...runtime.layout,
      root: runtime.layout?.root ?? { type: "column", children: [] },
    },
  };
}

function readStorage(key: string): string | null {
  try {
    return globalThis.localStorage?.getItem(key) ?? null;
  } catch {
    return null;
  }
}

function writeStorage(key: string, value: string): void {
  try {
    globalThis.localStorage?.setItem(key, value);
  } catch {
    // Preferences stay in memory when storage is unavailable.
  }
}

function walkLayout(node: LayoutNode | undefined, visit: (node: LayoutNode) => boolean): boolean {
  if (!node) return false;
  if (visit(node)) return true;
  for (const child of node.children ?? []) {
    if (walkLayout(child, visit)) return true;
  }
  return false;
}

function layoutHasComponent(layout: SkinRuntime["layout"] | null, component: string): boolean {
  return Boolean(
    walkLayout(layout?.root, (node) => node.type === "component" && node.component === component),
  );
}

function layoutVisualizer(layout: SkinRuntime["layout"] | null): string | null {
  let mode: string | null = null;
  walkLayout(layout?.root, (node) => {
    if (node.type === "component" && node.component === "visualizer") {
      const value = node.props?.mode;
      if (typeof value === "string") mode = value;
    }
    return false;
  });
  return mode;
}

function visualizerKey(id: string): string {
  return `${VISUALIZER_KEY}.${id}`;
}

class SkinsStore {
  catalog = $state<SkinCatalog | null>(null);
  runtime = $state<SkinRuntime | null>(null);
  loading = $state(true);
  error = $state<string | null>(null);
  notice = $state<string | null>(null);
  queueOpen = $state(false);
  visualizer = $state("spectrum");
  colors = $state({ accent: "#5ef0ff", secondary: "#7d8cff" });
  palette = $state<Record<string, string>>({});

  #handlers: SkinHandlers | null = null;
  #unsubscribe: (() => void) | null = null;
  #started = false;
  #runtimeRequest = 0;
  #failedRuntime = "";
  #onChanged = (event: { data: unknown }): void => {
    const catalog = normalizeCatalog(event.data as SkinCatalog);
    const previousActive = this.catalog?.activeId;
    const previousVersion = this.catalog?.skins?.find(
      (skin) => skin.id === previousActive,
    )?.version;
    const nextVersion = catalog.skins?.find((skin) => skin.id === catalog.activeId)?.version;
    const activeChanged = previousActive !== catalog.activeId;
    this.catalog = catalog;
    if (activeChanged || previousVersion !== nextVersion) {
      void this.#reloadRuntime();
    }
  };

  get activeSkin() {
    return this.catalog?.skins?.find((skin) => skin.id === this.catalog?.activeId) ?? null;
  }

  get theme(): string {
    return this.catalog?.activeTheme || this.runtime?.defaultTheme || "graphite";
  }

  get isDeclarative(): boolean {
    return Boolean(this.runtime && !this.runtime.builtin);
  }

  get hasQueue(): boolean {
    return layoutHasComponent(this.runtime?.layout ?? null, "queue");
  }

  get hasVisualizer(): boolean {
    return layoutHasComponent(this.runtime?.layout ?? null, "visualizer");
  }

  get visualizerOptions(): string[] {
    return this.runtime?.visualizerModes?.length
      ? this.runtime.visualizerModes
      : ["spectrum", "radial", "oscilloscope", "grid"];
  }

  setHandlers(handlers: SkinHandlers | null): void {
    this.#handlers = handlers;
  }

  toggleQueue(): void {
    this.#handlers?.toggleQueue();
  }

  openDevices(): void {
    this.#handlers?.openDevices();
  }

  openSettings(): void {
    this.#handlers?.openSettings();
  }

  openMusic(): void {
    this.#handlers?.openMusic();
  }

  async start(): Promise<void> {
    if (this.#started) return;
    this.#started = true;
    this.#unsubscribe = Events.On(SKIN_EVENT, this.#onChanged);
    await this.refresh();
  }

  stop(): void {
    this.#unsubscribe?.();
    this.#unsubscribe = null;
    this.#started = false;
  }

  async refresh(): Promise<void> {
    this.loading = true;
    try {
      const [catalog, runtime] = await Promise.all([
        SkinService.GetCatalog(),
        SkinService.GetRuntime(),
      ]);
      this.catalog = normalizeCatalog(catalog);
      this.#applyRuntime(normalizeRuntime(runtime));
      this.error = null;
    } catch (error) {
      this.error = errorMessage(error);
    } finally {
      this.loading = false;
    }
  }

  async #reloadRuntime(): Promise<void> {
    const request = ++this.#runtimeRequest;
    try {
      const runtime = normalizeRuntime(await SkinService.GetRuntime());
      if (request === this.#runtimeRequest) {
        this.#applyRuntime(runtime);
      }
    } catch (error) {
      if (request === this.#runtimeRequest) {
        this.error = errorMessage(error);
      }
    }
  }

  #applyRuntime(runtime: SkinRuntime): void {
    this.runtime = runtime;
    this.#failedRuntime = "";
    if (runtime.builtin) return;

    const modes =
      runtime.visualizerModes && runtime.visualizerModes.length
        ? runtime.visualizerModes
        : ["spectrum"];
    const stored = readStorage(visualizerKey(runtime.id));
    const declared = layoutVisualizer(runtime.layout);
    const preferred = stored || declared || runtime.defaultVisualizer;
    this.visualizer = modes.includes(preferred) ? preferred : modes[0];
  }

  setVisualizer(mode: string): void {
    if (!this.visualizerOptions.includes(mode)) return;
    this.visualizer = mode;
    if (this.runtime) {
      writeStorage(visualizerKey(this.runtime.id), mode);
    }
  }

  cycleVisualizer(): void {
    const options = this.visualizerOptions;
    const index = options.indexOf(this.visualizer);
    this.setVisualizer(options[(index + 1) % options.length]);
  }

  async activate(id: string): Promise<void> {
    try {
      const catalog = await SkinService.Activate(id);
      this.catalog = normalizeCatalog(catalog);
      await this.#reloadRuntime();
      this.error = null;
    } catch (error) {
      this.error = errorMessage(error);
    }
  }

  async setTheme(theme: string): Promise<void> {
    try {
      this.catalog = normalizeCatalog(await SkinService.SetTheme(theme));
      this.error = null;
    } catch (error) {
      this.error = errorMessage(error);
    }
  }

  cycleTheme(): void {
    const options = this.runtime?.themes ?? [];
    if (!options.length) return;
    const index = options.indexOf(this.theme);
    void this.setTheme(options[(index + 1) % options.length]);
  }

  async setThirdPartyDisabled(disabled: boolean): Promise<void> {
    try {
      this.catalog = normalizeCatalog(await SkinService.SetThirdPartyDisabled(disabled));
      await this.#reloadRuntime();
      this.error = null;
    } catch (error) {
      this.error = errorMessage(error);
    }
  }

  async reset(): Promise<void> {
    try {
      this.catalog = normalizeCatalog(await SkinService.Reset());
      await this.#reloadRuntime();
      this.error = null;
    } catch (error) {
      this.error = errorMessage(error);
    }
  }

  async uninstall(id: string): Promise<void> {
    try {
      this.catalog = normalizeCatalog(await SkinService.Uninstall(id));
      await this.#reloadRuntime();
      this.notice = "Skin removed.";
      this.error = null;
    } catch (error) {
      this.error = errorMessage(error);
    }
  }

  async installZip(): Promise<void> {
    await this.#install(() => SkinService.InstallZipDialog());
  }

  async installDirectory(): Promise<void> {
    await this.#install(() => SkinService.InstallDirectoryDialog());
  }

  async #install(action: () => Promise<InstallResult>): Promise<void> {
    try {
      const result = await action();
      if (result.canceled) {
        this.notice = "Installation canceled.";
        return;
      }
      this.catalog = normalizeCatalog(result.catalog);
      this.notice = `${result.skin.name} ${result.skin.version} installed. Activate it when ready.`;
      this.error = null;
    } catch (error) {
      this.error = errorMessage(error);
    }
  }

  reportRuntimeReady(id: string, theme: string): void {
    if (!id || this.runtime?.builtin) return;
    void SkinService.ReportRuntimeReady(id, theme).catch((error: unknown) => {
      this.error = errorMessage(error);
    });
  }

  async reportRuntimeFailure(id: string, reason: string): Promise<void> {
    if (this.#failedRuntime === id) return;
    this.#failedRuntime = id;
    try {
      this.catalog = normalizeCatalog(await SkinService.ReportRuntimeFailure(id, reason));
      await this.#reloadRuntime();
      this.error = "The active skin could not be rendered. Cadence restored a safe skin.";
    } catch (error) {
      this.error = errorMessage(error);
    }
  }

  setPalette(palette: Record<string, string>): void {
    this.palette = palette;
    this.colors = {
      accent: palette["--accent"] || "#5ef0ff",
      secondary: palette["--accent-2"] || "#7d8cff",
    };
  }

  dismissError(): void {
    this.error = null;
  }

  dismissNotice(): void {
    this.notice = null;
  }
}

export const skins = new SkinsStore();
