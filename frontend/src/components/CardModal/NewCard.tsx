import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { DescriptionEditor } from "@/components/DescriptionEditor/DescriptionEditor";
import { Button } from "@/components/ui/Button/Button";
import { FieldLabel } from "@/components/ui/FieldLabel/FieldLabel";
import { FormError } from "@/components/ui/FormError/FormError";
import { ModalDialog, ModalFooter } from "@/components/ui/Modal/Modal";
import { Muted } from "@/components/ui/Muted/Muted";
import { Spacer } from "@/components/ui/Spacer/Spacer";
import type { Priority } from "@/types";
import type { CardModalProps } from "./CardModal";
import { CardModalHeader } from "./CardModalHeader";
import { CardSidebar } from "./CardSidebar";
import s from "./CardModal.module.scss";

/** A new card: a plain form with a local (non-shared) editor. */
export function NewCard({ initialColumnId, columns, users, currentUser, onClose, onCreate }: CardModalProps) {
  const { t } = useTranslation();
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [columnId, setColumnId] = useState(initialColumnId);
  const [priority, setPriority] = useState<Priority>("medium");
  const [assigneeId, setAssigneeId] = useState<number | null>(currentUser.id);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await onCreate({ title, description, priority, assigneeId }, columnId);
      onClose();
    } catch (err) {
      setError((err as Error).message);
      setBusy(false);
    }
  }

  return (
    <ModalDialog onSubmit={submit}>
      <CardModalHeader title={t("card.newTask")} peers={[]} onClose={onClose} />
      <div className={s.body}>
        <div className={s.main}>
          <input className={s.titleInput} value={title} onChange={(e) => setTitle(e.target.value)} placeholder={t("card.titlePlaceholder")} maxLength={200} required autoFocus />
          <FieldLabel className={s.mainLabel}>{t("card.description")}</FieldLabel>
          <DescriptionEditor user={{ name: currentUser.name, color: currentUser.color }} initialMarkdown="" onChange={setDescription} />
          {error && <FormError>{error}</FormError>}
          <Muted as="p" className={s.attachmentsHint}>
            {t("attachments.saveFirst")}
          </Muted>
        </div>
        <CardSidebar
          columns={columns}
          users={users}
          columnId={columnId}
          priority={priority}
          assigneeId={assigneeId}
          author={currentUser}
          card={null}
          onColumn={setColumnId}
          onPriority={setPriority}
          onAssignee={setAssigneeId}
        />
      </div>
      <ModalFooter>
        <Spacer />
        <Button type="button" onClick={onClose}>
          {t("common.cancel")}
        </Button>
        <Button variant="primary" disabled={busy}>
          {t("common.create")}
        </Button>
      </ModalFooter>
    </ModalDialog>
  );
}
