import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { Button } from "@/components/ui/Button/Button";
import { Field } from "@/components/ui/Field/Field";
import { Form } from "@/components/ui/Form/Form";
import { FormError } from "@/components/ui/FormError/FormError";
import { useAuth } from "@/context/auth";
import { useAfterAuth } from "@/hooks/useAfterAuth";
import { AuthLayout } from "@/layouts/AuthLayout/AuthLayout";
import { AuthNote } from "@/layouts/AuthLayout/AuthNote";

export default function Login() {
  const { t } = useTranslation();
  const { login } = useAuth();
  const done = useAfterAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await login(email, password);
      done();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <AuthLayout title={t("auth.signInTitle")}>
      <Form onSubmit={onSubmit}>
        <Field label={t("common.email")}>
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required autoFocus />
        </Field>
        <Field label={t("common.password")}>
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
        </Field>
        {error && <FormError>{error}</FormError>}
        <Button variant="primary" block disabled={busy}>
          {busy ? t("auth.signingIn") : t("auth.signIn")}
        </Button>
      </Form>
      <AuthNote>
        <Link to="/forgot-password">{t("auth.forgotPassword")}</Link>
      </AuthNote>
      <AuthNote>
        {t("auth.demoAccount")} <code>demo@example.com</code> / <code>demo1234</code>
      </AuthNote>
      <AuthNote>
        {t("auth.noAccount")} <Link to="/register">{t("auth.signUp")}</Link>
      </AuthNote>
    </AuthLayout>
  );
}
