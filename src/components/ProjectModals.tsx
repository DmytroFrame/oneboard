import { useEffect, useMemo, useState, type FormEvent, type ReactNode } from "react";
import { api } from "../api";
import type { Member, Project, User } from "../types";
import { Avatar } from "./Avatar";

function Modal({ title, onClose, children, wide }: { title: string; onClose: () => void; children: ReactNode; wide?: boolean }) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div className="modal-backdrop" onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
      <div className={`modal ${wide ? "" : "modal--narrow"}`}>
        <div className="modal-header">
          <span className="modal-title">{title}</span>
          <button type="button" className="icon-btn" onClick={onClose} aria-label="Закрити">
            ✕
          </button>
        </div>
        {children}
      </div>
    </div>
  );
}

/** Suggests a project key from its name: first letters of words, latin only. */
function suggestKey(name: string) {
  const words = name.toUpperCase().match(/[A-Z][A-Z0-9]*/g) ?? [];
  const key = words.length > 1 ? words.map((w) => w[0]).join("") : (words[0] ?? "");
  return key.slice(0, 10);
}

export function CreateProjectModal({ onClose, onCreated }: { onClose: () => void; onCreated: (p: Project) => Promise<void> }) {
  const [name, setName] = useState("");
  const [key, setKey] = useState("");
  const [keyTouched, setKeyTouched] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const p = await api.createProject(name, key);
      await onCreated(p);
      onClose();
    } catch (err) {
      setError((err as Error).message);
      setBusy(false);
    }
  }

  return (
    <Modal title="Новий проєкт" onClose={onClose}>
      <form className="form modal-content" onSubmit={submit}>
        <label>
          Назва
          <input
            value={name}
            onChange={(e) => {
              setName(e.target.value);
              if (!keyTouched) setKey(suggestKey(e.target.value));
            }}
            placeholder="Наприклад: Website Redesign"
            maxLength={100}
            required
            autoFocus
          />
        </label>
        <label>
          Ключ
          <input
            value={key}
            onChange={(e) => {
              setKeyTouched(true);
              setKey(e.target.value.toUpperCase().replace(/[^A-Z0-9]/g, ""));
            }}
            placeholder="WEB"
            minLength={2}
            maxLength={10}
            pattern="[A-Z][A-Z0-9]{1,9}"
            required
          />
          <small>Префікс задач: {key || "KEY"}-1, {key || "KEY"}-2… (латиниця, 2–10 символів)</small>
        </label>
        {error && <div className="form-error">{error}</div>}
        <div className="modal-actions">
          <button type="button" className="btn btn-ghost" onClick={onClose}>
            Скасувати
          </button>
          <button className="btn btn-primary" disabled={busy}>
            Створити
          </button>
        </div>
      </form>
    </Modal>
  );
}

interface SettingsProps {
  project: Project;
  members: Member[];
  currentUserId: number;
  onClose: () => void;
  onMembersChanged: (members: Member[]) => void;
  onProjectChanged: () => Promise<void>;
  onLeftOrDeleted: () => Promise<void>;
}

export function ProjectSettingsModal({
  project,
  members,
  currentUserId,
  onClose,
  onMembersChanged,
  onProjectChanged,
  onLeftOrDeleted,
}: SettingsProps) {
  const isOwner = project.role === "owner";
  const [allUsers, setAllUsers] = useState<User[]>([]);
  const [query, setQuery] = useState("");
  const [name, setName] = useState(project.name);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (isOwner) api.users().then(setAllUsers).catch((err) => setError(err.message));
  }, [isOwner]);

  const candidates = useMemo(() => {
    const memberIds = new Set(members.map((m) => m.id));
    const q = query.trim().toLowerCase();
    return allUsers
      .filter((u) => !memberIds.has(u.id))
      .filter((u) => !q || u.name.toLowerCase().includes(q) || u.email.toLowerCase().includes(q));
  }, [allUsers, members, query]);

  async function run(action: () => Promise<void>) {
    setBusy(true);
    setError("");
    try {
      await action();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  const add = (u: User) => run(async () => onMembersChanged(await api.addMember(project.id, u.id)));

  const remove = (m: Member) =>
    run(async () => {
      if (!confirm(`Видалити ${m.name} з проєкту? Його задачі стануть непризначеними.`)) return;
      await api.removeMember(project.id, m.id);
      onMembersChanged(members.filter((x) => x.id !== m.id));
    });

  const rename = (e: FormEvent) => {
    e.preventDefault();
    run(async () => {
      await api.renameProject(project.id, name);
      await onProjectChanged();
    });
  };

  const leave = () =>
    run(async () => {
      if (!confirm(`Вийти з проєкту «${project.name}»?`)) return;
      await api.removeMember(project.id, currentUserId);
      await onLeftOrDeleted();
    });

  const destroy = () =>
    run(async () => {
      if (!confirm(`Видалити проєкт «${project.name}» разом з усіма задачами? Це незворотно.`)) return;
      await api.deleteProject(project.id);
      await onLeftOrDeleted();
    });

  return (
    <Modal title={`Налаштування · ${project.key}`} onClose={onClose} wide>
      <div className="modal-content settings">
        {error && <div className="form-error">{error}</div>}

        {isOwner && (
          <section>
            <h3>Назва проєкту</h3>
            <form className="inline-form" onSubmit={rename}>
              <input value={name} onChange={(e) => setName(e.target.value)} maxLength={100} required />
              <button className="btn btn-ghost" disabled={busy || name.trim() === project.name}>
                Зберегти
              </button>
            </form>
          </section>
        )}

        <section>
          <h3>Учасники · {members.length}</h3>
          <div className="member-list">
            {members.map((m) => (
              <div className="member-row" key={m.id}>
                <Avatar user={m} size={32} />
                <div className="member-info">
                  <span>
                    {m.name}
                    {m.id === currentUserId && <em> (ви)</em>}
                  </span>
                  <small>{m.email}</small>
                </div>
                <span className={`role role--${m.role}`}>{m.role === "owner" ? "Власник" : "Учасник"}</span>
                {isOwner && m.role !== "owner" && (
                  <button className="btn btn-danger btn-sm" onClick={() => remove(m)} disabled={busy}>
                    Видалити
                  </button>
                )}
              </div>
            ))}
          </div>
        </section>

        {isOwner && (
          <section>
            <h3>Додати людей</h3>
            <input
              className="member-search"
              placeholder="Пошук за ім'ям або email"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
            <div className="member-list member-list--scroll">
              {candidates.length === 0 && (
                <div className="muted">
                  {allUsers.length === 0 ? "Завантаження…" : "Немає користувачів для додавання"}
                </div>
              )}
              {candidates.map((u) => (
                <div className="member-row" key={u.id}>
                  <Avatar user={u} size={32} />
                  <div className="member-info">
                    <span>{u.name}</span>
                    <small>{u.email}</small>
                  </div>
                  <button className="btn btn-primary btn-sm" onClick={() => add(u)} disabled={busy}>
                    Додати
                  </button>
                </div>
              ))}
            </div>
            <p className="muted">Людина має спершу зареєструватися у Flowboard.</p>
          </section>
        )}

        <section className="danger-zone">
          {isOwner ? (
            <>
              <div>
                <strong>Видалити проєкт</strong>
                <p className="muted">Усі задачі проєкту буде видалено назавжди.</p>
              </div>
              <button className="btn btn-danger-solid" onClick={destroy} disabled={busy}>
                Видалити проєкт
              </button>
            </>
          ) : (
            <>
              <div>
                <strong>Вийти з проєкту</strong>
                <p className="muted">Ваші задачі в цьому проєкті стануть непризначеними.</p>
              </div>
              <button className="btn btn-danger-solid" onClick={leave} disabled={busy}>
                Вийти
              </button>
            </>
          )}
        </section>
      </div>
    </Modal>
  );
}
