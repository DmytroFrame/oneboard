import { useEffect, useState, type FormEvent } from "react";
import { COLUMNS, PRIORITIES, type Card, type CardInput, type Priority, type Status, type User } from "../types";
import { Avatar } from "./Avatar";

interface Props {
  card: Card | null; // null = creating a new card
  initialStatus: Status;
  users: User[];
  currentUser: User;
  canDelete: boolean;
  onClose: () => void;
  onSave: (input: CardInput, status: Status) => Promise<void>;
  onDelete: (card: Card) => Promise<void>;
}

const dateFmt = new Intl.DateTimeFormat("uk-UA", { dateStyle: "medium", timeStyle: "short" });

export function CardModal({ card, initialStatus, users, currentUser, canDelete, onClose, onSave, onDelete }: Props) {
  const [title, setTitle] = useState(card?.title ?? "");
  const [description, setDescription] = useState(card?.description ?? "");
  const [status, setStatus] = useState<Status>(card?.status ?? initialStatus);
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
      await onSave({ title, description, priority, assigneeId }, status);
      onClose();
    } catch (err) {
      setError((err as Error).message);
      setBusy(false);
    }
  }

  async function remove() {
    if (!card || !confirm(`Видалити ${card.key}?`)) return;
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
          <span className="modal-key">{card ? card.key : "Нова задача"}</span>
          <button type="button" className="icon-btn" onClick={onClose} aria-label="Закрити">
            ✕
          </button>
        </div>

        <div className="modal-body">
          <div className="modal-main">
            <input
              className="title-input"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Що потрібно зробити?"
              maxLength={200}
              required
              autoFocus
            />
            <label className="field">
              Опис
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                rows={8}
                placeholder="Додайте деталі…"
              />
            </label>
            {error && <div className="form-error">{error}</div>}
          </div>

          <aside className="modal-side">
            <label className="field">
              Статус
              <select value={status} onChange={(e) => setStatus(e.target.value as Status)}>
                {COLUMNS.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.title}
                  </option>
                ))}
              </select>
            </label>

            <label className="field">
              Виконавець
              <div className="select-with-avatar">
                <Avatar user={assignee} size={24} />
                <select
                  value={assigneeId ?? ""}
                  onChange={(e) => setAssigneeId(e.target.value ? Number(e.target.value) : null)}
                >
                  <option value="">Не призначено</option>
                  {users.map((u) => (
                    <option key={u.id} value={u.id}>
                      {u.name}
                    </option>
                  ))}
                </select>
              </div>
            </label>

            <label className="field">
              Пріоритет
              <select value={priority} onChange={(e) => setPriority(e.target.value as Priority)}>
                {PRIORITIES.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.title}
                  </option>
                ))}
              </select>
            </label>

            <div className="field">
              Автор
              <div className="person">
                <Avatar user={author} size={24} />
                <span>{author?.name ?? "Колишній учасник"}</span>
              </div>
            </div>

            {card && (
              <div className="meta">
                <div>Створено: {dateFmt.format(new Date(card.createdAt))}</div>
                <div>Оновлено: {dateFmt.format(new Date(card.updatedAt))}</div>
              </div>
            )}
          </aside>
        </div>

        <div className="modal-footer">
          {canDelete && (
            <button type="button" className="btn btn-danger" onClick={remove} disabled={busy}>
              Видалити
            </button>
          )}
          <div className="spacer" />
          <button type="button" className="btn btn-ghost" onClick={onClose}>
            Скасувати
          </button>
          <button className="btn btn-primary" disabled={busy}>
            {card ? "Зберегти" : "Створити"}
          </button>
        </div>
      </form>
    </div>
  );
}
