import { useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/Button/Button";
import { FormError } from "@/components/ui/FormError/FormError";
import { IconButton } from "@/components/ui/IconButton/IconButton";
import { InlineForm } from "@/components/ui/InlineForm/InlineForm";
import { Modal, ModalContent } from "@/components/ui/Modal/Modal";
import { Tabs } from "@/components/ui/Tabs/Tabs";
import { peopleApi } from "@/lib/api/people";
import type { DirectoryItem, DirectoryKind } from "@/types/people";
import s from "./DirectoriesModal.module.scss";

const KINDS: DirectoryKind[] = ["specialization", "tech_stack", "candidate_channel"];

/** HR edits the reference lists: specializations, tech stacks, candidate channels. */
export function DirectoriesModal({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation();
  const [kind, setKind] = useState<DirectoryKind>("specialization");
  const [items, setItems] = useState<DirectoryItem[]>([]);
  const [name, setName] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    peopleApi.directory(kind).then(setItems).catch((err) => setError(err.message));
  }, [kind]);

  async function run(action: () => Promise<void>) {
    setError("");
    try {
      await action();
    } catch (err) {
      setError((err as Error).message);
    }
  }

  const add = (e: FormEvent) => {
    e.preventDefault();
    run(async () => {
      const item = await peopleApi.addDirectoryItem(kind, name);
      setItems((list) => [...list, item].sort((a, b) => a.name.localeCompare(b.name)));
      setName("");
    });
  };

  const rename = (item: DirectoryItem, next: string) =>
    next.trim() && next !== item.name &&
    run(async () => {
      const saved = await peopleApi.renameDirectoryItem(item.id, next);
      setItems((list) => list.map((i) => (i.id === saved.id ? saved : i)));
    });

  const remove = (item: DirectoryItem) =>
    run(async () => {
      await peopleApi.deleteDirectoryItem(item.id);
      setItems((list) => list.filter((i) => i.id !== item.id));
    });

  return (
    <Modal title={t("people.directoriesTitle")} onClose={onClose}>
      <ModalContent className={s.content}>
        <Tabs tabs={KINDS.map((k) => ({ id: k, label: t(`people.dirKinds.${k}`) }))} value={kind} onChange={setKind} />
        <ul className={s.list}>
          {items.map((item) => (
            <li key={item.id}>
              <input defaultValue={item.name} maxLength={100} onBlur={(e) => rename(item, e.target.value)} aria-label={item.name} />
              <IconButton size="sm" aria-label={t("common.delete")} onClick={() => remove(item)}>
                ✕
              </IconButton>
            </li>
          ))}
        </ul>
        <InlineForm onSubmit={add}>
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder={t("people.newItem")} maxLength={100} required />
          <Button variant="primary">{t("common.add")}</Button>
        </InlineForm>
        {error && <FormError>{error}</FormError>}
      </ModalContent>
    </Modal>
  );
}
