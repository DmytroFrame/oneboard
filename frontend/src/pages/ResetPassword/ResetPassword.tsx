import { useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Link, useParams } from "react-router-dom";
import { Banner } from "@/components/ui/Banner/Banner";
import { Button } from "@/components/ui/Button/Button";
import { Center } from "@/components/ui/Center/Center";
import { Field } from "@/components/ui/Field/Field";
import { Form } from "@/components/ui/Form/Form";
import { FormError } from "@/components/ui/FormError/FormError";
import { AuthLayout } from "@/layouts/AuthLayout/AuthLayout";
import { AuthNote } from "@/layouts/AuthLayout/AuthNote";
import { api } from "@/lib/api";

type State = "checking" | "valid" | "expired" | "done";

/** Sets a new password from a one-time email link. The link is checked once when the page opens. */
export default function ResetPassword() {
  const { t } = useTranslation();
  const { token = "" } = useParams();
  const [state, setState] = useState<State>("checking");
  const [password, setPassword] = useState("");
  const [repeat, setRepeat] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    api
      .checkResetToken(token)
      .then(() => setState("valid"))
      .catch(() => setState("expired"));
  }, [token]);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (password !== repeat) return setError(t("auth.passwordsDiffer"));
    setBusy(true);
    setError("");
    try {
      await api.resetPassword(token, password);
      setState("done");
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <AuthLayout title={t("auth.newPasswordTitle")}>
      {state === "checking" && <Center>{t("auth.checkingLink")}</Center>}
      {state === "expired" && <Banner>{t("auth.linkExpired")}</Banner>}
      {state === "done" && <Banner tone="success">{t("auth.passwordChanged")}</Banner>}
      {state === "valid" && (
        <Form onSubmit={onSubmit}>
          <Field label={t("auth.newPassword")} hint={t("auth.passwordHint")}>
            <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required minLength={8} maxLength={72} autoFocus />
          </Field>
          <Field label={t("auth.repeatPassword")}>
            <input type="password" value={repeat} onChange={(e) => setRepeat(e.target.value)} required />
          </Field>
          {error && <FormError>{error}</FormError>}
          <Button variant="primary" block disabled={busy}>
            {t("auth.savePassword")}
          </Button>
        </Form>
      )}
      <AuthNote>
        {state === "expired" ? <Link to="/forgot-password">{t("auth.forgotPassword")}</Link> : <Link to="/login">{t("auth.backToSignIn")}</Link>}
      </AuthNote>
    </AuthLayout>
  );
}
