import { lazy, Suspense, useCallback, useEffect, useMemo, useRef, useState, type DragEvent } from "react";
import { useTranslation } from "react-i18next";
import { Navigate, useNavigate, useParams } from "react-router-dom";
import { api } from "../api";
import { useAuth } from "../auth";
import { Avatar } from "../components/Avatar";
import { AddColumn, BoardColumn, type ColumnState } from "../components/BoardColumn";
import { LiveCursors } from "../components/LiveCursors";
import { ProjectSettingsModal } from "../components/ProjectModals";
import { anchorAt, type BoardEvent } from "../realtime";
import type { Attachment, Card, CardCover, CardFilters, CardInput, Column, Member, Project } from "../types";
import { useBoardRealtime } from "../useBoardRealtime";
import { useLayout } from "./AppLayout";

// The editor (TipTap) and CRDT (Yjs) are heavy: load them when a task is first opened.
const CardModal = lazy(() => import("../components/CardModal").then((m) => ({ default: m.CardModal })));

const VISIBLE_AVATARS = 6;
const PAGE_SIZE = 20;

// cardId null = creating a new card. The card itself is looked up live, so edits by others show up.
type ModalState = { cardId: number | null; columnId: number; snapshot: Card | null } | null;
const POINTER_INTERVAL = 40; // ms between cursor messages
type DropTarget = { columnId: number; beforeId: number | null } | null;

/** Route wrapper: remounts the board when switching projects so all state resets. */
export default function BoardRoute() {
  const { projectId } = useParams();
  const { projects } = useLayout();
  const project = projects.find((p) => p.id === Number(projectId));
  if (!project) return <Navigate to="/home" replace />;
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
  const { t } = useTranslation();
  const { user } = useAuth();
  const { reload: reloadProjects } = useLayout();
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
  const boardRef = useRef<HTMLDivElement>(null);
  const lastPointer = useRef(0);
  const trailingPointer = useRef<ReturnType<typeof setTimeout> | null>(null);

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
    reloadMembers();
  }, [project.id, reloadMembers, user?.avatarUrl]);

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

  // ---------- real time ----------

  const { socket, peers, live, connected, clearDrag } = useBoardRealtime(project.id, handleEvent);
  const findCard = useCallback((id: number) => {
    for (const s of colsRef.current) {
      const c = s.cards.find((x) => x.id === id);
      if (c) return c;
    }
    return undefined;
  }, []);
  const onlineIds = useMemo(() => new Set([me.id, ...peers.map((p) => p.userId)]), [peers, me.id]);
  const remoteDragIds = useMemo(() => new Set(peers.flatMap((p) => (p.drag ? [p.drag.cardId] : []))), [peers]);

  // Tell the others which task we have open (re-sent after a reconnect).
  const focusCardId = modal?.cardId ?? null;
  useEffect(() => {
    if (connected) socket?.send({ type: "focus", cardId: focusCardId });
  }, [socket, connected, focusCardId]);

  /** Throttled, but the last position is always sent, so a cursor never stops short of where the pointer did. */
  function sendPointer(type: "cursor" | "drag", e: { target: EventTarget | null; clientX: number; clientY: number }) {
    const board = boardRef.current;
    if (!board || !socket) return;
    // Resolve now: the event's target and coordinates are only valid during the event.
    const anchor = anchorAt(e.target as Element | null, e.clientX, e.clientY, board);
    const msg = type === "drag" ? { type, cardId: draggingId, anchor } : { type, anchor };
    const send = () => {
      lastPointer.current = performance.now();
      trailingPointer.current = null;
      socket.send(msg);
    };
    if (trailingPointer.current) clearTimeout(trailingPointer.current);
    const wait = POINTER_INTERVAL - (performance.now() - lastPointer.current);
    if (wait <= 0) send();
    else trailingPointer.current = setTimeout(send, wait);
  }

  function replaceCard(card: Card) {
    setCols((prev) => prev.map((s) => (s.cards.some((c) => c.id === card.id) ? { ...s, cards: s.cards.map((c) => (c.id === card.id ? card : c)) } : s)));
  }

  /** Places a card someone else moved among the cards loaded here, using its new neighbours. */
  function applyRemoteMove(e: BoardEvent) {
    const card = e.card!;
    setCols((prev) =>
      prev.map((s) => {
        let cards = s.cards.filter((c) => c.id !== card.id);
        let total = s.total;
        if (s.column.id === e.fromColumnId) total -= 1;
        if (s.column.id === card.columnId) {
          total += 1;
          let idx = -1;
          if (e.afterId != null) {
            const i = cards.findIndex((c) => c.id === e.afterId);
            if (i >= 0) idx = i + 1;
          }
          if (idx < 0 && e.beforeId != null) idx = cards.findIndex((c) => c.id === e.beforeId);
          if (idx < 0 && e.afterId == null) idx = 0; // moved to the top
          if (idx < 0 && e.beforeId == null && s.cursor === null) idx = cards.length; // bottom of a fully loaded column
          if (idx >= 0) cards = [...cards.slice(0, idx), card, ...cards.slice(idx)];
        }
        return cards === s.cards && total === s.total ? s : { ...s, cards, total: Math.max(0, total) };
      }),
    );
  }

  /**
   * Committed board changes arrive here — from other people and from our own other tabs. Only the
   * tab that made a change skips it (it already shows it); that's told by the connection, not the user.
   */
  function handleEvent(e: BoardEvent) {
    const mine = !!e.origin && e.origin === socket?.connId;
    switch (e.type) {
      case "card.updated":
        if (e.card) replaceCard(e.card);
        break;
      case "card.created":
        if (mine || !e.card) break;
        if (hasFilters) return void reset();
        patchCol(e.card.columnId, (s) =>
          s.cards.some((c) => c.id === e.card!.id) ? {} : { cards: s.cursor === null ? [...s.cards, e.card!] : s.cards, total: s.total + 1 },
        );
        break;
      case "card.moved":
        if (e.card) clearDrag(e.card.id); // the drop happened: no more ghost, whatever messages got lost
        if (mine || !e.card) break;
        if (hasFilters) return void reset();
        applyRemoteMove(e);
        break;
      case "card.deleted":
        if (modal?.cardId === e.cardId) setModal(null);
        if (mine) break;
        patchCol(e.columnId!, (s) => ({ cards: s.cards.filter((c) => c.id !== e.cardId), total: Math.max(0, s.total - 1) }));
        break;
      case "columns.changed":
        if (!mine) reset();
        break;
      case "members.changed":
        reloadMembers();
        if (!mine) {
          reloadProjects(); // if we were the one removed, the project disappears and we leave the board
          reset();
        }
        break;
      case "project.deleted":
        reloadProjects();
        break;
    }
  }

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
    // A throttled drag position may still be pending; it must not re-announce the drag after it ended.
    if (trailingPointer.current) clearTimeout(trailingPointer.current);
    trailingPointer.current = null;
    socket?.send({ type: "drag", cardId: null });
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

  async function createCard(input: CardInput, columnId: number) {
    const created = await api.createCard(project.id, { ...input, columnId });
    // New cards go to the bottom: show it only if the column is fully loaded.
    patchCol(columnId, (s) => ({ cards: s.cursor === null ? [...s.cards, created] : s.cards, total: s.total + 1 }));
    reloadProjects();
    reloadMembers();
  }

  async function updateCard(card: Card, input: CardInput) {
    const saved = await api.updateCard(card.id, input);
    replaceCard(saved);
    if (input.assigneeId !== card.assigneeId) reloadMembers();
  }

  async function moveCardTo(card: Card, columnId: number) {
    if (columnId === card.columnId) return;
    const saved = await api.moveCard(card.id, columnId);
    patchCol(card.columnId, (s) => ({ cards: s.cards.filter((c) => c.id !== card.id), total: s.total - 1 }));
    patchCol(columnId, (s) => ({ cards: s.cursor === null ? [...s.cards, saved] : s.cards, total: s.total + 1 }));
    setModal((m) => (m && m.cardId === card.id ? { ...m, columnId, snapshot: saved } : m));
  }

  /** Keeps the card face (file count, cover) in sync with what the modal uploaded or deleted. */
  function onAttachmentsChange(card: Card, attachments: Attachment[]) {
    const media = attachments.find((a) => a.kind !== "file");
    const cover: CardCover | null = media ? { kind: media.kind, url: media.kind === "image" ? (media.thumbUrl ?? media.url) : media.url } : null;
    patchCol(card.columnId, (s) => ({
      cards: s.cards.map((c) => (c.id === card.id ? { ...c, attachmentCount: attachments.length, cover } : c)),
    }));
  }

  async function deleteCard(card: Card) {
    await api.deleteCard(card.id);
    patchCol(card.columnId, (s) => ({ cards: s.cards.filter((c) => c.id !== card.id), total: s.total - 1 }));
    reloadProjects();
    reloadMembers();
  }

  // ---------- render ----------

  const overflow = orderedMembers.length - VISIBLE_AVATARS;
  const modalCard = modal?.cardId ? (findCard(modal.cardId) ?? modal.snapshot) : null;
  const peersHere = modal?.cardId
    ? [...new Set(peers.filter((p) => p.cardId === modal.cardId).map((p) => p.userId))].flatMap((id) => membersById.get(id) ?? [])
    : [];
  const onlyMine = selectedUsers.size === 1 && selectedUsers.has(me.id);

  return (
    <div
      className="board"
      ref={boardRef}
      onPointerMove={(e) => sendPointer("cursor", e)}
      onPointerLeave={() => {
        if (trailingPointer.current) clearTimeout(trailingPointer.current);
        trailingPointer.current = null;
        socket?.send({ type: "cursor", anchor: null });
      }}
      onDragOver={(e) => draggingId !== null && sendPointer("drag", e)}
    >
      <div className="board-head">
        <div>
          <div className="breadcrumbs">{t("board.breadcrumbs", { name: project.name })}</div>
          <h1>{t("board.title", { key: project.key })}</h1>
        </div>
        <div className="board-head-actions">
          <button className="btn btn-ghost" onClick={() => setSettingsOpen(true)}>
            {t("board.membersAndSettings")}
          </button>
          <button
            className="btn btn-primary"
            disabled={columns.length === 0}
            onClick={() => setModal({ cardId: null, columnId: columns[0]!.id, snapshot: null })}
          >
            {t("board.create")}
          </button>
        </div>
      </div>

      <div className="toolbar">
        <input className="search" placeholder={t("board.search")} value={search} onChange={(e) => setSearch(e.target.value)} />

        <div className="avatar-group">
          {orderedMembers.slice(0, VISIBLE_AVATARS).map((u) => (
            <Avatar
              key={u.id}
              user={u}
              size={32}
              active={selectedUsers.has(u.id)}
              online={onlineIds.has(u.id)}
              onClick={() => toggleUser(u.id)}
            />
          ))}
          <div className="team-dropdown" ref={teamRef}>
            <button
              className={`avatar avatar--clickable avatar--more${teamOpen ? " avatar--active" : ""}`}
              style={{ width: 32, height: 32 }}
              onClick={() => setTeamOpen((v) => !v)}
              title={t("board.allMembers")}
            >
              {overflow > 0 ? `+${overflow}` : "☰"}
            </button>
            {teamOpen && (
              <div className="menu team-menu">
                <div className="team-menu-title">{t("board.members", { count: members.length })}</div>
                {orderedMembers.map((u) => (
                  <label key={u.id} className="team-row">
                    <input type="checkbox" checked={selectedUsers.has(u.id)} onChange={() => toggleUser(u.id)} />
                    <Avatar user={u} size={28} online={onlineIds.has(u.id)} />
                    <span className="team-row-name">
                      <span>
                        {u.name}
                        {u.id === me.id && <em> {t("common.you")}</em>}
                        {u.role === "owner" && <em> · {t("board.owner")}</em>}
                      </span>
                      <small>{u.email}</small>
                    </span>
                    <span className="badge" title={t("board.assignedTasks")}>
                      {u.assignedCount}
                    </span>
                  </label>
                ))}
                <label className="team-row">
                  <input type="checkbox" checked={showUnassigned} onChange={() => setShowUnassigned((v) => !v)} />
                  <Avatar user={null} size={28} />
                  <span className="team-row-name">{t("common.unassigned")}</span>
                </label>
                {isOwner && (
                  <button
                    className="team-menu-add"
                    onClick={() => {
                      setTeamOpen(false);
                      setSettingsOpen(true);
                    }}
                  >
                    {t("board.addPeople")}
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
          {t("board.onlyMine")}
        </button>
        {hasFilters && (
          <>
            <span className="muted">{t("board.found", { count: totalVisible })}</span>
            <button className="btn btn-link btn-sm" onClick={clearFilters}>
              {t("board.clearFilters")}
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
          <span className="spinner" /> {t("board.loading")}
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
              onCardClick={(card) => setModal({ cardId: card.id, columnId: card.columnId, snapshot: card })}
              onAddCard={() => setModal({ cardId: null, columnId: s.column.id, snapshot: null })}
              remoteDragIds={remoteDragIds}
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
        <Suspense fallback={<div className="modal-backdrop" />}>
        <CardModal
          key={modal.cardId ?? "new"}
          card={modalCard}
          initialColumnId={modal.columnId}
          columns={columns}
          users={orderedMembers}
          currentUser={me}
          canDelete={!!modalCard && (modalCard.authorId === me.id || isOwner)}
          isProjectOwner={isOwner}
          socket={socket}
          peersHere={peersHere}
          onAttachmentsChange={onAttachmentsChange}
          onClose={() => setModal(null)}
          onCreate={createCard}
          onUpdate={updateCard}
          onMove={moveCardTo}
          onDelete={deleteCard}
        />
        </Suspense>
      )}

      <LiveCursors boardRef={boardRef} peers={peers} live={live} usersById={membersById} findCard={findCard} />

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
            navigate("/home", { replace: true });
          }}
        />
      )}
    </div>
  );
}
