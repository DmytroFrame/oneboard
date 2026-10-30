import { useTranslation } from "react-i18next";
import { usePreferences } from "./preferences";

/** Quick switch between light and dark; "automatic" is chosen on the settings page. */
export function ThemeToggle() {
  const { t } = useTranslation();
  const { resolvedTheme, update } = usePreferences();
  const dark = resolvedTheme === "dark";
  const label = dark ? t("theme.toLight") : t("theme.toDark");
  return (
    <button
      type="button"
      className="icon-btn theme-toggle"
      onClick={() => update({ theme: dark ? "light" : "dark" }).catch(() => {})}
      title={label}
      aria-label={label}
    >
      {dark ? (
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <circle cx="12" cy="12" r="4" />
          <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.41-1.41M17.66 6.34l1.41-1.41" />
        </svg>
      ) : (
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
        </svg>
      )}
    </button>
  );
}
