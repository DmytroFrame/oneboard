import { useCallback, useSyncExternalStore } from "react";
import { api } from "./api";
import { useAuth } from "./auth";
import { i18n, resolveLocale } from "./i18n";
import type { Settings, ThemePref } from "./types";

// The last applied settings are mirrored to localStorage: index.html reads the theme before first paint,
// and signed-out pages (landing, sign-in) keep the visitor's choice.
const THEME_KEY = "flowboard:theme";
const LANGUAGE_KEY = "flowboard:language";
const media = window.matchMedia("(prefers-color-scheme: dark)");

function read(key: string) {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

function write(key: string, value: string) {
  try {
    localStorage.setItem(key, value);
  } catch {
    // storage unavailable (private mode) — settings still apply for this session
  }
}

export function storedSettings(): Settings {
  const theme = read(THEME_KEY);
  const language = read(LANGUAGE_KEY);
  return {
    theme: theme === "light" || theme === "dark" ? theme : "auto",
    language: language === "en" || language === "uk" || language === "de" ? language : "auto",
  };
}

interface Snapshot {
  settings: Settings;
  resolvedTheme: "light" | "dark";
}

const resolveTheme = (t: ThemePref) => (t === "auto" ? (media.matches ? "dark" : "light") : t);

let snapshot: Snapshot = { settings: storedSettings(), resolvedTheme: resolveTheme(storedSettings().theme) };
const listeners = new Set<() => void>();

function publish(settings: Settings) {
  snapshot = { settings, resolvedTheme: resolveTheme(settings.theme) };
  document.documentElement.dataset.theme = snapshot.resolvedTheme;
  listeners.forEach((l) => l());
}

media.addEventListener("change", () => publish(snapshot.settings));

/** Applies settings to the page (theme + language) and remembers them locally. */
export function applySettings(settings: Settings) {
  write(THEME_KEY, settings.theme);
  write(LANGUAGE_KEY, settings.language);
  const locale = resolveLocale(settings.language);
  if (i18n.language !== locale) void i18n.changeLanguage(locale);
  publish(settings);
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function usePreferencesSnapshot() {
  return useSyncExternalStore(subscribe, () => snapshot);
}

/**
 * Current settings and a setter. Signed in, changes are saved to the account (optimistically applied first);
 * signed out, they only live in this browser.
 */
export function usePreferences() {
  const { user, setUser } = useAuth();
  const { settings, resolvedTheme } = usePreferencesSnapshot();

  const update = useCallback(
    async (patch: Partial<Settings>) => {
      const previous = snapshot.settings;
      const next = { ...previous, ...patch };
      applySettings(next);
      if (!user) return;
      try {
        setUser(await api.updateSettings(next));
      } catch (err) {
        applySettings(previous);
        throw err;
      }
    },
    [user, setUser],
  );

  return { settings, resolvedTheme, update };
}
