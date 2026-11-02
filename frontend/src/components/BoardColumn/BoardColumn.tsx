import { useCallback, useEffect, useRef, useState, type DragEvent, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { BoardCard } from "@/components/BoardCard/BoardCard";
import { Button } from "@/components/ui/Button/Button";
import { IconButton } from "@/components/ui/IconButton/IconButton";
import { Menu } from "@/components/ui/Menu/Menu";
import { MenuItem } from "@/components/ui/MenuItem/MenuItem";
import { Spacer } from "@/components/ui/Spacer/Spacer";
import { Spinner } from "@/components/ui/Spinner/Spinner";
import { useClickOutside } from "@/hooks/useClickOutside";
import { cx } from "@/lib/cx";
import type { Card, Column, User } from "@/types";
import s from "./BoardColumn.module.scss";

export interface ColumnState {
  column: Column;
  cards: Card[];
  total: number; // matching the current filters
  cursor: string | null; // null = everything loaded
  loading: boolean;
  error: string;
}

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

  const closeMenu = useCallback(() => setMenu("closed"), []);
  useClickOutside(menuRef, menu !== "closed", closeMenu);

  async function submitRename(e: FormEvent) {
    e.preventDefault();
    if (name.trim() && name.trim() !== state.column.name) await props.onRename(name.trim());
    setRenaming(false);
  }

  return (
    <section
      className={cx(s.column, dropBeforeId !== undefined && s.over)}
      data-column-id={state.column.id}
      // Accepting dragenter too (not just dragover) makes the column the drop target per the HTML5
      // spec; desktop browsers are lenient, the touch drag polyfill is not.
      onDragEnter={props.onDragOver}
      onDragOver={props.onDragOver}
      onDrop={props.onDrop}
    >
      <div className={s.head}>
        {renaming ? (
          <form onSubmit={submitRename} className={s.rename}>
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
          <span className={s.title} onDoubleClick={() => isOwner && setRenaming(true)}>
            {state.column.name}
          </span>
        )}
        <span className={s.count}>{state.total}</span>
        <Spacer />
        {isOwner && (
          <div className={s.menuWrap} ref={menuRef}>
            <IconButton size="sm" onClick={() => setMenu(menu === "closed" ? "open" : "closed")} aria-label={t("column.actions")}>
              ⋯
            </IconButton>
            {menu === "open" && (
              <Menu align="right" className={s.menu}>
                <MenuItem onClick={() => (setMenu("closed"), setRenaming(true))}>{t("column.rename")}</MenuItem>
                <MenuItem disabled={index === 0} onClick={() => (setMenu("closed"), props.onMove(index - 1))}>
                  {t("column.moveLeft")}
                </MenuItem>
                <MenuItem disabled={index === allColumns.length - 1} onClick={() => (setMenu("closed"), props.onMove(index + 1))}>
                  {t("column.moveRight")}
                </MenuItem>
                <MenuItem
                  danger
                  disabled={others.length === 0}
                  onClick={() => {
                    setMoveTo(others[Math.max(0, index - 1)]?.id ?? null);
                    setMenu("delete");
                  }}
                >
                  {t("column.delete")}
                </MenuItem>
              </Menu>
            )}
            {menu === "delete" && (
              <Menu align="right" className={cx(s.menu, s.menuForm)}>
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
                <div className={s.menuActions}>
                  <Button size="sm" onClick={() => setMenu("closed")}>
                    {t("common.cancel")}
                  </Button>
                  <Button variant="dangerSolid" size="sm" onClick={() => (setMenu("closed"), props.onDelete(moveTo))}>
                    {t("common.delete")}
                  </Button>
                </div>
              </Menu>
            )}
          </div>
        )}
      </div>

      {/* data-column-body: edge auto-scroll while dragging on touch screens (lib/touch.ts). */}
      <div className={s.body} ref={bodyRef} data-column-body>
        {state.cards.map((card) => (
          <div key={card.id} className={s.slot}>
            {dropBeforeId === card.id && <div className={s.dropIndicator} />}
            <BoardCard
              card={card}
              assignee={card.assigneeId ? membersById.get(card.assigneeId) : null}
              dragging={draggingId === card.id}
              remoteDrag={props.remoteDragIds.has(card.id)}
              onDragStart={props.onCardDragStart}
              onDragEnd={props.onCardDragEnd}
              onClick={props.onCardClick}
            />
          </div>
        ))}
        {dropBeforeId === null && <div className={s.dropIndicator} />}

        {hasMore && <div ref={sentinelRef} className={s.sentinel} />}
        {state.loading && (
          <div className={s.status}>
            <Spinner /> {t("common.loading")}
          </div>
        )}
        {state.error && (
          <div className={cx(s.status, s.statusError)}>
            {state.error}{" "}
            <Button variant="link" size="sm" onClick={props.onLoadMore}>
              {t("common.retry")}
            </Button>
          </div>
        )}
        {!state.loading && state.cards.length === 0 && !hasMore && <div className={s.empty}>{t("column.empty")}</div>}

        <button type="button" className={s.addCard} onClick={props.onAddCard}>
          {t("column.createTask")}
        </button>
      </div>
    </section>
  );
}
