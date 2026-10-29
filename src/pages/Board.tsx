import { useCallback, useEffect, useMemo, useRef, useState, type DragEvent } from "react";
import { Navigate, useNavigate, useParams } from "react-router-dom";
import { api } from "../api";
import { useAuth } from "../auth";
import { Avatar } from "../components/Avatar";
import { AddColumn, BoardColumn, type ColumnState } from "../components/BoardColumn";
import { CardModal } from "../components/CardModal";
import { ProjectSettingsModal } from "../components/ProjectModals";
import type { Card, CardFilters, CardInput, Column, Member, Project } from "../types";
import { rememberProject, useLayout } from "./AppLayout";

const VISIBLE_AVATARS = 6;
const PAGE_SIZE = 20;

type ModalState = { card: Card | null; columnId: number } | null;
type DropTarget = { columnId: number; beforeId: number | null } | null;

/** Route wrapper: remounts the board when switching projects so all state resets. */
export default function BoardRoute() {
  const { projectId } = useParams();
  const { projects } = useLayout();
  const project = projects.find((p) => p.id === Number(projectId));
  if (!project) return <Navigate to="/projects" replace />;
  return <Board key={project.id} project={project} />;
}

function useDebounced<T>(value: T, ms: number) {
  const [v, setV] = useState(value);
  useEffect(() => {
    const t = setTimeout(() => setV(value), ms);
    return () => clearTimeout(t);
  }, [value, ms]);
  return v;
}

function Board({ project }: { project: Project }) {
  const { user } = useAuth();
  const { reloadProjects } = useLayout();
  const navigate = useNavigate();
  const me = user!;
  const isOwner = project.role === "owner";

  const [cols, setCols] = useState<ColumnState[]>([]);
  const [members, setMembers] = useState<Member[]>([]);
  const [booting, setBooting] = useState(true);
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

  // Latest columns state for async callbacks; generation discards responses from stale filter sets.
  const colsRef = useRef(cols);
  colsRef.current = cols;
  const generation = useRef(0);

  const debouncedSearch = useDebounced(search, 300);
  const filters: CardFilters = useMemo(
    () => ({ q: debouncedSearch, assignees: [...selectedUsers], unassigned: showUnassigned }),
    [debouncedSearch, selectedUsers, showUnassigned],
  );
  const filtersRef = useRef(filters);
  filtersRef.current = filters;

  const patchCol = useCallback((columnId: number, patch: (s: ColumnState) => Partial<ColumnState>) => {
    setCols((prev) => prev.map((s) => (s.column.id === columnId ? { ...s, ...patch(s) } : s)));
  }, []);

  const fetchPage = useCallback(
    async (columnId: number, cursor: string | null) => {
      const gen = generation.current;
      patchCol(columnId, () => ({ loading: true, error: "" }));
      try {
        const page = await api.cards(project.id, columnId, filtersRef.current, cursor, PAGE_SIZE);
        if (gen !== generation.current) return;
        patchCol(columnId, (s) => {
          // Dedupe: a card moved here optimistically may also arrive in a later page.
          const seen = new Set(s.cards.map((c) => c.id));
          return {
            cards: cursor ? [...s.cards, ...page.cards.filter((c) => !seen.has(c.id))] : page.cards,
            total: page.total,
            cursor: page.nextCursor,
            loading: false,
          };
        });
      } catch (err) {
        if (gen !== generation.current) return;
        patchCol(columnId, () => ({ loading: false, error: (err as Error).message }));
      }
    },
    [project.id, patchCol],
  );

  /** Reload columns and the first page of each column. */
  const reset = useCallback(async () => {
    const gen = ++generation.current;
    try {
      const columns = await api.columns(project.id);
      if (gen !== generation.current) return;
      setCols(columns.map((column) => ({ column, cards: [], total: 0, cursor: null, loading: true, error: "" })));
      setBooting(false);
      await Promise.all(columns.map((c) => fetchPage(c.id, null)));
    } catch (err) {
      setError((err as Error).message);
      setBooting(false);
    }
  }, [project.id, fetchPage]);

  const loadMore = useCallback(
    (columnId: number) => {
      const s = colsRef.current.find((c) => c.column.id === columnId);
      if (!s || s.loading) return;
      if (s.error || s.cursor) fetchPage(columnId, s.cursor);
    },
    [fetchPage],
  );

  const reloadMembers = useCallback(() => {
    api.members(project.id).then(setMembers).catch(() => {});
  }, [project.id]);

  useEffect(() => {
    rememberProject(project.id);
    reloadMembers();
  }, [project.id, reloadMembers]);

  useEffect(() => {
    reset();
  }, [reset, filters]);

  useEffect(() => {
    if (!teamOpen) return;
    const onClick = (e: MouseEvent) => {
      if (!teamRef.current?.contains(e.target as Node)) setTeamOpen(false);
    };
    document.addEventListener("mousedown", onClick);
    return () => document.removeEventListener("mousedown", onClick);
  }, [teamOpen]);

  const membersById = useMemo(() => new Map(members.map((u) => [u.id, u])), [members]);
  const columns = useMemo(() => cols.map((s) => s.column), [cols]);

  // Current user first, then everyone else alphabetically — like Jira's avatar group.
  const orderedMembers = useMemo(
    () => [...members].sort((a, b) => (a.id === me.id ? -1 : b.id === me.id ? 1 : a.name.localeCompare(b.name))),
    [members, me.id],
  );

  const hasFilters = selectedUsers.size > 0 || showUnassigned || search.trim() !== "";
  const totalVisible = cols.reduce((n, s) => n + s.total, 0);

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
    setSelectedUsers((prev) => new Set([...prev].filter((id) => next.some((m) => m.id === id))));
    setMembers(next);
    reloadProjects();
    reset(); // removed members' cards were unassigned on the server
  }

  // ---------- columns ----------

  /** Apply a column list from the server while keeping already loaded cards. */
  function applyColumns(next: Column[]) {
    setCols((prev) => {
      const byId = new Map(prev.map((s) => [s.column.id, s]));
      return next.map((column) => {
        const existing = byId.get(column.id);
        return existing
          ? { ...existing, column }
          : { column, cards: [], total: 0, cursor: null, loading: false, error: "" };
      });
    });
  }

  async function columnAction(action: () => Promise<Column[]>, fullReload = false) {
    try {
      const next = await action();
      if (fullReload) await reset();
      else applyColumns(next);
    } catch (err) {
      setError((err as Error).message);
    }
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

  function onColumnDragOver(e: DragEvent<HTMLElement>, columnId: number) {
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
    if (dropTarget?.columnId !== columnId || dropTarget.beforeId !== beforeId) {
      setDropTarget({ columnId, beforeId });
    }
  }

  async function onColumnDrop(e: DragEvent, columnId: number) {
    e.preventDefault();
    const id = draggingId;
    const beforeId = dropTarget?.columnId === columnId ? dropTarget.beforeId : null;
    onDragEnd();
    if (id === null) return;

    const source = cols.find((s) => s.cards.some((c) => c.id === id));
    const target = cols.find((s) => s.column.id === columnId);
    const card = source?.cards.find((c) => c.id === id);
    if (!source || !target || !card) return;

    // Dropping below the last *loaded* card of a partially loaded column means "after it", not "end of column".
    let place: { beforeId?: number; afterId?: number } = {};
    if (beforeId !== null) place = { beforeId };
    else if (target.cursor !== null) {
      const last = target.cards.filter((c) => c.id !== id).at(-1);
      if (last) place = { afterId: last.id };
    }

    const moved = { ...card, columnId };
    setCols((prev) =>
      prev.map((s) => {
        let cards = s.cards;
        let total = s.total;
        if (s.column.id === source.column.id) {
          cards = cards.filter((c) => c.id !== id);
          total -= 1;
        }
        if (s.column.id === columnId) {
          const found = beforeId === null ? -1 : cards.findIndex((c) => c.id === beforeId);
          const idx = found === -1 ? cards.length : found;
          cards = [...cards.slice(0, idx), moved, ...cards.slice(idx)];
          total += 1;
        }
        return { ...s, cards, total };
      }),
    );

    try {
      const saved = await api.moveCard(id, columnId, place);
      patchCol(columnId, (s) => ({ cards: s.cards.map((c) => (c.id === id ? saved : c)) }));
    } catch (err) {
      setError((err as Error).message);
      reset();
    }
  }

  // ---------- CRUD ----------

  async function saveCard(input: CardInput, columnId: number) {
    if (!modal) return;
    if (modal.card === null) {
      const created = await api.createCard(project.id, { ...input, columnId });
      // New cards go to the bottom: show it only if the column is fully loaded.
      patchCol(columnId, (s) => ({ cards: s.cursor === null ? [...s.cards, created] : s.cards, total: s.total + 1 }));
    } else {
      const old = modal.card;
      let saved = await api.updateCard(old.id, input);
      if (columnId !== old.columnId) {
        saved = await api.moveCard(old.id, columnId);
        patchCol(old.columnId, (s) => ({ cards: s.cards.filter((c) => c.id !== old.id), total: s.total - 1 }));
        patchCol(columnId, (s) => ({ cards: s.cursor === null ? [...s.cards, saved] : s.cards, total: s.total + 1 }));
      } else {
        patchCol(columnId, (s) => ({ cards: s.cards.map((c) => (c.id === old.id ? saved : c)) }));
      }
    }
    reloadProjects();
    reloadMembers();
  }

  async function deleteCard(card: Card) {
    await api.deleteCard(card.id);
    patchCol(card.columnId, (s) => ({ cards: s.cards.filter((c) => c.id !== card.id), total: s.total - 1 }));
    reloadProjects();
    reloadMembers();
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
          <button
            className="btn btn-primary"
            disabled={columns.length === 0}
            onClick={() => setModal({ card: null, columnId: columns[0]!.id })}
          >
            + Створити
          </button>
        </div>
      </div>

      <div className="toolbar">
        <input className="search" placeholder="Пошук по дошці" value={search} onChange={(e) => setSearch(e.target.value)} />

        <div className="avatar-group">
          {orderedMembers.slice(0, VISIBLE_AVATARS).map((u) => (
            <Avatar key={u.id} user={u} size={32} active={selectedUsers.has(u.id)} onClick={() => toggleUser(u.id)} />
          ))}
          <div className="team-dropdown" ref={teamRef}>
            <button
              className={`avatar avatar--clickable avatar--more${teamOpen ? " avatar--active" : ""}`}
              style={{ width: 32, height: 32 }}
              onClick={() => setTeamOpen((v) => !v)}
              title="Усі учасники проєкту"
            >
              {overflow > 0 ? `+${overflow}` : "☰"}
            </button>
            {teamOpen && (
              <div className="menu team-menu">
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
                    <span className="badge" title="Призначено задач">
                      {u.assignedCount}
                    </span>
                  </label>
                ))}
                <label className="team-row">
                  <input type="checkbox" checked={showUnassigned} onChange={() => setShowUnassigned((v) => !v)} />
                  <Avatar user={null} size={28} />
                  <span className="team-row-name">Не призначено</span>
                </label>
                {isOwner && (
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
          <>
            <span className="muted">Знайдено: {totalVisible}</span>
            <button className="btn btn-link btn-sm" onClick={clearFilters}>
              Очистити фільтри
            </button>
          </>
        )}
      </div>

      {error && (
        <div className="banner-error">
          {error} <button onClick={() => setError("")}>✕</button>
        </div>
      )}

      {booting ? (
        <div className="center">
          <span className="spinner" /> Завантаження дошки…
        </div>
      ) : (
        <div className="columns">
          {cols.map((s, i) => (
            <BoardColumn
              key={s.column.id}
              state={s}
              index={i}
              allColumns={columns}
              isOwner={isOwner}
              draggingId={draggingId}
              dropBeforeId={dropTarget?.columnId === s.column.id ? dropTarget.beforeId : undefined}
              membersById={membersById}
              onDragOver={(e) => onColumnDragOver(e, s.column.id)}
              onDrop={(e) => onColumnDrop(e, s.column.id)}
              onCardDragStart={onDragStart}
              onCardDragEnd={onDragEnd}
              onCardClick={(card) => setModal({ card, columnId: card.columnId })}
              onAddCard={() => setModal({ card: null, columnId: s.column.id })}
              onLoadMore={() => loadMore(s.column.id)}
              onRename={(name) => columnAction(() => api.renameColumn(s.column.id, name))}
              onMove={(index) => columnAction(() => api.moveColumn(s.column.id, index))}
              onDelete={(moveTo) => columnAction(() => api.deleteColumn(s.column.id, moveTo), true)}
            />
          ))}
          {isOwner && <AddColumn onAdd={(name) => columnAction(() => api.createColumn(project.id, name))} />}
        </div>
      )}

      {modal && (
        <CardModal
          key={modal.card?.id ?? "new"}
          card={modal.card}
          initialColumnId={modal.columnId}
          columns={columns}
          users={orderedMembers}
          currentUser={me}
          canDelete={!!modal.card && (modal.card.authorId === me.id || isOwner)}
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
