import { useTranslation } from "react-i18next";
import { LOCALES } from "../i18n";
import { usePreferences } from "../preferences";
import type { LanguagePref } from "../types";

/** Compact language picker for pages without the settings screen (landing, sign-in). */
export function LanguageSelect() {
  const { t } = useTranslation();
  const { settings, update } = usePreferences();
  return (
    <select
      className="lang-select"
      value={settings.language}
      onChange={(e) => update({ language: e.target.value as LanguagePref }).catch(() => {})}
      aria-label={t("settings.language")}
    >
      <option value="auto">{t("settings.languageAuto")}</option>
      {LOCALES.map((l) => (
        <option key={l} value={l}>
          {t(`languageNames.${l}`)}
        </option>
      ))}
    </select>
  );
}
