import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/Button/Button";
import { Field } from "@/components/ui/Field/Field";
import { FormError } from "@/components/ui/FormError/FormError";
import { Modal, ModalActions, ModalContent } from "@/components/ui/Modal/Modal";
import { Muted } from "@/components/ui/Muted/Muted";
import { useAuth } from "@/context/auth";
import { peopleApi } from "@/lib/api/people";
import type { CompanyRole } from "@/types";

interface Props {
  onClose: () => void;
  onCreated: () => void;
  /** Fixed role (candidates are added from their own page). */
  role?: CompanyRole;
}

/** HR adds someone: an account with a generated password sent by email. */
export function AddPersonModal({ onClose, onCreated, role: fixedRole }: Props) {
  const { t } = useTranslation();
  const { user } = useAuth();
  const roles = user?.assignableRoles ?? [];
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<CompanyRole>(fixedRole ?? (roles.includes("EMPLOYEE") ? "EMPLOYEE" : (roles[0] ?? "EMPLOYEE")));
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await peopleApi.create(name, email, role);
      onCreated();
      onClose();
    } catch (err) {
      setError((err as Error).message);
      setBusy(false);
    }
  }

  return (
    <Modal title={t("people.addPersonTitle")} onClose={onClose}>
      <ModalContent onSubmit={submit}>
        <Field label={t("people.nameLatin")}>
          <input value={name} onChange={(e) => setName(e.target.value)} maxLength={100} required autoFocus />
        </Field>
        <Field label={t("common.email")}>
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
        </Field>
        {!fixedRole && (
          <Field label={t("people.colRole")}>
            <select value={role} onChange={(e) => setRole(e.target.value as CompanyRole)}>
              {roles.map((r) => (
                <option key={r} value={r}>
                  {t(`companyRoles.${r}`)}
                </option>
              ))}
            </select>
          </Field>
        )}
        <Muted as="p">{t("people.addPersonHint")}</Muted>
        {error && <FormError>{error}</FormError>}
        <ModalActions>
          <Button type="button" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button variant="primary" disabled={busy}>
            {t("common.create")}
          </Button>
        </ModalActions>
      </ModalContent>
    </Modal>
  );
}
