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

export default function Register() {
  const { t } = useTranslation();
  const { register } = useAuth();
  const done = useAfterAuth();
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await register(name, email, password);
      done();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <AuthLayout title={t("auth.signUpTitle")}>
      <Form onSubmit={onSubmit}>
        <Field label={t("common.name")}>
          <input value={name} onChange={(e) => setName(e.target.value)} required autoFocus maxLength={100} />
        </Field>
        <Field label={t("common.email")} hint={t("auth.inviteOnly")}>
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
        </Field>
        <Field label={t("common.password")} hint={t("auth.passwordHint")}>
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required minLength={8} maxLength={72} />
        </Field>
        {error && <FormError>{error}</FormError>}
        <Button variant="primary" block disabled={busy}>
          {busy ? t("auth.creating") : t("auth.createAccount")}
        </Button>
      </Form>
      <AuthNote>
        {t("auth.haveAccount")} <Link to="/login">{t("auth.signIn")}</Link>
      </AuthNote>
    </AuthLayout>
  );
}
