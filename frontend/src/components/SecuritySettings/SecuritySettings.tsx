import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Banner } from "@/components/ui/Banner/Banner";
import { Button } from "@/components/ui/Button/Button";
import { Field } from "@/components/ui/Field/Field";
import { Form } from "@/components/ui/Form/Form";
import { FormError } from "@/components/ui/FormError/FormError";
import { Panel } from "@/components/ui/Panel/Panel";
import { api } from "@/lib/api";
import s from "./SecuritySettings.module.scss";

/** Changing your own password. */
export function SecuritySettings() {
  const { t } = useTranslation();
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [repeat, setRepeat] = useState("");
  const [error, setError] = useState("");
  const [done, setDone] = useState(false);
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setDone(false);
    if (next !== repeat) return setError(t("auth.passwordsDiffer"));
    setBusy(true);
    setError("");
    try {
      await api.changePassword(current, next);
      setCurrent("");
      setNext("");
      setRepeat("");
      setDone(true);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Panel>
      <h2>{t("settings.security")}</h2>
      <Form onSubmit={onSubmit} className={s.form}>
        <Field label={t("settings.currentPassword")}>
          <input type="password" autoComplete="current-password" value={current} onChange={(e) => setCurrent(e.target.value)} required />
        </Field>
        <Field label={t("auth.newPassword")} hint={t("auth.passwordHint")}>
          <input type="password" autoComplete="new-password" value={next} onChange={(e) => setNext(e.target.value)} required minLength={8} maxLength={72} />
        </Field>
        <Field label={t("auth.repeatPassword")}>
          <input type="password" autoComplete="new-password" value={repeat} onChange={(e) => setRepeat(e.target.value)} required />
        </Field>
        {error && <FormError>{error}</FormError>}
        {done && <Banner tone="success">{t("settings.passwordUpdated")}</Banner>}
        <div>
          <Button variant="primary" disabled={busy}>
            {t("settings.changePassword")}
          </Button>
        </div>
      </Form>
    </Panel>
  );
}
