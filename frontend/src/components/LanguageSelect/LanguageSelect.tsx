import { useTranslation } from "react-i18next";
import { usePreferences } from "@/context/preferences";
import { LOCALES } from "@/i18n";
import type { LanguagePref } from "@/types";
import s from "./LanguageSelect.module.scss";

/** Compact language picker for pages without the settings screen (landing, sign-in). */
export function LanguageSelect() {
  const { t } = useTranslation();
  const { settings, update } = usePreferences();
  return (
    <select
      className={s.select}
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
