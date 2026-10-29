import { useCallback, useEffect, useMemo, useRef, useState, type DragEvent } from "react";
import { Navigate, useNavigate, useParams } from "react-router-dom";
import { api } from "../api";
import { useAuth } from "../auth";
import { Avatar } from "../components/Avatar";
import { CardModal } from "../components/CardModal";
import { ProjectSettingsModal } from "../components/ProjectModals";
import { COLUMNS, type Card, type CardInput, type Member, type Project, type Status } from "../types";
import { rememberProject, useLayout } from "./AppLayout";

const VISIBLE_AVATARS = 6;

const PRIORITY_ICON: Record<Card["priority"], { icon: string; color: string; label: string }> = {
  low: { icon: "↓", color: "#2684FF", label: "Низький" },
  medium: { icon: "=", color: "#FFAB00", label: "Середній" },
  high: { icon: "↑", color: "#FF7452", label: "Високий" },
  critical: { icon: "⇈", color: "#DE350B", label: "Критичний" },
};

type ModalState = { card: Card | null; status: Status } | null;
type DropTarget = { status: Status; beforeId: number | null } | null;

/** Returns a new card list with `id` moved into `status` before `beforeId` (or to the end). */
function applyMove(cards: Card[], id: number, status: Status, beforeId: number | null): Card[] {
  const moving = cards.find((c) => c.id === id);
  if (!moving) return cards;
  const column = cards
    .filter((c) => c.status === status && c.id !== id)
    .sort((a, b) => a.position - b.position);
  const idx = beforeId === null ? -1 : column.findIndex((c) => c.id === beforeId);
  column.splice(idx === -1 ? column.length : idx, 0, { ...moving, status });
  const updated = new Map(column.map((c, position) => [c.id, { ...c, position }]));
  return cards.map((c) => updated.get(c.id) ?? c);
}

/** Route wrapper: remounts the board when switching projects so all state resets. */
export default function BoardRoute() {
  const { projectId } = useParams();
  const { projects } = useLayout();
  const project = projects.find((p) => p.id === Number(projectId));
  if (!project) return <Navigate to="/projects" replace />;
  return <Board key={project.id} project={project} />;
}

function Board({ project }: { project: Project }) {
  const { user } = useAuth();
  const { reloadProjects } = useLayout();
  const navigate = useNavigate();
  const me = user!;
  const [cards, setCards] = useState<Card[]>([]);
  const [members, setMembers] = useState<Member[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [search, setSearch] = useState("");
  const [selectedUsers, setSelectedUsers] = useState<Set<number>>(new Set());
  const [showUnassigned, setShowUnassigned] = useState(false);
  const [teamOpen, setTeamOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [modal, setModal] = useState<ModalState>(null);
  const [draggingId, setDraggingId] = useState<number | null>(null);
  const [dropTarget, setDropTarget] = useState<DropTarget>(null);
  const teamRef = useRef<HTMLDivElement>(null);

  const load = useCallback(async () => {
    try {
      const [c, m] = await Promise.all([api.cards(project.id), api.members(project.id)]);
      setCards(c);
      setMembers(m);
      setError("");
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  }, [project.id]);

  useEffect(() => {
    rememberProject(project.id);
    load();
  }, [load, project.id]);

  useEffect(() => {
    if (!teamOpen) return;
    const onClick = (e: MouseEvent) => {
      if (!teamRef.current?.contains(e.target as Node)) setTeamOpen(false);
    };
    document.addEventListener("mousedown", onClick);
    return () => document.removeEventListener("mousedown", onClick);
  }, [teamOpen]);

  const membersById = useMemo(() => new Map(members.map((u) => [u.id, u])), [members]);

  // Current user first, then everyone else alphabetically — like Jira's avatar group.
  const orderedMembers = useMemo(
    () => [...members].sort((a, b) => (a.id === me.id ? -1 : b.id === me.id ? 1 : a.name.localeCompare(b.name))),
    [members, me.id],
  );

  const assignedCount = useMemo(() => {
    const m = new Map<number, number>();
    for (const c of cards) if (c.assigneeId) m.set(c.assigneeId, (m.get(c.assigneeId) ?? 0) + 1);
    return m;
  }, [cards]);

  const hasFilters = selectedUsers.size > 0 || showUnassigned || search.trim() !== "";

  const visible = useMemo(() => {
    const q = search.trim().toLowerCase();
    return cards.filter((c) => {
      if (selectedUsers.size > 0 || showUnassigned) {
        const match = c.assigneeId === null ? showUnassigned : selectedUsers.has(c.assigneeId);
        if (!match) return false;
      }
      if (q && !c.title.toLowerCase().includes(q) && !c.key.toLowerCase().includes(q)) return false;
      return true;
    });
  }, [cards, selectedUsers, showUnassigned, search]);

  const columns = useMemo(
    () =>
      COLUMNS.map((col) => ({
        ...col,
        cards: visible.filter((c) => c.status === col.id).sort((a, b) => a.position - b.position),
      })),
    [visible],
  );

  function toggleUser(id: number) {
    setSelectedUsers((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function clearFilters() {
    setSelectedUsers(new Set());
    setShowUnassigned(false);
    setSearch("");
  }

  function onMembersChanged(next: Member[]) {
    const ids = new Set(next.map((m) => m.id));
    // Removed members' cards are unassigned on the server; mirror that locally.
    setCards((prev) => prev.map((c) => (c.assigneeId && !ids.has(c.assigneeId) ? { ...c, assigneeId: null } : c)));
    setSelectedUsers((prev) => new Set([...prev].filter((id) => ids.has(id))));
    setMembers(next);
    reloadProjects();
  }

  // ---------- drag & drop ----------

  function onDragStart(e: DragEvent, card: Card) {
    e.dataTransfer.effectAllowed = "move";
    e.dataTransfer.setData("text/plain", String(card.id));
    // Defer so the browser captures the drag image before the card fades.
    requestAnimationFrame(() => setDraggingId(card.id));
  }

  function onDragEnd() {
    setDraggingId(null);
    setDropTarget(null);
  }

  function onColumnDragOver(e: DragEvent<HTMLElement>, status: Status) {
    if (draggingId === null) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = "move";
    const els = e.currentTarget.querySelectorAll<HTMLElement>("[data-card-id]");
    let beforeId: number | null = null;
    for (const el of els) {
      const id = Number(el.dataset.cardId);
      if (id === draggingId) continue;
      const rect = el.getBoundingClientRect();
      if (e.clientY < rect.top + rect.height / 2) {
        beforeId = id;
        break;
      }
    }
    if (dropTarget?.status !== status || dropTarget.beforeId !== beforeId) {
      setDropTarget({ status, beforeId });
    }
  }

  async function onColumnDrop(e: DragEvent, status: Status) {
    e.preventDefault();
    const id = draggingId;
    const beforeId = dropTarget?.status === status ? dropTarget.beforeId : null;
    onDragEnd();
    if (id === null) return;

    setCards((prev) => applyMove(prev, id, status, beforeId));
    try {
      const saved = await api.moveCard(id, status, beforeId);
      setCards((prev) => prev.map((c) => (c.id === saved.id ? { ...c, updatedAt: saved.updatedAt } : c)));
    } catch (err) {
      setError((err as Error).message);
      load();
    }
  }

  // ---------- CRUD ----------

  async function saveCard(input: CardInput, status: Status) {
    if (!modal) return;
    if (modal.card === null) {
      const created = await api.createCard(project.id, { ...input, status });
      setCards((prev) => [...prev, created]);
      reloadProjects();
      return;
    }
    const id = modal.card.id;
    const saved = await api.updateCard(id, input);
    if (status !== modal.card.status) {
      await api.moveCard(id, status, null);
      // Positions in the target column were renumbered, so refresh the board.
      await load();
    } else {
      setCards((prev) => prev.map((c) => (c.id === id ? saved : c)));
    }
  }

  async function deleteCard(card: Card) {
    await api.deleteCard(card.id);
    setCards((prev) => prev.filter((c) => c.id !== card.id));
    reloadProjects();
  }

  // ---------- render ----------

  const overflow = orderedMembers.length - VISIBLE_AVATARS;
  const onlyMine = selectedUsers.size === 1 && selectedUsers.has(me.id);

  return (
    <div className="board">
      <div className="board-head">
        <div>
          <div className="breadcrumbs">Проєкти / {project.name}</div>
          <h1>Дошка {project.key}</h1>
        </div>
        <div className="board-head-actions">
          <button className="btn btn-ghost" onClick={() => setSettingsOpen(true)}>
            ⚙ Учасники та налаштування
          </button>
          <button className="btn btn-primary" onClick={() => setModal({ card: null, status: "todo" })}>
            + Створити
          </button>
        </div>
      </div>

      <div className="toolbar">
        <input className="search" placeholder="Пошук по дошці" value={search} onChange={(e) => setSearch(e.target.value)} />

        <div className="avatar-group">
          {orderedMembers.slice(0, VISIBLE_AVATARS).map((u) => (
            <Avatar key={u.id} user={u} size={34} active={selectedUsers.has(u.id)} onClick={() => toggleUser(u.id)} />
          ))}
          <div className="team-dropdown" ref={teamRef}>
            <button
              className={`avatar avatar--clickable avatar--more${teamOpen ? " avatar--active" : ""}`}
              style={{ width: 34, height: 34 }}
              onClick={() => setTeamOpen((v) => !v)}
              title="Усі учасники проєкту"
            >
              {overflow > 0 ? `+${overflow}` : "☰"}
            </button>
            {teamOpen && (
              <div className="team-menu">
                <div className="team-menu-title">Учасники · {members.length}</div>
                {orderedMembers.map((u) => (
                  <label key={u.id} className="team-row">
                    <input type="checkbox" checked={selectedUsers.has(u.id)} onChange={() => toggleUser(u.id)} />
                    <Avatar user={u} size={28} />
                    <span className="team-row-name">
                      <span>
                        {u.name}
                        {u.id === me.id && <em> (ви)</em>}
                        {u.role === "owner" && <em> · власник</em>}
                      </span>
                      <small>{u.email}</small>
                    </span>
                    <span className="badge">{assignedCount.get(u.id) ?? 0}</span>
                  </label>
                ))}
                <label className="team-row">
                  <input type="checkbox" checked={showUnassigned} onChange={() => setShowUnassigned((v) => !v)} />
                  <Avatar user={null} size={28} />
                  <span className="team-row-name">Не призначено</span>
                  <span className="badge">{cards.filter((c) => c.assigneeId === null).length}</span>
                </label>
                {project.role === "owner" && (
                  <button
                    className="team-menu-add"
                    onClick={() => {
                      setTeamOpen(false);
                      setSettingsOpen(true);
                    }}
                  >
                    + Додати людей
                  </button>
                )}
              </div>
            )}
          </div>
        </div>

        <button
          className={`btn btn-sm ${onlyMine ? "btn-selected" : "btn-ghost"}`}
          onClick={() => setSelectedUsers(onlyMine ? new Set() : new Set([me.id]))}
        >
          Тільки мої задачі
        </button>
        {hasFilters && (
          <button className="btn btn-link btn-sm" onClick={clearFilters}>
            Очистити фільтри
          </button>
        )}
      </div>

      {error && (
        <div className="banner-error">
          {error} <button onClick={() => setError("")}>✕</button>
        </div>
      )}

      {loading ? (
        <div className="center">Завантаження дошки…</div>
      ) : (
        <div className="columns">
          {columns.map((col) => (
            <section
              key={col.id}
              className={`column${dropTarget?.status === col.id ? " column--over" : ""}`}
              onDragOver={(e) => onColumnDragOver(e, col.id)}
              onDrop={(e) => onColumnDrop(e, col.id)}
            >
              <div className="column-head">
                <span>{col.title}</span>
                <span className="column-count">{col.cards.length}</span>
              </div>
              <div className="column-body">
                {col.cards.map((card) => (
                  <div key={card.id}>
                    {dropTarget?.status === col.id && dropTarget.beforeId === card.id && <div className="drop-indicator" />}
                    <article
                      data-card-id={card.id}
                      className={`card${draggingId === card.id ? " card--dragging" : ""}${card.status === "done" ? " card--done" : ""}`}
                      draggable
                      onDragStart={(e) => onDragStart(e, card)}
                      onDragEnd={onDragEnd}
                      onClick={() => setModal({ card, status: card.status })}
                    >
                      <div className="card-title">{card.title}</div>
                      <div className="card-footer">
                        <span
                          className="priority"
                          style={{ color: PRIORITY_ICON[card.priority].color }}
                          title={`Пріоритет: ${PRIORITY_ICON[card.priority].label}`}
                        >
                          {PRIORITY_ICON[card.priority].icon}
                        </span>
                        <span className="card-key">{card.key}</span>
                        <div className="spacer" />
                        <Avatar user={card.assigneeId ? membersById.get(card.assigneeId) : null} size={24} />
                      </div>
                    </article>
                  </div>
                ))}
                {dropTarget?.status === col.id && dropTarget.beforeId === null && <div className="drop-indicator" />}
                <button className="add-card" onClick={() => setModal({ card: null, status: col.id })}>
                  + Створити задачу
                </button>
              </div>
            </section>
          ))}
        </div>
      )}

      {modal && (
        <CardModal
          key={modal.card?.id ?? "new"}
          card={modal.card}
          initialStatus={modal.status}
          users={orderedMembers}
          currentUser={me}
          canDelete={!!modal.card && (modal.card.authorId === me.id || project.role === "owner")}
          onClose={() => setModal(null)}
          onSave={saveCard}
          onDelete={deleteCard}
        />
      )}

      {settingsOpen && (
        <ProjectSettingsModal
          project={project}
          members={members}
          currentUserId={me.id}
          onClose={() => setSettingsOpen(false)}
          onMembersChanged={onMembersChanged}
          onProjectChanged={reloadProjects}
          onLeftOrDeleted={async () => {
            await reloadProjects();
            navigate("/projects", { replace: true });
          }}
        />
      )}
    </div>
  );
}
