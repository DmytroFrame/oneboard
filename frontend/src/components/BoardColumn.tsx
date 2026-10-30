import { useEffect, useRef, useState, type DragEvent, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import type { Card, Column, User } from "../types";
import { PaperclipIcon } from "./Attachments";
import { Avatar } from "./Avatar";

export interface ColumnState {
  column: Column;
  cards: Card[];
  total: number; // matching the current filters
  cursor: string | null; // null = everything loaded
  loading: boolean;
  error: string;
}

export const PRIORITY_ICON: Record<Card["priority"], { icon: string; color: string }> = {
  low: { icon: "↓", color: "#30a7ff" },
  medium: { icon: "=", color: "#ff9f0a" },
  high: { icon: "↑", color: "#ff6b3d" },
  critical: { icon: "⇈", color: "#ff375f" },
};

interface Props {
  state: ColumnState;
  index: number;
  allColumns: Column[];
  isOwner: boolean;
  draggingId: number | null;
  remoteDragIds: Set<number>; // cards someone else is dragging right now
  dropBeforeId: number | null | undefined; // undefined = this column isn't the drop target
  membersById: Map<number, User>;
  onDragOver: (e: DragEvent<HTMLElement>) => void;
  onDrop: (e: DragEvent<HTMLElement>) => void;
  onCardDragStart: (e: DragEvent, card: Card) => void;
  onCardDragEnd: () => void;
  onCardClick: (card: Card) => void;
  onAddCard: () => void;
  onLoadMore: () => void;
  onRename: (name: string) => Promise<void>;
  onMove: (index: number) => Promise<void>;
  onDelete: (moveTo: number | null) => Promise<void>;
}

export function BoardColumn(props: Props) {
  const { t } = useTranslation();
  const { state, index, allColumns, isOwner, draggingId, dropBeforeId, membersById } = props;
  const bodyRef = useRef<HTMLDivElement>(null);
  const sentinelRef = useRef<HTMLDivElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const [menu, setMenu] = useState<"closed" | "open" | "delete">("closed");
  const [renaming, setRenaming] = useState(false);
  const [name, setName] = useState(state.column.name);
  const others = allColumns.filter((c) => c.id !== state.column.id);
  const [moveTo, setMoveTo] = useState<number | null>(null);
  const hasMore = state.cursor !== null;

  // Infinite scroll: load the next page when the sentinel at the bottom of the column comes into view.
  const { onLoadMore } = props;
  useEffect(() => {
    const el = sentinelRef.current;
    if (!el || !hasMore || state.loading) return;
    const io = new IntersectionObserver((entries) => entries[0]?.isIntersecting && onLoadMore(), {
      root: bodyRef.current,
      rootMargin: "200px",
    });
    io.observe(el);
    return () => io.disconnect();
  }, [hasMore, state.loading, state.cards.length, onLoadMore]);

  useEffect(() => {
    if (menu === "closed") return;
    const onClick = (e: MouseEvent) => {
      if (!menuRef.current?.contains(e.target as Node)) setMenu("closed");
    };
    document.addEventListener("mousedown", onClick);
    return () => document.removeEventListener("mousedown", onClick);
  }, [menu]);

  async function submitRename(e: FormEvent) {
    e.preventDefault();
    if (name.trim() && name.trim() !== state.column.name) await props.onRename(name.trim());
    setRenaming(false);
  }

  return (
    <section
      className={`column${dropBeforeId !== undefined ? " column--over" : ""}`}
      data-column-id={state.column.id}
      // Accepting dragenter too (not just dragover) makes the column the drop target per the HTML5
      // spec; desktop browsers are lenient, the touch drag polyfill is not.
      onDragEnter={props.onDragOver}
      onDragOver={props.onDragOver}
      onDrop={props.onDrop}
    >
      <div className="column-head">
        {renaming ? (
          <form onSubmit={submitRename} className="column-rename">
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              onBlur={submitRename}
              onKeyDown={(e) => e.key === "Escape" && (setName(state.column.name), setRenaming(false))}
              maxLength={60}
              autoFocus
            />
          </form>
        ) : (
          <span className="column-title" onDoubleClick={() => isOwner && setRenaming(true)}>
            {state.column.name}
          </span>
        )}
        <span className="column-count">{state.total}</span>
        <div className="spacer" />
        {isOwner && (
          <div className="column-menu-wrap" ref={menuRef}>
            <button className="icon-btn icon-btn--sm" onClick={() => setMenu(menu === "closed" ? "open" : "closed")} aria-label={t("column.actions")}>
              ⋯
            </button>
            {menu === "open" && (
              <div className="menu column-menu">
                <button onClick={() => (setMenu("closed"), setRenaming(true))}>{t("column.rename")}</button>
                <button disabled={index === 0} onClick={() => (setMenu("closed"), props.onMove(index - 1))}>
                  {t("column.moveLeft")}
                </button>
                <button disabled={index === allColumns.length - 1} onClick={() => (setMenu("closed"), props.onMove(index + 1))}>
                  {t("column.moveRight")}
                </button>
                <button
                  className="menu-danger"
                  disabled={others.length === 0}
                  onClick={() => {
                    setMoveTo(others[Math.max(0, index - 1)]?.id ?? null);
                    setMenu("delete");
                  }}
                >
                  {t("column.delete")}
                </button>
              </div>
            )}
            {menu === "delete" && (
              <div className="menu column-menu column-menu--form">
                <strong>{t("column.deleteConfirm", { name: state.column.name })}</strong>
                <label>
                  {t("column.moveCardsTo")}
                  <select value={moveTo ?? ""} onChange={(e) => setMoveTo(Number(e.target.value))}>
                    {others.map((c) => (
                      <option key={c.id} value={c.id}>
                        {c.name}
                      </option>
                    ))}
                  </select>
                </label>
                <div className="menu-actions">
                  <button className="btn btn-ghost btn-sm" onClick={() => setMenu("closed")}>
                    {t("common.cancel")}
                  </button>
                  <button className="btn btn-danger-solid btn-sm" onClick={() => (setMenu("closed"), props.onDelete(moveTo))}>
                    {t("common.delete")}
                  </button>
                </div>
              </div>
            )}
          </div>
        )}
      </div>

      <div className="column-body" ref={bodyRef}>
        {state.cards.map((card) => (
          <div key={card.id} className="card-slot">
            {dropBeforeId === card.id && <div className="drop-indicator" />}
            <article
              data-card-id={card.id}
              className={`card${draggingId === card.id ? " card--dragging" : ""}${props.remoteDragIds.has(card.id) ? " card--remote-drag" : ""}`}
              draggable
              onDragStart={(e) => props.onCardDragStart(e, card)}
              onDragEnd={props.onCardDragEnd}
              onClick={() => props.onCardClick(card)}
            >
              {card.cover && (
                <div className={`card-cover card-cover--${card.cover.kind}`}>
                  {card.cover.kind === "image" ? (
                    <img src={card.cover.url} alt="" loading="lazy" draggable={false} />
                  ) : (
                    <>
                      <video src={`${card.cover.url}#t=0.1`} preload="metadata" muted playsInline />
                      <span className="play-badge">▶</span>
                    </>
                  )}
                </div>
              )}
              <div className="card-title">{card.title}</div>
              <div className="card-footer">
                <span
                  className="priority"
                  style={{ color: PRIORITY_ICON[card.priority].color }}
                  title={t("priority.label", { name: t(`priority.${card.priority}`) })}
                >
                  {PRIORITY_ICON[card.priority].icon}
                </span>
                <span className="card-key">{card.key}</span>
                {card.attachmentCount > 0 && (
                  <span className="card-attachments" title={t("attachments.count", { count: card.attachmentCount })}>
                    <PaperclipIcon size={13} />
                    {card.attachmentCount}
                  </span>
                )}
                <div className="spacer" />
                <Avatar user={card.assigneeId ? membersById.get(card.assigneeId) : null} size={24} />
              </div>
            </article>
          </div>
        ))}
        {dropBeforeId === null && <div className="drop-indicator" />}

        {hasMore && <div ref={sentinelRef} className="sentinel" />}
        {state.loading && (
          <div className="column-status">
            <span className="spinner" /> {t("common.loading")}
          </div>
        )}
        {state.error && (
          <div className="column-status column-status--error">
            {state.error}{" "}
            <button className="btn btn-link btn-sm" onClick={props.onLoadMore}>
              {t("common.retry")}
            </button>
          </div>
        )}
        {!state.loading && state.cards.length === 0 && !hasMore && <div className="column-empty">{t("column.empty")}</div>}

        <button className="add-card" onClick={props.onAddCard}>
          {t("column.createTask")}
        </button>
      </div>
    </section>
  );
}

export function AddColumn({ onAdd }: { onAdd: (name: string) => Promise<void> }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    setBusy(true);
    try {
      await onAdd(name.trim());
      setName("");
      setOpen(false);
    } finally {
      setBusy(false);
    }
  }

  if (!open) {
    return (
      <button className="add-column" onClick={() => setOpen(true)}>
        {t("column.addColumn")}
      </button>
    );
  }
  return (
    <form className="add-column add-column--open" onSubmit={submit}>
      <input
        value={name}
        onChange={(e) => setName(e.target.value)}
        onKeyDown={(e) => e.key === "Escape" && setOpen(false)}
        placeholder={t("column.namePlaceholder")}
        maxLength={60}
        autoFocus
      />
      <div className="menu-actions">
        <button type="button" className="btn btn-ghost btn-sm" onClick={() => setOpen(false)}>
          {t("common.cancel")}
        </button>
        <button className="btn btn-primary btn-sm" disabled={busy || !name.trim()}>
          {t("common.add")}
        </button>
      </div>
    </form>
  );
}
