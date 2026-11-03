import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { Banner } from "@/components/ui/Banner/Banner";
import { Button } from "@/components/ui/Button/Button";
import { Field } from "@/components/ui/Field/Field";
import { Form } from "@/components/ui/Form/Form";
import { FormError } from "@/components/ui/FormError/FormError";
import { Muted } from "@/components/ui/Muted/Muted";
import { AuthLayout } from "@/layouts/AuthLayout/AuthLayout";
import { AuthNote } from "@/layouts/AuthLayout/AuthNote";
import { api } from "@/lib/api";

/** Asks for a one-time password-reset link by email. */
export default function ForgotPassword() {
  const { t } = useTranslation();
  const [email, setEmail] = useState("");
  const [sent, setSent] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api.requestPasswordReset(email);
      setSent(true);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <AuthLayout title={t("auth.resetTitle")}>
      {sent ? (
        <Banner tone="success">{t("auth.linkSent")}</Banner>
      ) : (
        <Form onSubmit={onSubmit}>
          <Muted as="p">{t("auth.resetText")}</Muted>
          <Field label={t("common.email")}>
            <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required autoFocus />
          </Field>
          {error && <FormError>{error}</FormError>}
          <Button variant="primary" block disabled={busy}>
            {t("auth.sendLink")}
          </Button>
        </Form>
      )}
      <AuthNote>
        <Link to="/login">{t("auth.backToSignIn")}</Link>
      </AuthNote>
    </AuthLayout>
  );
}
