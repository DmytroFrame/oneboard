import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/Button/Button";
import { Field } from "@/components/ui/Field/Field";
import { FormError } from "@/components/ui/FormError/FormError";
import { Modal, ModalActions, ModalContent } from "@/components/ui/Modal/Modal";
import { Muted } from "@/components/ui/Muted/Muted";
import { api } from "@/lib/api";
import type { Organization } from "@/types";

interface Props {
  onClose: () => void;
  onCreated: (o: Organization) => Promise<void>;
}

export function CreateOrgModal({ onClose, onCreated }: Props) {
  const { t } = useTranslation();
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await onCreated(await api.createOrganization(name));
      onClose();
    } catch (err) {
      setError((err as Error).message);
      setBusy(false);
    }
  }

  return (
    <Modal title={t("org.newTitle")} onClose={onClose}>
      <ModalContent onSubmit={submit}>
        <Field label={t("common.name")}>
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder={t("org.namePlaceholder")} maxLength={100} required autoFocus />
        </Field>
        <Muted as="p">{t("org.rolesHint")}</Muted>
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
