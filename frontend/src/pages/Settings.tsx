import { useEffect, useRef, useState, type ChangeEvent } from "react";
import { useTranslation } from "react-i18next";
import { api } from "../api";
import { useAuth } from "../auth";
import { Avatar } from "../components/Avatar";
import { detectLocale, LOCALES } from "../i18n";
import { usePreferences } from "../preferences";
import { isIOS, promptInstall, useInstallState } from "../pwa";
import type { LanguagePref, ThemePref } from "../types";

const AVATAR_SIZE = 256;
const MAX_SOURCE_BYTES = 20 << 20;

/** Center-crops the image to a square and scales it down, so uploads stay small whatever the camera produced. */
async function toSquareAvatar(file: File): Promise<Blob> {
  const bitmap = await createImageBitmap(file);
  const side = Math.min(bitmap.width, bitmap.height);
  const canvas = document.createElement("canvas");
  canvas.width = canvas.height = Math.min(AVATAR_SIZE, side);
  const ctx = canvas.getContext("2d")!;
  ctx.imageSmoothingQuality = "high";
  ctx.drawImage(bitmap, (bitmap.width - side) / 2, (bitmap.height - side) / 2, side, side, 0, 0, canvas.width, canvas.height);
  bitmap.close();
  const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, "image/webp", 0.9));
  // Browsers without WebP encoding fall back to PNG.
  return blob?.type === "image/webp" ? blob : new Promise((resolve) => canvas.toBlob((b) => resolve(b!), "image/png"));
}

export default function SettingsPage() {
  const { t } = useTranslation();
  return (
    <div className="settings-page">
      <div className="settings-head">
        <h1>{t("settings.title")}</h1>
        <p className="muted">{t("settings.subtitle")}</p>
      </div>
      <ProfileSection />
      <PreferencesSection />
      <InstallSection />
    </div>
  );
}

/** Installing OneBoard as an app (PWA): a button where the browser allows it, instructions on iOS. */
function InstallSection() {
  const { t } = useTranslation();
  const { canInstall, installed } = useInstallState();
  if (!installed && !canInstall && !isIOS) return null; // browser can't install (or already offered elsewhere)
  return (
    <section className="settings-card install-card">
      <img src="/pwa-192x192.png" alt="" width={56} height={56} />
      <div className="install-text">
        <h2>{t("pwa.title")}</h2>
        <p className="muted">{installed ? t("pwa.installed") : isIOS && !canInstall ? t("pwa.iosHint") : t("pwa.text")}</p>
      </div>
      {canInstall && !installed && (
        <button className="btn btn-primary" onClick={() => void promptInstall()}>
          {t("pwa.install")}
        </button>
      )}
    </section>
  );
}

function ProfileSection() {
  const { t } = useTranslation();
  const { user, setUser } = useAuth();
  const inputRef = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function run(action: () => Promise<void>) {
    setBusy(true);
    setError("");
    try {
      await action();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  function onFile(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    e.target.value = ""; // allow picking the same file again
    if (!file) return;
    if (!file.type.startsWith("image/")) return setError(t("settings.chooseImage"));
    if (file.size > MAX_SOURCE_BYTES) return setError(t("settings.fileTooBig"));
    run(async () => {
      let image: Blob;
      try {
        image = await toSquareAvatar(file);
      } catch {
        throw new Error(t("settings.cannotRead"));
      }
      setUser(await api.uploadAvatar(image));
    });
  }

  if (!user) return null;
  return (
    <section className="settings-card">
      <h2>{t("settings.profile")}</h2>
      <div className="profile-row">
        <div className={`profile-avatar${busy ? " profile-avatar--busy" : ""}`}>
          <Avatar user={user} size={88} />
          {busy && <span className="spinner" />}
        </div>
        <div className="profile-info">
          <div className="profile-name">{user.name}</div>
          <div className="profile-email">{user.email}</div>
          <input ref={inputRef} type="file" accept="image/png,image/jpeg,image/webp,image/gif" hidden onChange={onFile} />
          <div className="profile-actions">
            <button className="btn btn-primary btn-sm" disabled={busy} onClick={() => inputRef.current?.click()}>
              {user.avatarUrl ? t("settings.changePhoto") : t("settings.uploadPhoto")}
            </button>
            {user.avatarUrl && (
              <button className="btn btn-ghost btn-sm" disabled={busy} onClick={() => run(async () => setUser(await api.removeAvatar()))}>
                {t("settings.removePhoto")}
              </button>
            )}
          </div>
          <p className="profile-hint">{t("settings.photoHint")}</p>
        </div>
      </div>
      {error && <div className="form-error">{error}</div>}
    </section>
  );
}

const THEMES: { id: ThemePref; label: "settings.themeAuto" | "settings.themeLight" | "settings.themeDark" }[] = [
  { id: "auto", label: "settings.themeAuto" },
  { id: "light", label: "settings.themeLight" },
  { id: "dark", label: "settings.themeDark" },
];

function PreferencesSection() {
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
    <section className="settings-card">
      <div className="settings-card-head">
        <h2>{t("settings.language")}</h2>
        <span className={`saved-badge${status === "saved" ? " saved-badge--on" : ""}`} aria-live="polite">
          ✓ {t("settings.saved")}
        </span>
      </div>
      <div className="option-grid" role="radiogroup" aria-label={t("settings.language")}>
        {languages.map((l) => (
          <button
            key={l.id}
            type="button"
            role="radio"
            aria-checked={settings.language === l.id}
            className={`option${settings.language === l.id ? " option--selected" : ""}`}
            onClick={() => save({ language: l.id })}
          >
            <span className="option-code">{l.id === "auto" ? "A" : l.id.toUpperCase()}</span>
            <span>{l.label}</span>
          </button>
        ))}
      </div>
      <p className="muted settings-hint">{t("settings.languageHint")}</p>

      <h2 className="settings-subhead">{t("settings.theme")}</h2>
      <div className="theme-grid" role="radiogroup" aria-label={t("settings.theme")}>
        {THEMES.map((th) => (
          <button
            key={th.id}
            type="button"
            role="radio"
            aria-checked={settings.theme === th.id}
            className={`theme-option${settings.theme === th.id ? " option--selected" : ""}`}
            onClick={() => save({ theme: th.id })}
          >
            <span className={`theme-swatch theme-swatch--${th.id}`} aria-hidden="true">
              <span />
              <span />
              <span />
            </span>
            <span>{t(th.label)}</span>
          </button>
        ))}
      </div>
      <p className="muted settings-hint">{t("settings.themeHint")}</p>
      {error && <div className="form-error">{error}</div>}
    </section>
  );
}
