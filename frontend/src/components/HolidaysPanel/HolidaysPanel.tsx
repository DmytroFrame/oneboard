import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/Button/Button";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog/ConfirmDialog";
import { FormError } from "@/components/ui/FormError/FormError";
import { IconButton } from "@/components/ui/IconButton/IconButton";
import { Muted } from "@/components/ui/Muted/Muted";
import { Panel } from "@/components/ui/Panel/Panel";
import { useAuth } from "@/context/auth";
import { timeApi } from "@/lib/api/time";
import { can } from "@/lib/companyRoles";
import { formatDate } from "@/lib/format";
import type { Holiday } from "@/types/time";
import s from "./HolidaysPanel.module.scss";

type Draft = Omit<Holiday, "id"> & { id?: number };
const blank = (): Draft => ({ day: "", nameLatin: "", nameCyrillic: "" });

/** This year's public holidays (they are days off in time tracking); HR keeps the list. */
export function HolidaysPanel() {
  const { t, i18n } = useTranslation();
  const { user } = useAuth();
  const year = new Date().getFullYear();
  const [items, setItems] = useState<Holiday[]>([]);
  const [draft, setDraft] = useState<Draft | null>(null);
  const [removing, setRemoving] = useState<Holiday | null>(null);
  const [error, setError] = useState("");
  const editable = can(user, "editHolidays");
  const cyrillic = i18n.language === "uk";

  const load = () => timeApi.holidays(year).then(setItems).catch((err) => setError(err.message));
  useEffect(() => void load(), []); // eslint-disable-line react-hooks/exhaustive-deps

  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await timeApi.saveHoliday(draft!);
      setDraft(null);
      setError("");
      load();
    } catch (err) {
      setError((err as Error).message);
    }
  };
  const name = (h: Holiday) => (cyrillic ? h.nameCyrillic || h.nameLatin : h.nameLatin || h.nameCyrillic);

  return (
    <Panel compact>
      <div className={s.head}>
        <h2>{t("holidays.title")}</h2>
        {editable && !draft && (
          <Button size="sm" onClick={() => setDraft(blank())}>
            {t("holidays.add")}
          </Button>
        )}
      </div>
      {draft && (
        <form className={s.form} onSubmit={save}>
          <input type="date" required value={draft.day} onChange={(e) => setDraft({ ...draft, day: e.target.value })} aria-label={t("holidays.date")} />
          <input required placeholder={t("holidays.nameLatin")} value={draft.nameLatin} onChange={(e) => setDraft({ ...draft, nameLatin: e.target.value })} />
          <input placeholder={t("holidays.nameCyrillic")} value={draft.nameCyrillic} onChange={(e) => setDraft({ ...draft, nameCyrillic: e.target.value })} />
          <div className={s.actions}>
            <Button size="sm" variant="primary" type="submit">
              {t("common.save")}
            </Button>
            <Button size="sm" onClick={() => setDraft(null)}>
              {t("common.cancel")}
            </Button>
          </div>
        </form>
      )}
      {error && <FormError>{error}</FormError>}
      {items.length === 0 && <Muted as="p" className={s.flush}>{t("holidays.empty")}</Muted>}
      <ul className={s.list}>
        {items.map((h) => (
          <li key={h.id} className={s.item}>
            <span className={s.day}>{formatDate(h.day, i18n.language, { day: "numeric", month: "short" })}</span>
            <span className={s.name}>{name(h)}</span>
            {editable && (
              <span className={s.tools}>
                <IconButton size="sm" aria-label={t("common.edit")} onClick={() => setDraft(h)}>
                  ✎
                </IconButton>
                <IconButton size="sm" aria-label={t("common.delete")} onClick={() => setRemoving(h)}>
                  ×
                </IconButton>
              </span>
            )}
          </li>
        ))}
      </ul>
      {removing && (
        <ConfirmDialog
          title={t("holidays.deleteConfirm", { name: name(removing) })}
          confirmLabel={t("common.delete")}
          danger
          onClose={() => setRemoving(null)}
          onConfirm={async () => {
            await timeApi.deleteHoliday(removing.id);
            setRemoving(null);
            load();
          }}
        />
      )}
    </Panel>
  );
}
