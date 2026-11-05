import IntlMessageFormat from "intl-messageformat";
import { en, type MessageKey } from "./locales/en";

export type { MessageKey } from "./locales/en";

export const LOCALES = ["en", "es"] as const;
export type Locale = (typeof LOCALES)[number];

export type Catalog = Record<MessageKey, string>;
export type MessageParams = Record<string, string | number>;

const LOCALE_STORAGE_KEY = "cadence.locale";

type CatalogLoader = () => Promise<Catalog>;

const defaultLoaders: Record<Locale, CatalogLoader> = {
  en: async () => en,
  es: async () => (await import("./locales/es")).es,
};

export interface I18nOptions {
  /** Overrides the base catalog. Used by tests and future plugin catalogs. */
  base?: Catalog;
  /** Overrides a locale loader. Missing locales fall back to the defaults. */
  loaders?: Partial<Record<Locale, CatalogLoader>>;
}

function isLocale(value: unknown): value is Locale {
  return typeof value === "string" && (LOCALES as readonly string[]).includes(value);
}

function readStoredLocale(): string | null {
  try {
    return globalThis.localStorage?.getItem(LOCALE_STORAGE_KEY) ?? null;
  } catch {
    return null;
  }
}

function persistLocale(locale: Locale): void {
  try {
    globalThis.localStorage?.setItem(LOCALE_STORAGE_KEY, locale);
  } catch {
    // Locale changes stay in memory when storage is unavailable.
  }
}

function setDocumentLang(locale: Locale): void {
  if (typeof document !== "undefined") {
    document.documentElement.lang = locale;
  }
}

function detectLocale(): Locale {
  if (typeof navigator !== "undefined") {
    const language = navigator.language?.toLowerCase() ?? "";
    if (language.startsWith("es")) return "es";
  }
  return "en";
}

/**
 * Locale runtime: holds the active catalog, caches ICU formatters and exposes
 * `Intl` helpers for numbers, dates and relative time.
 */
export class I18n {
  locale = $state<Locale>("en");

  #base: Catalog;
  #catalog: Catalog;
  #loaders: Record<Locale, CatalogLoader>;
  #formatters: Record<string, IntlMessageFormat> = {};
  #request = 0;
  #storageSyncStarted = false;

  constructor(options: I18nOptions = {}) {
    this.#base = options.base ?? en;
    this.#catalog = this.#base;
    this.#loaders = {
      en: async () => this.#base,
      es: defaultLoaders.es,
      ...options.loaders,
    };
  }

  /**
   * Loads the catalog (dynamic import for non-base locales), drops cached
   * formatters, persists the choice and updates `document.documentElement.lang`.
   * Calls are latest-wins: a slow loader cannot overwrite a newer selection.
   */
  async setLocale(locale: Locale): Promise<void> {
    await this.#applyLocale(locale, true);
  }

  /**
   * Applies a locale received from another webview. Matches `setLocale`'s
   * latest-wins behavior but never persists, so a storage event cannot echo
   * back a write and start a loop.
   */
  async syncLocale(locale: Locale): Promise<void> {
    await this.#applyLocale(locale, false);
  }

  /**
   * Mirrors `cadence.locale` writes made by other windows. The listener is
   * installed once per webview; `storage` events only fire in other documents,
   * and `syncLocale` does not write, so updates stay one-way.
   */
  startStorageSync(): void {
    if (this.#storageSyncStarted || typeof window === "undefined") return;
    this.#storageSyncStarted = true;
    window.addEventListener("storage", (event) => {
      if (event.key !== LOCALE_STORAGE_KEY || !event.newValue) return;
      if (!isLocale(event.newValue) || event.newValue === this.locale) return;
      void this.syncLocale(event.newValue).catch(() => {
        // Keep the current catalog when another window's locale chunk fails.
      });
    });
  }

  async #applyLocale(locale: Locale, persist: boolean): Promise<void> {
    const request = ++this.#request;
    let catalog: Catalog;
    try {
      catalog = await this.#loaders[locale]();
    } catch (error) {
      // A superseded switch must not surface a rejection from an abandoned load.
      if (request !== this.#request) return;
      throw error;
    }
    if (request !== this.#request) return;

    this.#catalog = catalog;
    this.#formatters = {};
    this.locale = locale;
    if (persist) persistLocale(locale);
    setDocumentLang(locale);
  }

  /**
   * Translates an ICU message, falling back to the base catalog and finally
   * the raw key. Reading `this.locale` makes Svelte templates track changes.
   */
  t(key: MessageKey, params?: MessageParams): string {
    const locale = this.locale;
    const pattern = this.#catalog[key] || this.#base[key] || key;
    const cacheKey = `${locale}:${key}`;
    let formatter = this.#formatters[cacheKey];
    if (!formatter) {
      formatter = new IntlMessageFormat(pattern, locale);
      this.#formatters[cacheKey] = formatter;
    }
    return String(formatter.format(params ?? {}));
  }

  formatNumber(value: number, options?: Intl.NumberFormatOptions): string {
    return new Intl.NumberFormat(this.locale, options).format(value);
  }

  formatDate(value: Date | number | string, options?: Intl.DateTimeFormatOptions): string {
    // Formatting is pure; a reactive SvelteDate adds tracking for no benefit.
    // eslint-disable-next-line svelte/prefer-svelte-reactivity
    const date = value instanceof Date ? value : new Date(value);
    return new Intl.DateTimeFormat(this.locale, options).format(date);
  }

  formatRelativeTime(
    value: number,
    unit: Intl.RelativeTimeFormatUnit,
    options?: Intl.RelativeTimeFormatOptions,
  ): string {
    return new Intl.RelativeTimeFormat(this.locale, options).format(value, unit);
  }
}

export const i18n = new I18n();

/**
 * Module-level translator. Calling it inside a Svelte template tracks the
 * active locale because it reads the reactive `i18n.locale` field.
 */
export function t(key: MessageKey, params?: MessageParams): string {
  return i18n.t(key, params);
}

/**
 * Applies the saved locale, then the browser language, defaulting to English.
 * A failing catalog loader falls back to English so startup never blocks mount.
 */
export async function initI18n(instance: I18n = i18n): Promise<Locale> {
  instance.startStorageSync();
  const stored = readStoredLocale();
  const locale = isLocale(stored) ? stored : detectLocale();
  try {
    await instance.setLocale(locale);
    return locale;
  } catch {
    try {
      await instance.setLocale("en");
    } catch {
      // The base catalog is already loaded; mounting must continue regardless.
      setDocumentLang("en");
    }
    return "en";
  }
}

export { LOCALE_STORAGE_KEY };
