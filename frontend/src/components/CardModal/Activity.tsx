import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/Button/Button";
import { IconButton } from "@/components/ui/IconButton/IconButton";
import { Muted } from "@/components/ui/Muted/Muted";
import { Tabs } from "@/components/ui/Tabs/Tabs";
import { tasksApi } from "@/lib/api/tasks";
import type { Card, Column, User } from "@/types";
import type { CardComment, HistoryEntry } from "@/types/tasks";
import s from "./CardModal.module.scss";

type Tab = "comments" | "history" | "worklog";

interface Props {
  card: Card;
  currentUser: User;
  isProjectOwner: boolean;
  columns: Column[];
  users: User[];
  /** The work log tab (time tracking), rendered by the caller. */
  worklog?: ReactNode;
  onChanged: () => void;
}

/** Comments, change history and the work log of a task. */
export function Activity({ card, currentUser, isProjectOwner, columns, users, worklog, onChanged }: Props) {
  const { t } = useTranslation();
  const [tab, setTab] = useState<Tab>("comments");
  const tabs: { id: Tab; label: string }[] = [
    { id: "comments", label: t("tasks.tabCommentsActivity") },
    { id: "history", label: t("tasks.tabHistory") },
  ];
  if (worklog) tabs.push({ id: "worklog", label: t("tasks.tabWorklog") });
  return (
    <section className={s.block}>
      <Tabs tabs={tabs} value={tab} onChange={setTab} />
      {tab === "comments" && <Comments cardId={card.id} currentUser={currentUser} isProjectOwner={isProjectOwner} onChanged={onChanged} />}
      {tab === "history" && <History cardId={card.id} columns={columns} users={users} />}
      {tab === "worklog" && worklog}
    </section>
  );
}

function when(iso: string, locale: string) {
  return new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" }).format(new Date(iso));
}

function Comments({ cardId, currentUser, isProjectOwner, onChanged }: { cardId: number; currentUser: User; isProjectOwner: boolean; onChanged: () => void }) {
  const { t, i18n } = useTranslation();
  const [items, setItems] = useState<CardComment[] | null>(null);
  const [body, setBody] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    tasksApi.comments(cardId).then(setItems).catch((err) => setError(err.message));
  }, [cardId]);

  async function send(e: FormEvent) {
    e.preventDefault();
    try {
      const c = await tasksApi.addComment(cardId, body);
      setItems((l) => [...(l ?? []), c]);
      setBody("");
      onChanged();
    } catch (err) {
      setError((err as Error).message);
    }
  }

  return (
    <div className={s.feed}>
      {items?.length === 0 && <Muted as="p">{t("tasks.noComments")}</Muted>}
      {items?.map((c) => (
        <div key={c.id} className={s.comment}>
          <div className={s.commentHead}>
            <strong>{c.authorName || "—"}</strong>
            <span>
              {when(c.createdAt, i18n.language)}
              {c.updatedAt !== c.createdAt && ` · ${t("tasks.edited")}`}
            </span>
            {(c.authorId === currentUser.id || isProjectOwner) && (
              <IconButton
                size="sm"
                aria-label={t("common.delete")}
                onClick={() => tasksApi.deleteComment(c.id).then(() => (setItems((l) => l!.filter((x) => x.id !== c.id)), onChanged()))}
              >
                ✕
              </IconButton>
            )}
          </div>
          <div className={s.commentBody}>{c.body}</div>
        </div>
      ))}
      <form className={s.commentForm} onSubmit={send}>
        <textarea rows={2} value={body} onChange={(e) => setBody(e.target.value)} placeholder={t("tasks.commentPlaceholder")} maxLength={10000} required />
        <Button variant="primary" size="sm">
          {t("tasks.send")}
        </Button>
      </form>
      {error && <Muted as="p">{error}</Muted>}
    </div>
  );
}

function History({ cardId, columns, users }: { cardId: number; columns: Column[]; users: User[] }) {
  const { t, i18n } = useTranslation();
  const [items, setItems] = useState<HistoryEntry[]>([]);
  const [more, setMore] = useState(true);

  const load = (offset: number) =>
    tasksApi.history(cardId, offset).then((page) => {
      setItems((l) => (offset ? [...l, ...page] : page));
      setMore(page.length === 30);
    });
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => void load(0), [cardId]);

  const show = (field: string, v: string) => {
    if (!v) return "—";
    if (field === "column") return columns.find((c) => String(c.id) === v)?.name ?? v;
    if (field === "assignee") return users.find((u) => String(u.id) === v)?.name ?? v;
    if (field === "priority") return t(`priority.${v}` as "priority.low");
    if (field === "specialType") return t(`tasks.specialTypes.${v}` as "tasks.specialTypes.bench");
    if (field === "description") return "…";
    return v;
  };

  return (
    <div className={s.feed}>
      {items.length === 0 && <Muted as="p">{t("tasks.noHistory")}</Muted>}
      {items.map((h) => (
        <div key={h.id} className={s.historyRow}>
          <span>
            <strong>{h.actorName || "—"}</strong>{" "}
            {t("tasks.changed", { field: t(`tasks.historyFields.${h.field}` as "tasks.historyFields.title", { defaultValue: h.field }) })}
            {h.field !== "description" && (
              <>
                : <s>{show(h.field, h.old)}</s> → {show(h.field, h.new)}
              </>
            )}
          </span>
          <small>{when(h.createdAt, i18n.language)}</small>
        </div>
      ))}
      {more && items.length > 0 && (
        <Button type="button" size="sm" onClick={() => load(items.length)}>
          {t("tasks.loadMore")}
        </Button>
      )}
    </div>
  );
}
