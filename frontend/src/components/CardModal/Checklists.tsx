import { useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/Button/Button";
import { FieldLabel } from "@/components/ui/FieldLabel/FieldLabel";
import { IconButton } from "@/components/ui/IconButton/IconButton";
import { InlineForm } from "@/components/ui/InlineForm/InlineForm";
import { tasksApi } from "@/lib/api/tasks";
import type { Checklist } from "@/types/tasks";
import s from "./CardModal.module.scss";

/** A task's checklists: progress, items to tick, collapsing. */
export function Checklists({ cardId, onChanged, onError }: { cardId: number; onChanged: () => void; onError: (m: string) => void }) {
  const { t } = useTranslation();
  const [lists, setLists] = useState<Checklist[]>([]);
  const [title, setTitle] = useState("");
  const [adding, setAdding] = useState(false);

  const load = () => tasksApi.checklists(cardId).then(setLists).catch((err) => onError(err.message));
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => void load(), [cardId]);

  async function run(action: () => Promise<unknown>) {
    try {
      await action();
      await load();
      onChanged();
    } catch (err) {
      onError((err as Error).message);
    }
  }

  const create = (e: FormEvent) => {
    e.preventDefault();
    run(() => tasksApi.createChecklist(cardId, title)).then(() => (setTitle(""), setAdding(false)));
  };

  return (
    <section className={s.block}>
      <div className={s.blockHead}>
        <FieldLabel>{t("tasks.checklists")}</FieldLabel>
        <Button type="button" size="sm" onClick={() => setAdding((v) => !v)}>
          {t("tasks.newChecklist")}
        </Button>
      </div>
      {adding && (
        <InlineForm onSubmit={create}>
          <input value={title} onChange={(e) => setTitle(e.target.value)} placeholder={t("tasks.checklistTitle")} maxLength={500} required autoFocus />
          <Button variant="primary" size="sm">
            {t("common.add")}
          </Button>
        </InlineForm>
      )}
      {lists.map((list) => (
        <ChecklistView key={list.id} list={list} run={run} />
      ))}
    </section>
  );
}

function ChecklistView({ list, run }: { list: Checklist; run: (a: () => Promise<unknown>) => Promise<void> }) {
  const { t } = useTranslation();
  const [text, setText] = useState("");
  const done = list.items.filter((i) => i.done).length;
  const pct = list.items.length ? Math.round((done / list.items.length) * 100) : 0;

  return (
    <div className={s.checklist}>
      <div className={s.checklistHead}>
        <button type="button" className={s.collapse} aria-expanded={!list.collapsed} onClick={() => run(() => tasksApi.updateChecklist(list.id, list.title, !list.collapsed))}>
          {list.collapsed ? "▸" : "▾"} {list.title}
        </button>
        <span className={s.progressText}>{t("tasks.progress", { done, total: list.items.length })}</span>
        <IconButton size="sm" aria-label={t("tasks.deleteChecklist")} onClick={() => run(() => tasksApi.deleteChecklist(list.id))}>
          ✕
        </IconButton>
      </div>
      <div className={s.progress}>
        <span style={{ width: `${pct}%` }} />
      </div>
      {!list.collapsed && (
        <>
          <ul className={s.items}>
            {list.items.map((it) => (
              <li key={it.id}>
                <label>
                  <input type="checkbox" checked={it.done} onChange={(e) => run(() => tasksApi.updateItem(it.id, it.text, e.target.checked))} />
                  <span className={it.done ? s.itemDone : undefined}>{it.text}</span>
                </label>
                <IconButton size="sm" aria-label={t("common.delete")} onClick={() => run(() => tasksApi.deleteItem(it.id))}>
                  ✕
                </IconButton>
              </li>
            ))}
          </ul>
          <InlineForm
            onSubmit={(e) => {
              e.preventDefault();
              run(() => tasksApi.addItem(list.id, text)).then(() => setText(""));
            }}
          >
            <input value={text} onChange={(e) => setText(e.target.value)} placeholder={t("tasks.addItem")} maxLength={500} required />
            <Button size="sm">{t("common.add")}</Button>
          </InlineForm>
        </>
      )}
    </div>
  );
}
