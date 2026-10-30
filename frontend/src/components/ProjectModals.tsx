import { useEffect, useMemo, useState, type FormEvent, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { api } from "../api";
import type { Member, Project, User } from "../types";
import { Avatar } from "./Avatar";

export function Modal({ title, onClose, children, wide }: { title: string; onClose: () => void; children: ReactNode; wide?: boolean }) {
  const { t } = useTranslation();
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
          <button type="button" className="icon-btn" onClick={onClose} aria-label={t("common.close")}>
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
  const { t } = useTranslation();
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
    <Modal title={t("project.newTitle")} onClose={onClose}>
      <form className="form modal-content" onSubmit={submit}>
        <label>
          {t("common.name")}
          <input
            value={name}
            onChange={(e) => {
              setName(e.target.value);
              if (!keyTouched) setKey(suggestKey(e.target.value));
            }}
            placeholder={t("project.namePlaceholder")}
            maxLength={100}
            required
            autoFocus
          />
        </label>
        <label>
          {t("project.key")}
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
          <small>{t("project.keyHint", { key: key || "KEY" })}</small>
        </label>
        {error && <div className="form-error">{error}</div>}
        <div className="modal-actions">
          <button type="button" className="btn btn-ghost" onClick={onClose}>
            {t("common.cancel")}
          </button>
          <button className="btn btn-primary" disabled={busy}>
            {t("common.create")}
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
  const { t } = useTranslation();
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
      if (!confirm(t("project.confirmRemoveMember", { name: m.name }))) return;
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
      if (!confirm(t("project.confirmLeave", { name: project.name }))) return;
      await api.removeMember(project.id, currentUserId);
      await onLeftOrDeleted();
    });

  const destroy = () =>
    run(async () => {
      if (!confirm(t("project.confirmDelete", { name: project.name }))) return;
      await api.deleteProject(project.id);
      await onLeftOrDeleted();
    });

  return (
    <Modal title={t("project.settingsTitle", { key: project.key })} onClose={onClose} wide>
      <div className="modal-content settings">
        {error && <div className="form-error">{error}</div>}

        {isOwner && (
          <section>
            <h3>{t("project.projectName")}</h3>
            <form className="inline-form" onSubmit={rename}>
              <input value={name} onChange={(e) => setName(e.target.value)} maxLength={100} required />
              <button className="btn btn-ghost" disabled={busy || name.trim() === project.name}>
                {t("common.save")}
              </button>
            </form>
          </section>
        )}

        <section>
          <h3>{t("project.members", { count: members.length })}</h3>
          <div className="member-list">
            {members.map((m) => (
              <div className="member-row" key={m.id}>
                <Avatar user={m} size={32} />
                <div className="member-info">
                  <span>
                    {m.name}
                    {m.id === currentUserId && <em> {t("common.you")}</em>}
                  </span>
                  <small>{m.email}</small>
                </div>
                <span className={`role role--${m.role}`}>{t(`roles.${m.role}`)}</span>
                {isOwner && m.role !== "owner" && (
                  <button className="btn btn-danger btn-sm" onClick={() => remove(m)} disabled={busy}>
                    {t("common.remove")}
                  </button>
                )}
              </div>
            ))}
          </div>
        </section>

        {isOwner && (
          <section>
            <h3>{t("project.addPeople")}</h3>
            <input
              className="member-search"
              placeholder={t("project.searchPeople")}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
            <div className="member-list member-list--scroll">
              {candidates.length === 0 && (
                <div className="muted">
                  {allUsers.length === 0 ? t("common.loading") : t("project.noCandidates")}
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
                    {t("common.add")}
                  </button>
                </div>
              ))}
            </div>
            <p className="muted">{t("project.mustRegister")}</p>
          </section>
        )}

        <section className="danger-zone">
          {isOwner ? (
            <>
              <div>
                <strong>{t("project.deleteTitle")}</strong>
                <p className="muted">{t("project.deleteText")}</p>
              </div>
              <button className="btn btn-danger-solid" onClick={destroy} disabled={busy}>
                {t("project.deleteTitle")}
              </button>
            </>
          ) : (
            <>
              <div>
                <strong>{t("project.leaveTitle")}</strong>
                <p className="muted">{t("project.leaveText")}</p>
              </div>
              <button className="btn btn-danger-solid" onClick={leave} disabled={busy}>
                {t("project.leave")}
              </button>
            </>
          )}
        </section>
      </div>
    </Modal>
  );
}
