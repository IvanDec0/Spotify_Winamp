import { onSkinsChanged, skins as skinsApi, toApiError } from "../shared/api";
import type { InstallResult, LayoutNode, SkinCatalog, SkinRuntime } from "../shared/api/types";
import type { MessageKey, MessageParams } from "../shared/i18n";

const VISUALIZER_KEY = "cadence.skin.visualizer";

export interface SkinHandlers {
  toggleQueue: () => void;
  openDevices: () => void;
  openSettings: () => void;
  openMusic: () => void;
}

export interface SkinsNotice {
  key: MessageKey;
  params?: MessageParams;
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
  errorKey = $state<MessageKey | null>(null);
  notice = $state<SkinsNotice | null>(null);
  queueOpen = $state(false);
  visualizer = $state("spectrum");
  colors = $state({ accent: "#5ef0ff", secondary: "#7d8cff" });
  palette = $state<Record<string, string>>({});

  #handlers: SkinHandlers | null = null;
  #unsubscribe: (() => void) | null = null;
  #started = false;
  #runtimeRequest = 0;
  #failedRuntime = "";

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
    this.#unsubscribe = onSkinsChanged((next) => {
      const catalog = normalizeCatalog(next);
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
    });
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
      const [catalog, runtime] = await Promise.all([skinsApi.getCatalog(), skinsApi.getRuntime()]);
      this.catalog = normalizeCatalog(catalog);
      this.#applyRuntime(normalizeRuntime(runtime));
      this.error = null;
      this.errorKey = null;
    } catch (error) {
      this.error = toApiError(error).message;
      this.errorKey = null;
    } finally {
      this.loading = false;
    }
  }

  async #reloadRuntime(): Promise<void> {
    const request = ++this.#runtimeRequest;
    try {
      const runtime = normalizeRuntime(await skinsApi.getRuntime());
      if (request === this.#runtimeRequest) {
        this.#applyRuntime(runtime);
      }
    } catch (error) {
      if (request === this.#runtimeRequest) {
        this.error = toApiError(error).message;
        this.errorKey = null;
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
      const catalog = await skinsApi.activate(id);
      this.catalog = normalizeCatalog(catalog);
      await this.#reloadRuntime();
      this.error = null;
      this.errorKey = null;
    } catch (error) {
      this.error = toApiError(error).message;
      this.errorKey = null;
    }
  }

  async setTheme(theme: string): Promise<void> {
    try {
      this.catalog = normalizeCatalog(await skinsApi.setTheme(theme));
      this.error = null;
      this.errorKey = null;
    } catch (error) {
      this.error = toApiError(error).message;
      this.errorKey = null;
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
      this.catalog = normalizeCatalog(await skinsApi.setThirdPartyDisabled(disabled));
      await this.#reloadRuntime();
      this.error = null;
      this.errorKey = null;
    } catch (error) {
      this.error = toApiError(error).message;
      this.errorKey = null;
    }
  }

  async reset(): Promise<void> {
    try {
      this.catalog = normalizeCatalog(await skinsApi.reset());
      await this.#reloadRuntime();
      this.error = null;
      this.errorKey = null;
    } catch (error) {
      this.error = toApiError(error).message;
      this.errorKey = null;
    }
  }

  async uninstall(id: string): Promise<void> {
    try {
      this.catalog = normalizeCatalog(await skinsApi.uninstall(id));
      await this.#reloadRuntime();
      this.notice = { key: "skins.noticeRemoved" };
      this.error = null;
      this.errorKey = null;
    } catch (error) {
      this.error = toApiError(error).message;
      this.errorKey = null;
    }
  }

  async installZip(): Promise<void> {
    await this.#install(() => skinsApi.installZipDialog());
  }

  async installDirectory(): Promise<void> {
    await this.#install(() => skinsApi.installDirectoryDialog());
  }

  async #install(action: () => Promise<InstallResult>): Promise<void> {
    try {
      const result = await action();
      if (result.canceled) {
        this.notice = { key: "skins.noticeInstallCanceled" };
        return;
      }
      this.catalog = normalizeCatalog(result.catalog);
      this.notice = {
        key: "skins.noticeInstalled",
        params: { name: result.skin.name, version: result.skin.version },
      };
      this.error = null;
      this.errorKey = null;
    } catch (error) {
      this.error = toApiError(error).message;
      this.errorKey = null;
    }
  }

  reportRuntimeReady(id: string, theme: string): void {
    if (!id || this.runtime?.builtin) return;
    void skinsApi.reportRuntimeReady(id, theme).catch((error: unknown) => {
      this.error = toApiError(error).message;
      this.errorKey = null;
    });
  }

  async reportRuntimeFailure(id: string, reason: string): Promise<void> {
    if (this.#failedRuntime === id) return;
    this.#failedRuntime = id;
    try {
      this.catalog = normalizeCatalog(await skinsApi.reportRuntimeFailure(id, reason));
      await this.#reloadRuntime();
      this.error = null;
      this.errorKey = "skins.errorSafeMode";
    } catch (error) {
      this.error = toApiError(error).message;
      this.errorKey = null;
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
    this.errorKey = null;
  }

  dismissNotice(): void {
    this.notice = null;
  }
}

export const skins = new SkinsStore();
