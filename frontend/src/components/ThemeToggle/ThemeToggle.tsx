import { useTranslation } from "react-i18next";
import { IconButton } from "@/components/ui/IconButton/IconButton";
import { MoonIcon, SunIcon } from "@/components/ui/icons";
import { usePreferences } from "@/context/preferences";
import s from "./ThemeToggle.module.scss";

/** Quick switch between light and dark; "automatic" is chosen on the settings page. */
export function ThemeToggle() {
  const { t } = useTranslation();
  const { resolvedTheme, update } = usePreferences();
  const dark = resolvedTheme === "dark";
  const label = dark ? t("theme.toLight") : t("theme.toDark");
  return (
    <IconButton className={s.toggle} onClick={() => update({ theme: dark ? "light" : "dark" }).catch(() => {})} title={label} aria-label={label}>
      {dark ? <SunIcon /> : <MoonIcon />}
    </IconButton>
  );
}
