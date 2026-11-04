import { beforeEach, describe, expect, it } from "vitest";
import {
  en,
  es,
  I18n,
  i18n,
  initI18n,
  LOCALE_STORAGE_KEY,
  t,
  type Catalog,
  type MessageKey,
} from "./index";

const originalLanguage = navigator.language;

function setBrowserLanguage(language: string): void {
  Object.defineProperty(window.navigator, "language", {
    configurable: true,
    value: language,
  });
}

beforeEach(async () => {
  setBrowserLanguage(originalLanguage);
  localStorage.clear();
  await i18n.setLocale("en");
  document.documentElement.lang = "";
});

describe("i18n catalogs", () => {
  it("has the same keys in both locales and no empty values", () => {
    expect(Object.keys(es).sort()).toEqual(Object.keys(en).sort());
    for (const value of [...Object.values(en), ...Object.values(es)]) {
      expect(value.trim()).not.toBe("");
    }
  });
});

describe("i18n runtime", () => {
  it("interpolates ICU placeholders", () => {
    const custom = new I18n({
      base: { ...en, "ui.retry": "Retry {what}" },
    });
    expect(custom.t("ui.retry", { what: "sync" })).toBe("Retry sync");
  });

  it("formats plurals per locale", async () => {
    expect(t("ui.items", { count: 1 })).toBe("1 item");
    expect(t("ui.items", { count: 2 })).toBe("2 items");

    await i18n.setLocale("es");
    expect(t("ui.items", { count: 1 })).toBe("1 elemento");
    expect(t("ui.items", { count: 2 })).toBe("2 elementos");
  });

  it("switches locale, updates the document language and persists", async () => {
    await i18n.setLocale("es");

    expect(i18n.locale).toBe("es");
    expect(document.documentElement.lang).toBe("es");
    expect(localStorage.getItem(LOCALE_STORAGE_KEY)).toBe("es");
    expect(t("ui.close")).toBe("Cerrar");
  });

  it("invalidates cached formatters when the locale changes", async () => {
    expect(t("ui.items", { count: 2 })).toBe("2 items");
    await i18n.setLocale("es");
    expect(t("ui.items", { count: 2 })).toBe("2 elementos");
    await i18n.setLocale("en");
    expect(t("ui.items", { count: 2 })).toBe("2 items");
  });

  it("falls back to the base catalog and then to the key", async () => {
    const partial = new I18n({
      loaders: { es: async () => ({}) as Catalog },
    });
    await partial.setLocale("es");

    expect(partial.t("ui.close")).toBe(en["ui.close"]);
    expect(partial.t("missing.key" as MessageKey)).toBe("missing.key");
  });

  it("reads the stored locale before the browser language", async () => {
    setBrowserLanguage("es-AR");
    localStorage.setItem(LOCALE_STORAGE_KEY, "en");

    await expect(initI18n()).resolves.toBe("en");
    expect(i18n.locale).toBe("en");
    expect(document.documentElement.lang).toBe("en");
  });

  it("detects Spanish from the browser language and defaults elsewhere to English", async () => {
    localStorage.clear();
    setBrowserLanguage("es-AR");
    await expect(initI18n()).resolves.toBe("es");
    expect(i18n.locale).toBe("es");
    expect(document.documentElement.lang).toBe("es");

    localStorage.clear();
    setBrowserLanguage("fr-FR");
    await expect(initI18n()).resolves.toBe("en");
    expect(i18n.locale).toBe("en");
  });

  it("ignores an invalid stored locale", async () => {
    localStorage.setItem(LOCALE_STORAGE_KEY, "de");
    setBrowserLanguage("es-MX");

    await expect(initI18n()).resolves.toBe("es");
  });

  it("formats numbers, dates and relative time with the active locale", async () => {
    const number = 1234.5;
    expect(i18n.formatNumber(number)).toBe(new Intl.NumberFormat("en").format(number));

    const date = new Date(Date.UTC(2026, 10, 4, 12, 0, 0));
    const options: Intl.DateTimeFormatOptions = { timeZone: "UTC", dateStyle: "long" };
    expect(i18n.formatDate(date, options)).toBe(
      new Intl.DateTimeFormat("en", options).format(date),
    );

    expect(i18n.formatRelativeTime(-1, "day")).toBe(
      new Intl.RelativeTimeFormat("en").format(-1, "day"),
    );

    await i18n.setLocale("es");
    expect(i18n.formatRelativeTime(-1, "day")).toBe(
      new Intl.RelativeTimeFormat("es").format(-1, "day"),
    );
  });
});
