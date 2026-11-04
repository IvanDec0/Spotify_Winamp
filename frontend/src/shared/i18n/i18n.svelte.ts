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
    const request = ++this.#request;
    const catalog = await this.#loaders[locale]();
    if (request !== this.#request) return;

    this.#catalog = catalog;
    this.#formatters = {};
    this.locale = locale;
    persistLocale(locale);
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
