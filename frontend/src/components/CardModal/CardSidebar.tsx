import { useMemo, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Avatar } from "@/components/Avatar/Avatar";
import { Field } from "@/components/ui/Field/Field";
import { PRIORITIES, type Card, type Column, type Priority, type User } from "@/types";
import s from "./CardModal.module.scss";

/** Column, assignee, priority, author and dates, right of the text. */
interface Props {
  columns: Column[];
  users: User[];
  columnId: number;
  priority: Priority;
  assigneeId: number | null;
  author: User | undefined;
  card: Card | null;
  onColumn: (id: number) => void;
  onPriority: (p: Priority) => void;
  onAssignee: (id: number | null) => void;
  /** More fields below the standard ones (task details). */
  children?: ReactNode;
}

export function CardSidebar({ columns, users, columnId, priority, assigneeId, author, card, onColumn, onPriority, onAssignee, children }: Props) {
  const { t, i18n } = useTranslation();
  const dateFmt = useMemo(() => new Intl.DateTimeFormat(i18n.language, { dateStyle: "medium", timeStyle: "short" }), [i18n.language]);
  const assignee = users.find((u) => u.id === assigneeId);
  return (
    <aside className={s.side}>
      <Field label={t("card.column")}>
        <select value={columnId} onChange={(e) => onColumn(Number(e.target.value))}>
          {columns.map((c) => (
            <option key={c.id} value={c.id}>
              {c.name}
            </option>
          ))}
        </select>
      </Field>
      <Field label={t("card.assignee")}>
        <div className={s.selectWithAvatar}>
          <Avatar user={assignee} size={24} />
          <select value={assigneeId ?? ""} onChange={(e) => onAssignee(e.target.value ? Number(e.target.value) : null)}>
            <option value="">{t("common.unassigned")}</option>
            {users.map((u) => (
              <option key={u.id} value={u.id}>
                {u.name}
              </option>
            ))}
          </select>
        </div>
      </Field>
      <Field label={t("card.priority")}>
        <select value={priority} onChange={(e) => onPriority(e.target.value as Priority)}>
          {PRIORITIES.map((p) => (
            <option key={p} value={p}>
              {t(`priority.${p}`)}
            </option>
          ))}
        </select>
      </Field>
      <Field as="div" label={t("card.author")}>
        <div className={s.person}>
          <Avatar user={author} size={24} />
          <span>{author?.name ?? t("common.formerMember")}</span>
        </div>
      </Field>
      {children}
      {card && (
        <div className={s.meta}>
          <div>{t("card.created", { date: dateFmt.format(new Date(card.createdAt)) })}</div>
          <div>{t("card.updated", { date: dateFmt.format(new Date(card.updatedAt)) })}</div>
        </div>
      )}
    </aside>
  );
}
