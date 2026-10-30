import { useEffect, useMemo, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { PRIORITIES, type Card, type CardInput, type Column, type Priority, type User } from "../types";
import { Avatar } from "./Avatar";

interface Props {
  card: Card | null; // null = creating a new card
  initialColumnId: number;
  columns: Column[];
  users: User[];
  currentUser: User;
  canDelete: boolean;
  onClose: () => void;
  onSave: (input: CardInput, columnId: number) => Promise<void>;
  onDelete: (card: Card) => Promise<void>;
}

export function CardModal({ card, initialColumnId, columns, users, currentUser, canDelete, onClose, onSave, onDelete }: Props) {
  const { t, i18n } = useTranslation();
  const dateFmt = useMemo(() => new Intl.DateTimeFormat(i18n.language, { dateStyle: "medium", timeStyle: "short" }), [i18n.language]);
  const [title, setTitle] = useState(card?.title ?? "");
  const [description, setDescription] = useState(card?.description ?? "");
  const [columnId, setColumnId] = useState(card?.columnId ?? initialColumnId);
  const [priority, setPriority] = useState<Priority>(card?.priority ?? "medium");
  const [assigneeId, setAssigneeId] = useState<number | null>(card ? card.assigneeId : currentUser.id);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const author = card ? users.find((u) => u.id === card.authorId) : currentUser;
  const assignee = users.find((u) => u.id === assigneeId);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await onSave({ title, description, priority, assigneeId }, columnId);
      onClose();
    } catch (err) {
      setError((err as Error).message);
      setBusy(false);
    }
  }

  async function remove() {
    if (!card || !confirm(t("card.deleteConfirm", { key: card.key }))) return;
    setBusy(true);
    try {
      await onDelete(card);
      onClose();
    } catch (err) {
      setError((err as Error).message);
      setBusy(false);
    }
  }

  return (
    <div className="modal-backdrop" onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
      <form className="modal" onSubmit={submit}>
        <div className="modal-header">
          <span className="modal-key">{card ? card.key : t("card.newTask")}</span>
          <button type="button" className="icon-btn" onClick={onClose} aria-label={t("common.close")}>
            ✕
          </button>
        </div>

        <div className="modal-body">
          <div className="modal-main">
            <input
              className="title-input"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder={t("card.titlePlaceholder")}
              maxLength={200}
              required
              autoFocus
            />
            <label className="field">
              {t("card.description")}
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                rows={8}
                placeholder={t("card.descriptionPlaceholder")}
              />
            </label>
            {error && <div className="form-error">{error}</div>}
          </div>

          <aside className="modal-side">
            <label className="field">
              {t("card.column")}
              <select value={columnId} onChange={(e) => setColumnId(Number(e.target.value))}>
                {columns.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </select>
            </label>

            <label className="field">
              {t("card.assignee")}
              <div className="select-with-avatar">
                <Avatar user={assignee} size={24} />
                <select
                  value={assigneeId ?? ""}
                  onChange={(e) => setAssigneeId(e.target.value ? Number(e.target.value) : null)}
                >
                  <option value="">{t("common.unassigned")}</option>
                  {users.map((u) => (
                    <option key={u.id} value={u.id}>
                      {u.name}
                    </option>
                  ))}
                </select>
              </div>
            </label>

            <label className="field">
              {t("card.priority")}
              <select value={priority} onChange={(e) => setPriority(e.target.value as Priority)}>
                {PRIORITIES.map((p) => (
                  <option key={p} value={p}>
                    {t(`priority.${p}`)}
                  </option>
                ))}
              </select>
            </label>

            <div className="field">
              {t("card.author")}
              <div className="person">
                <Avatar user={author} size={24} />
                <span>{author?.name ?? t("common.formerMember")}</span>
              </div>
            </div>

            {card && (
              <div className="meta">
                <div>{t("card.created", { date: dateFmt.format(new Date(card.createdAt)) })}</div>
                <div>{t("card.updated", { date: dateFmt.format(new Date(card.updatedAt)) })}</div>
              </div>
            )}
          </aside>
        </div>

        <div className="modal-footer">
          {canDelete && (
            <button type="button" className="btn btn-danger" onClick={remove} disabled={busy}>
              {t("common.delete")}
            </button>
          )}
          <div className="spacer" />
          <button type="button" className="btn btn-ghost" onClick={onClose}>
            {t("common.cancel")}
          </button>
          <button className="btn btn-primary" disabled={busy}>
            {card ? t("common.save") : t("common.create")}
          </button>
        </div>
      </form>
    </div>
  );
}
