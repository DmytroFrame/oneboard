import { useRef, useState, type ChangeEvent } from "react";
import { useTranslation } from "react-i18next";
import { Avatar } from "@/components/Avatar/Avatar";
import { Button } from "@/components/ui/Button/Button";
import { FormError } from "@/components/ui/FormError/FormError";
import { Panel } from "@/components/ui/Panel/Panel";
import { Spinner } from "@/components/ui/Spinner/Spinner";
import { useAuth } from "@/context/auth";
import { api } from "@/lib/api";
import { toSquareAvatar } from "@/lib/avatarImage";
import { cx } from "@/lib/cx";
import s from "./ProfileSettings.module.scss";

const MAX_SOURCE_BYTES = 20 << 20;

/** Name, email and the profile photo (upload / remove). */
export function ProfileSettings() {
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
    <Panel>
      <h2>{t("settings.profile")}</h2>
      <div className={s.row}>
        <div className={cx(s.avatar, busy && s.busy)}>
          <Avatar user={user} size={88} className={s.photo} />
          {busy && <Spinner className={s.spinner} />}
        </div>
        <div>
          <div className={s.name}>{user.name}</div>
          <div className={s.email}>{user.email}</div>
          <input ref={inputRef} type="file" accept="image/png,image/jpeg,image/webp,image/gif" hidden onChange={onFile} />
          <div className={s.actions}>
            <Button variant="primary" size="sm" disabled={busy} onClick={() => inputRef.current?.click()}>
              {user.avatarUrl ? t("settings.changePhoto") : t("settings.uploadPhoto")}
            </Button>
            {user.avatarUrl && (
              <Button size="sm" disabled={busy} onClick={() => run(async () => setUser(await api.removeAvatar()))}>
                {t("settings.removePhoto")}
              </Button>
            )}
          </div>
          <p className={s.hint}>{t("settings.photoHint")}</p>
        </div>
      </div>
      {error && <FormError>{error}</FormError>}
    </Panel>
  );
}
