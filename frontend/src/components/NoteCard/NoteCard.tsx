import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import s from "./NoteCard.module.scss";

interface Props {
  author: string; // "" = anonymous
  createdAt: string;
  badge?: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
}

/** One comment or piece of feedback: who (or "anonymous"), when, the text. */
export function NoteCard({ author, createdAt, badge, actions, children }: Props) {
  const { t, i18n } = useTranslation();
  const when = new Intl.DateTimeFormat(i18n.language, { dateStyle: "medium", timeStyle: "short" }).format(new Date(createdAt));
  return (
    <article className={s.note}>
      <header className={s.head}>
        <strong>{author || t("people.anonymousAuthor")}</strong>
        {badge}
        <span className={s.when}>{when}</span>
        {actions}
      </header>
      <div className={s.body}>{children}</div>
    </article>
  );
}
