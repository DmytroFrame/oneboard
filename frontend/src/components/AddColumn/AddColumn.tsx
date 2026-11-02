import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/Button/Button";
import { cx } from "@/lib/cx";
import s from "./AddColumn.module.scss";

/** The last slot on a board: a dashed button that opens into a "new column" form. */
export function AddColumn({ onAdd }: { onAdd: (name: string) => Promise<void> }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    setBusy(true);
    try {
      await onAdd(name.trim());
      setName("");
      setOpen(false);
    } finally {
      setBusy(false);
    }
  }

  if (!open) {
    return (
      <button type="button" className={s.add} onClick={() => setOpen(true)}>
        {t("column.addColumn")}
      </button>
    );
  }
  return (
    <form className={cx(s.add, s.open)} onSubmit={submit}>
      <input
        value={name}
        onChange={(e) => setName(e.target.value)}
        onKeyDown={(e) => e.key === "Escape" && setOpen(false)}
        placeholder={t("column.namePlaceholder")}
        maxLength={60}
        autoFocus
      />
      <div className={s.actions}>
        <Button type="button" size="sm" onClick={() => setOpen(false)}>
          {t("common.cancel")}
        </Button>
        <Button variant="primary" size="sm" disabled={busy || !name.trim()}>
          {t("common.add")}
        </Button>
      </div>
    </form>
  );
}
