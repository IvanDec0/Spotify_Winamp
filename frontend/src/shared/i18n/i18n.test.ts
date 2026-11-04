import { render, screen, waitFor } from "@testing-library/svelte";
import IntlMessageFormat from "intl-messageformat";
import { beforeEach, describe, expect, it } from "vitest";
import {
  en,
  I18n,
  i18n,
  initI18n,
  LOCALE_STORAGE_KEY,
  t,
  type Catalog,
  type MessageKey,
} from "./index";
import { es } from "./locales/es";
import Fixture from "./test-fixture.svelte";

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

  it("parses every message as valid ICU", () => {
    for (const [key, value] of Object.entries(en)) {
      expect(() => new IntlMessageFormat(value, "en"), key).not.toThrow();
    }
    for (const [key, value] of Object.entries(es)) {
      expect(() => new IntlMessageFormat(value, "es"), key).not.toThrow();
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

  it("clears cached formatters when the same locale is reloaded", async () => {
    let version = 0;
    const instance = new I18n({
      loaders: {
        es: async () => ({
          ...es,
          "ui.close": version++ === 0 ? "Cerrar uno" : "Cerrar dos",
        }),
      },
    });

    await instance.setLocale("es");
    expect(instance.t("ui.close")).toBe("Cerrar uno");

    await instance.setLocale("es");
    expect(instance.t("ui.close")).toBe("Cerrar dos");
  });

  it("applies the latest locale switch when loaders resolve out of order", async () => {
    let resolveEs!: (catalog: Catalog) => void;
    let resolveEn!: (catalog: Catalog) => void;
    const pendingEs = new Promise<Catalog>((resolve) => {
      resolveEs = resolve;
    });
    const pendingEn = new Promise<Catalog>((resolve) => {
      resolveEn = resolve;
    });
    const instance = new I18n({
      loaders: { es: () => pendingEs, en: () => pendingEn },
    });

    const first = instance.setLocale("es");
    const second = instance.setLocale("en");

    resolveEn(en);
    await second;
    expect(instance.locale).toBe("en");
    expect(localStorage.getItem(LOCALE_STORAGE_KEY)).toBe("en");

    resolveEs(es);
    await first;
    expect(instance.locale).toBe("en");
    expect(localStorage.getItem(LOCALE_STORAGE_KEY)).toBe("en");
    expect(document.documentElement.lang).toBe("en");
  });

  it("reacts to locale changes in Svelte templates", async () => {
    render(Fixture);
    expect(screen.getByText("Close")).not.toBeNull();

    await i18n.setLocale("es");
    await waitFor(() => expect(screen.getByText("Cerrar")).not.toBeNull());
  });

  it("falls back to English when the stored locale loader rejects", async () => {
    localStorage.setItem(LOCALE_STORAGE_KEY, "es");
    const instance = new I18n({
      loaders: {
        es: async () => {
          throw new Error("catalog chunk failed");
        },
      },
    });

    await expect(initI18n(instance)).resolves.toBe("en");
    expect(instance.locale).toBe("en");
    expect(document.documentElement.lang).toBe("en");
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
