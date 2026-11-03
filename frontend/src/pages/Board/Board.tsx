import { lazy, Suspense, useCallback, useEffect, useMemo, useRef, useState, type DragEvent } from "react";
import { useTranslation } from "react-i18next";
import { Navigate, useNavigate, useParams, useSearchParams } from "react-router-dom";
import { AddColumn } from "@/components/AddColumn/AddColumn";
import { BoardColumn, type ColumnState } from "@/components/BoardColumn/BoardColumn";
import { LiveCursors } from "@/components/LiveCursors/LiveCursors";
import { PageHeader } from "@/components/PageHeader/PageHeader";
import { ProjectSettingsModal } from "@/components/ProjectSettingsModal/ProjectSettingsModal";
import { ProjectTabs } from "@/components/ProjectTabs/ProjectTabs";
import { TeamFilter } from "@/components/TeamFilter/TeamFilter";
import { Banner } from "@/components/ui/Banner/Banner";
import { Button } from "@/components/ui/Button/Button";
import { Center } from "@/components/ui/Center/Center";
import { ModalBackdrop } from "@/components/ui/Modal/Modal";
import { Muted } from "@/components/ui/Muted/Muted";
import { Spinner } from "@/components/ui/Spinner/Spinner";
import { VoiceDock } from "@/components/VoiceDock/VoiceDock";
import { useAuth } from "@/context/auth";
import { useBoardRealtime } from "@/hooks/useBoardRealtime";
import { useDebounced } from "@/hooks/useDebounced";
import { useLayout } from "@/layouts/AppLayout/useLayout";
import { api } from "@/lib/api";
import { tasksApi } from "@/lib/api/tasks";
import { cx } from "@/lib/cx";
import { anchorAt, type BoardEvent } from "@/lib/realtime";
import type { Attachment, Card, CardCover, CardFilters, CardInput, Column, Member, Project, Tag } from "@/types";
import type { Planning } from "@/types/tasks";
import s from "./Board.module.scss";

// The editor (TipTap) and CRDT (Yjs) are heavy: load them when a task is first opened.
const CardModal = lazy(() => import("@/components/CardModal/CardModal").then((m) => ({ default: m.CardModal })));

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
  const [notice, setNotice] = useState("");
  const [search, setSearch] = useState("");
  const [selectedUsers, setSelectedUsers] = useState<Set<number>>(new Set());
  const [showUnassigned, setShowUnassigned] = useState(false);
  const [planFilter, setPlanFilter] = useState<{ milestone: number | null; sprint: number | null; version: number | null; tag: number | null }>({
    milestone: null, sprint: null, version: null, tag: null,
  });
  const [planning, setPlanning] = useState<Planning>({ milestones: [], sprints: [], versions: [] });
  const [tags, setTags] = useState<Tag[]>([]);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [modal, setModal] = useState<ModalState>(null);
  // Deep link to a task (?card=ID), e.g. from the backlog or time tracking.
  const [params, setParams] = useSearchParams();
  const linkedCard = Number(params.get("card")) || null;
  useEffect(() => {
    if (!linkedCard) return;
    api
      .card(linkedCard)
      .then((card) => card.projectId === project.id && setModal({ cardId: card.id, columnId: card.columnId, snapshot: card }))
      .catch(() => {});
    setParams({}, { replace: true });
  }, [linkedCard, project.id, setParams]);
  const [draggingId, setDraggingId] = useState<number | null>(null);
  const [dropTarget, setDropTarget] = useState<DropTarget>(null);
  const boardRef = useRef<HTMLDivElement>(null);
  const lastPointer = useRef(0);
  const trailingPointer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Latest columns state for async callbacks; generation discards responses from stale filter sets.
  const colsRef = useRef(cols);
  colsRef.current = cols;
  const generation = useRef(0);

  const debouncedSearch = useDebounced(search, 300);
  const filters: CardFilters = useMemo(
    () => ({
      q: debouncedSearch, assignees: [...selectedUsers], unassigned: showUnassigned,
      milestone: planFilter.milestone, sprint: planFilter.sprint, version: planFilter.version, tags: planFilter.tag ? [planFilter.tag] : [],
    }),
    [debouncedSearch, selectedUsers, showUnassigned, planFilter],
  );

  // Milestones, sprints, versions and tags for the filters (and the task editor).
  const loadPlanning = useCallback(() => {
    tasksApi.planning(project.id).then(setPlanning).catch(() => {});
    tasksApi.tags(project.id).then(setTags).catch(() => {});
  }, [project.id]);
  useEffect(loadPlanning, [loadPlanning]);
  const filtersRef = useRef(filters);
  filtersRef.current = filters;

  const patchCol = useCallback((columnId: number, patch: (col: ColumnState) => Partial<ColumnState>) => {
    setCols((prev) => prev.map((col) => (col.column.id === columnId ? { ...col, ...patch(col) } : col)));
  }, []);

  const fetchPage = useCallback(
    async (columnId: number, cursor: string | null) => {
      const gen = generation.current;
      patchCol(columnId, () => ({ loading: true, error: "" }));
      try {
        const page = await api.cards(project.id, columnId, filtersRef.current, cursor, PAGE_SIZE);
        if (gen !== generation.current) return;
        patchCol(columnId, (col) => {
          // Dedupe: a card moved here optimistically may also arrive in a later page.
          const seen = new Set(col.cards.map((c) => c.id));
          return {
            cards: cursor ? [...col.cards, ...page.cards.filter((c) => !seen.has(c.id))] : page.cards,
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
      const col = colsRef.current.find((c) => c.column.id === columnId);
      if (!col || col.loading) return;
      if (col.error || col.cursor) fetchPage(columnId, col.cursor);
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

  const membersById = useMemo(() => new Map(members.map((u) => [u.id, u])), [members]);
  const columns = useMemo(() => cols.map((col) => col.column), [cols]);

  // ---------- real time ----------

  const { socket, peers, live, connected, clearDrag } = useBoardRealtime(project.id, handleEvent);
  const findCard = useCallback((id: number) => {
    for (const col of colsRef.current) {
      const c = col.cards.find((x) => x.id === id);
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
    // Resolve now: the event'col target and coordinates are only valid during the event.
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
    setCols((prev) => prev.map((col) => (col.cards.some((c) => c.id === card.id) ? { ...col, cards: col.cards.map((c) => (c.id === card.id ? card : c)) } : col)));
  }

  /** Places a card someone else moved among the cards loaded here, using its new neighbours. */
  function applyRemoteMove(e: BoardEvent) {
    const card = e.card!;
    setCols((prev) =>
      prev.map((col) => {
        let cards = col.cards.filter((c) => c.id !== card.id);
        let total = col.total;
        if (col.column.id === e.fromColumnId) total -= 1;
        if (col.column.id === card.columnId) {
          total += 1;
          let idx = -1;
          if (e.afterId != null) {
            const i = cards.findIndex((c) => c.id === e.afterId);
            if (i >= 0) idx = i + 1;
          }
          if (idx < 0 && e.beforeId != null) idx = cards.findIndex((c) => c.id === e.beforeId);
          if (idx < 0 && e.afterId == null) idx = 0; // moved to the top
          if (idx < 0 && e.beforeId == null && col.cursor === null) idx = cards.length; // bottom of a fully loaded column
          if (idx >= 0) cards = [...cards.slice(0, idx), card, ...cards.slice(idx)];
        }
        return cards === col.cards && total === col.total ? col : { ...col, cards, total: Math.max(0, total) };
      }),
    );
  }

  /**
   * Committed board changes arrive here — from other people and from our own other tabs. Only the
   * tab that made a change skips it (it already shows it); that'col told by the connection, not the user.
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
        patchCol(e.card.columnId, (col) =>
          col.cards.some((c) => c.id === e.card!.id) ? {} : { cards: col.cursor === null ? [...col.cards, e.card!] : col.cards, total: col.total + 1 },
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
        patchCol(e.columnId!, (col) => ({ cards: col.cards.filter((c) => c.id !== e.cardId), total: Math.max(0, col.total - 1) }));
        break;
      case "columns.changed":
        if (!mine) reset();
        loadPlanning(); // planning or tags may have changed too
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

  // Current user first, then everyone else alphabetically — like Jira'col avatar group.
  const orderedMembers = useMemo(
    () => [...members].sort((a, b) => (a.id === me.id ? -1 : b.id === me.id ? 1 : a.name.localeCompare(b.name))),
    [members, me.id],
  );

  const hasFilters =
    selectedUsers.size > 0 || showUnassigned || search.trim() !== "" || Object.values(planFilter).some((v) => v !== null);
  const totalVisible = cols.reduce((n, col) => n + col.total, 0);

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
    setPlanFilter({ milestone: null, sprint: null, version: null, tag: null });
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
      const byId = new Map(prev.map((col) => [col.column.id, col]));
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

    const source = cols.find((col) => col.cards.some((c) => c.id === id));
    const target = cols.find((col) => col.column.id === columnId);
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
      prev.map((col) => {
        let cards = col.cards;
        let total = col.total;
        if (col.column.id === source.column.id) {
          cards = cards.filter((c) => c.id !== id);
          total -= 1;
        }
        if (col.column.id === columnId) {
          const found = beforeId === null ? -1 : cards.findIndex((c) => c.id === beforeId);
          const idx = found === -1 ? cards.length : found;
          cards = [...cards.slice(0, idx), moved, ...cards.slice(idx)];
          total += 1;
        }
        return { ...col, cards, total };
      }),
    );

    try {
      const saved = await api.moveCard(id, columnId, place);
      patchCol(columnId, (col) => ({ cards: col.cards.map((c) => (c.id === id ? saved : c)) }));
    } catch (err) {
      setError((err as Error).message);
      reset();
    }
  }

  // ---------- CRUD ----------

  async function createCard(input: CardInput, columnId: number) {
    const created = await api.createCard(project.id, { ...input, columnId });
    // New cards go to the bottom: show it only if the column is fully loaded.
    patchCol(columnId, (col) => ({ cards: col.cursor === null ? [...col.cards, created] : col.cards, total: col.total + 1 }));
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
    patchCol(card.columnId, (col) => ({ cards: col.cards.filter((c) => c.id !== card.id), total: col.total - 1 }));
    patchCol(columnId, (col) => ({ cards: col.cursor === null ? [...col.cards, saved] : col.cards, total: col.total + 1 }));
    setModal((m) => (m && m.cardId === card.id ? { ...m, columnId, snapshot: saved } : m));
  }

  /** Keeps the card face (file count, cover) in sync with what the modal uploaded or deleted. */
  function onAttachmentsChange(card: Card, attachments: Attachment[]) {
    const media = attachments.find((a) => a.kind !== "file");
    const cover: CardCover | null = media ? { kind: media.kind, url: media.kind === "image" ? (media.thumbUrl ?? media.url) : media.url } : null;
    patchCol(card.columnId, (col) => ({
      cards: col.cards.map((c) => (c.id === card.id ? { ...c, attachmentCount: attachments.length, cover } : c)),
    }));
  }

  async function deleteCard(card: Card) {
    await api.deleteCard(card.id);
    patchCol(card.columnId, (col) => ({ cards: col.cards.filter((c) => c.id !== card.id), total: col.total - 1 }));
    reloadProjects();
    reloadMembers();
  }

  // ---------- render ----------

  const modalCard = modal?.cardId ? (findCard(modal.cardId) ?? modal.snapshot) : null;
  const peersHere = modal?.cardId
    ? [...new Set(peers.filter((p) => p.cardId === modal.cardId).map((p) => p.userId))].flatMap((id) => membersById.get(id) ?? [])
    : [];
  const onlyMine = selectedUsers.size === 1 && selectedUsers.has(me.id);

  return (
    <div
      className={s.board}
      ref={boardRef}
      onPointerMove={(e) => sendPointer("cursor", e)}
      onPointerLeave={() => {
        if (trailingPointer.current) clearTimeout(trailingPointer.current);
        trailingPointer.current = null;
        socket?.send({ type: "cursor", anchor: null });
      }}
      onDragOver={(e) => draggingId !== null && sendPointer("drag", e)}
    >
      <PageHeader
        breadcrumbs={t("board.breadcrumbs", { name: project.name })}
        title={t("board.title", { key: project.key })}
        actions={
          <>
            <Button onClick={() => setSettingsOpen(true)}>{t("board.membersAndSettings")}</Button>
            <Button variant="primary" disabled={columns.length === 0} onClick={() => setModal({ cardId: null, columnId: columns[0]!.id, snapshot: null })}>
              {t("board.create")}
            </Button>
          </>
        }
      />

      <ProjectTabs project={project} />

      <div className={s.toolbar}>
        <input className={s.search} placeholder={t("board.search")} value={search} onChange={(e) => setSearch(e.target.value)} />

        <TeamFilter
          members={orderedMembers}
          meId={me.id}
          onlineIds={onlineIds}
          selected={selectedUsers}
          showUnassigned={showUnassigned}
          onToggleUser={toggleUser}
          onToggleUnassigned={() => setShowUnassigned((v) => !v)}
          onAddPeople={isOwner ? () => setSettingsOpen(true) : undefined}
        />

        <Button variant={onlyMine ? "selected" : "ghost"} size="sm" onClick={() => setSelectedUsers(onlyMine ? new Set() : new Set([me.id]))}>
          {t("board.onlyMine")}
        </Button>
        <PlanFilter
          label={t("tasks.anyMilestone")}
          value={planFilter.milestone}
          options={planning.milestones}
          onChange={(milestone) => setPlanFilter((f) => ({ ...f, milestone }))}
        />
        <PlanFilter
          label={t("tasks.anySprint")}
          value={planFilter.sprint}
          options={planning.sprints.filter((x) => x.status !== "past" || x.id === planFilter.sprint)}
          onChange={(sprint) => setPlanFilter((f) => ({ ...f, sprint }))}
        />
        <PlanFilter
          label={t("tasks.anyVersion")}
          value={planFilter.version}
          options={planning.versions.filter((x) => x.status !== "archived" || x.id === planFilter.version)}
          onChange={(version) => setPlanFilter((f) => ({ ...f, version }))}
        />
        <PlanFilter label={t("tasks.anyTag")} value={planFilter.tag} options={tags} onChange={(tag) => setPlanFilter((f) => ({ ...f, tag }))} />
        {hasFilters && (
          <>
            <Muted>{t("board.found", { count: totalVisible })}</Muted>
            <Button variant="link" size="sm" onClick={clearFilters}>
              {t("board.clearFilters")}
            </Button>
          </>
        )}
      </div>

      {error && <Banner onDismiss={() => setError("")}>{error}</Banner>}
      {notice && (
        <Banner tone="success" onDismiss={() => setNotice("")}>
          {notice}
        </Banner>
      )}

      {booting ? (
        <Center>
          <Spinner /> {t("board.loading")}
        </Center>
      ) : (
        // data-board-columns: the touch drag polyfill auto-scrolls this row (lib/touch.ts).
        <div className={cx(s.columns, draggingId !== null && s.dragging)} data-board-columns>
          {cols.map((col, i) => (
            <BoardColumn
              key={col.column.id}
              state={col}
              index={i}
              allColumns={columns}
              isOwner={isOwner}
              draggingId={draggingId}
              dropBeforeId={dropTarget?.columnId === col.column.id ? dropTarget.beforeId : undefined}
              membersById={membersById}
              onDragOver={(e) => onColumnDragOver(e, col.column.id)}
              onDrop={(e) => onColumnDrop(e, col.column.id)}
              onCardDragStart={onDragStart}
              onCardDragEnd={onDragEnd}
              onCardClick={(card) => setModal({ cardId: card.id, columnId: card.columnId, snapshot: card })}
              onAddCard={() => setModal({ cardId: null, columnId: col.column.id, snapshot: null })}
              remoteDragIds={remoteDragIds}
              onLoadMore={() => loadMore(col.column.id)}
              onRename={(name) => columnAction(() => api.renameColumn(col.column.id, name))}
              onMove={(index) => columnAction(() => api.moveColumn(col.column.id, index))}
              onDelete={(moveTo) => columnAction(() => api.deleteColumn(col.column.id, moveTo), true)}
              planning={planning}
              onBulkChanged={(message, failed) => {
                if (message) (failed ? setError : setNotice)(message);
                reset();
                loadPlanning();
              }}
            />
          ))}
          {isOwner && <AddColumn onAdd={(name) => columnAction(() => api.createColumn(project.id, name))} />}
        </div>
      )}

      {modal && (
        <Suspense fallback={<ModalBackdrop onClose={() => setModal(null)} />}>
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
            planning={planning}
            tags={tags}
            onCardChanged={replaceCard}
            onCardAdded={(card) =>
              patchCol(card.columnId, (col) => ({ cards: col.cursor === null ? [...col.cards, card] : col.cards, total: col.total + 1 }))
            }
            onCardRemoved={(card) =>
              patchCol(card.columnId, (col) => ({ cards: col.cards.filter((c) => c.id !== card.id), total: Math.max(0, col.total - 1) }))
            }
          />
        </Suspense>
      )}

      <LiveCursors boardRef={boardRef} peers={peers} live={live} usersById={membersById} findCard={findCard} />
      <VoiceDock projectId={project.id} me={me} usersById={membersById} />

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

/** A compact select for one planning filter; hidden when the project has none of that kind. */
function PlanFilter({ label, value, options, onChange }: {
  label: string;
  value: number | null;
  options: { id: number; name: string }[];
  onChange: (id: number | null) => void;
}) {
  if (options.length === 0 && value === null) return null;
  return (
    <select className={s.filter} value={value ?? ""} aria-label={label} onChange={(e) => onChange(e.target.value ? Number(e.target.value) : null)}>
      <option value="">{label}</option>
      {options.map((o) => (
        <option key={o.id} value={o.id}>
          {o.name}
        </option>
      ))}
    </select>
  );
}
