import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/Button/Button";
import { Field } from "@/components/ui/Field/Field";
import { FormError } from "@/components/ui/FormError/FormError";
import { Modal, ModalActions, ModalContent } from "@/components/ui/Modal/Modal";
import { Muted } from "@/components/ui/Muted/Muted";
import { api } from "@/lib/api";
import type { Organization, Project } from "@/types";

/** Suggests a project key from its name: first letters of words, latin only. */
function suggestKey(name: string) {
  const words = name.toUpperCase().match(/[A-Z][A-Z0-9]*/g) ?? [];
  const key = words.length > 1 ? words.map((w) => w[0]).join("") : (words[0] ?? "");
  return key.slice(0, 10);
}

interface Props {
  orgs: Organization[]; // organizations where the user may create projects
  defaultOrgId?: number;
  onClose: () => void;
  onCreated: (p: Project) => Promise<void>;
}

export function CreateProjectModal({ orgs, defaultOrgId, onClose, onCreated }: Props) {
  const { t } = useTranslation();
  const [orgId, setOrgId] = useState(defaultOrgId ?? orgs[0]?.id ?? 0);
  const [name, setName] = useState("");
  const [key, setKey] = useState("");
  const [keyTouched, setKeyTouched] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const p = await api.createProject(orgId, name, key);
      await onCreated(p);
      onClose();
    } catch (err) {
      setError((err as Error).message);
      setBusy(false);
    }
  }

  return (
    <Modal title={t("project.newTitle")} onClose={onClose}>
      <ModalContent onSubmit={submit}>
        {orgs.length === 0 && <Muted as="p">{t("projectExtra.noOrgToCreate")}</Muted>}
        <Field label={t("projectExtra.organization")}>
          <select value={orgId} onChange={(e) => setOrgId(Number(e.target.value))} required>
            {orgs.map((o) => (
              <option key={o.id} value={o.id}>
                {o.name}
              </option>
            ))}
          </select>
        </Field>
        <Field label={t("common.name")}>
          <input
            value={name}
            onChange={(e) => {
              setName(e.target.value);
              if (!keyTouched) setKey(suggestKey(e.target.value));
            }}
            placeholder={t("project.namePlaceholder")}
            maxLength={100}
            required
            autoFocus
          />
        </Field>
        <Field label={t("project.key")} hint={t("project.keyHint", { key: key || "KEY" })}>
          <input
            value={key}
            onChange={(e) => {
              setKeyTouched(true);
              setKey(e.target.value.toUpperCase().replace(/[^A-Z0-9]/g, ""));
            }}
            placeholder="WEB"
            minLength={2}
            maxLength={10}
            pattern="[A-Z][A-Z0-9]{1,9}"
            required
          />
        </Field>
        {error && <FormError>{error}</FormError>}
        <ModalActions>
          <Button type="button" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button variant="primary" disabled={busy || orgs.length === 0}>
            {t("common.create")}
          </Button>
        </ModalActions>
      </ModalContent>
    </Modal>
  );
}
