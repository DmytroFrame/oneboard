import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { FormError } from "@/components/ui/FormError/FormError";
import { Muted } from "@/components/ui/Muted/Muted";
import { Panel } from "@/components/ui/Panel/Panel";
import { usePreferences } from "@/context/preferences";
import { detectLocale, LOCALES } from "@/i18n";
import { cx } from "@/lib/cx";
import type { LanguagePref, ThemePref } from "@/types";
import s from "./PreferencesSettings.module.scss";

const THEMES: { id: ThemePref; label: "settings.themeAuto" | "settings.themeLight" | "settings.themeDark" }[] = [
  { id: "auto", label: "settings.themeAuto" },
  { id: "light", label: "settings.themeLight" },
  { id: "dark", label: "settings.themeDark" },
];

/** Language and theme pickers; saved to the account as soon as one is chosen. */
export function PreferencesSettings() {
  const { t } = useTranslation();
  const { settings, update } = usePreferences();
  const [status, setStatus] = useState<"idle" | "saved">("idle");
  const [error, setError] = useState("");

  useEffect(() => {
    if (status !== "saved") return;
    const timer = setTimeout(() => setStatus("idle"), 1800);
    return () => clearTimeout(timer);
  }, [status]);

  async function save(patch: { language?: LanguagePref; theme?: ThemePref }) {
    setError("");
    try {
      await update(patch);
      setStatus("saved");
    } catch (err) {
      setError((err as Error).message);
    }
  }

  const languages: { id: LanguagePref; label: string }[] = [
    { id: "auto", label: t("settings.languageAutoDetected", { language: t(`languageNames.${detectLocale()}`) }) },
    ...LOCALES.map((l) => ({ id: l, label: t(`languageNames.${l}`) })),
  ];

  return (
    <Panel>
      <div className={s.head}>
        <h2>{t("settings.language")}</h2>
        <span className={cx(s.saved, status === "saved" && s.savedOn)} aria-live="polite">
          ✓ {t("settings.saved")}
        </span>
      </div>
      <div className={s.optionGrid} role="radiogroup" aria-label={t("settings.language")}>
        {languages.map((l) => (
          <button
            key={l.id}
            type="button"
            role="radio"
            aria-checked={settings.language === l.id}
            className={cx(s.option, settings.language === l.id && s.selected)}
            onClick={() => save({ language: l.id })}
          >
            <span className={s.code}>{l.id === "auto" ? "A" : l.id.toUpperCase()}</span>
            <span>{l.label}</span>
          </button>
        ))}
      </div>
      <Muted as="p" className={s.hint}>
        {t("settings.languageHint")}
      </Muted>

      <h2 className={s.subhead}>{t("settings.theme")}</h2>
      <div className={s.themeGrid} role="radiogroup" aria-label={t("settings.theme")}>
        {THEMES.map((th) => (
          <button
            key={th.id}
            type="button"
            role="radio"
            aria-checked={settings.theme === th.id}
            className={cx(s.option, s.themeOption, settings.theme === th.id && s.selected)}
            onClick={() => save({ theme: th.id })}
          >
            <span className={cx(s.swatch, th.id === "dark" && s.swatchDark, th.id === "auto" && s.swatchAuto)} aria-hidden="true">
              <span />
              <span />
              <span />
            </span>
            <span>{t(th.label)}</span>
          </button>
        ))}
      </div>
      <Muted as="p" className={s.hint}>
        {t("settings.themeHint")}
      </Muted>
      {error && <FormError>{error}</FormError>}
    </Panel>
  );
}
