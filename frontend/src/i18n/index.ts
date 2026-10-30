import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import type { LanguagePref, Locale } from "../types";
import { de } from "./de";
import { en } from "./en";
import { uk } from "./uk";

export const LOCALES: Locale[] = ["en", "uk", "de"];

declare module "i18next" {
  interface CustomTypeOptions {
    resources: { translation: typeof en };
  }
}

/** The first browser language we support, or English. */
export function detectLocale(): Locale {
  const langs = navigator.languages?.length ? navigator.languages : [navigator.language];
  for (const l of langs) {
    const base = l.toLowerCase().split("-")[0] as Locale;
    if (LOCALES.includes(base)) return base;
  }
  return "en";
}

export function resolveLocale(pref: LanguagePref): Locale {
  return pref === "auto" ? detectLocale() : pref;
}

i18n.on("languageChanged", (lng) => {
  document.documentElement.lang = lng;
  document.title = i18n.t("meta.title");
});

export function initI18n(pref: LanguagePref) {
  void i18n.use(initReactI18next).init({
    resources: { en: { translation: en }, uk: { translation: uk }, de: { translation: de } },
    lng: resolveLocale(pref),
    fallbackLng: "en",
    interpolation: { escapeValue: false }, // React already escapes
    initAsync: false, // resources are bundled, so render translated from the first frame
  });
}

/** Translates an API error code (errors.<code>), falling back to the server's English message. */
export function translateError(code: string | undefined, fallback: string): string {
  const key = `errors.${code}`;
  return code && i18n.exists(key) ? (i18n.t as (k: string) => string)(key) : fallback;
}

export { i18n };
