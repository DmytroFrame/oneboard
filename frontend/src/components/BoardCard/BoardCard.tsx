import type { DragEvent } from "react";
import { useTranslation } from "react-i18next";
import { Avatar } from "@/components/Avatar/Avatar";
import { PriorityIcon } from "@/components/PriorityIcon/PriorityIcon";
import { PaperclipIcon } from "@/components/ui/icons";
import { PlayBadge } from "@/components/ui/PlayBadge/PlayBadge";
import { Spacer } from "@/components/ui/Spacer/Spacer";
import { cx } from "@/lib/cx";
import { formatMinutes } from "@/lib/format";
import type { Card, User } from "@/types";
import s from "./BoardCard.module.scss";

interface Props {
  card: Card;
  assignee: User | null | undefined;
  dragging: boolean;
  /** Someone else is dragging it right now. */
  remoteDrag: boolean;
  onDragStart: (e: DragEvent, card: Card) => void;
  onDragEnd: () => void;
  onClick: (card: Card) => void;
}

/** A task on the board: cover, title, priority, key, file count, assignee. */
export function BoardCard({ card, assignee, dragging, remoteDrag, onDragStart, onDragEnd, onClick }: Props) {
  const { t } = useTranslation();
  return (
    <article
      // data-card-id: drop positions, live cursor anchors (lib/realtime.ts) and focus bubbles find cards by it.
      data-card-id={card.id}
      className={cx(s.card, dragging && s.dragging, remoteDrag && s.remoteDrag)}
      draggable
      onDragStart={(e) => onDragStart(e, card)}
      onDragEnd={onDragEnd}
      onClick={() => onClick(card)}
    >
      {card.cover && (
        <div className={s.cover}>
          {card.cover.kind === "image" ? (
            <img src={card.cover.url} alt="" loading="lazy" draggable={false} />
          ) : (
            <>
              <video src={`${card.cover.url}#t=0.1`} preload="metadata" muted playsInline />
              <PlayBadge />
            </>
          )}
        </div>
      )}
      {card.tags.length > 0 && (
        <div className={s.tags}>
          {card.tags.map((tag) => (
            <span key={tag.id} className={s.tag} style={{ background: tag.color }} title={tag.name}>
              {tag.name}
            </span>
          ))}
        </div>
      )}
      <div className={s.title}>{card.title}</div>
      <div className={s.footer}>
        <PriorityIcon priority={card.priority} />
        <span className={s.key}>{card.key}</span>
        {card.attachmentCount > 0 && (
          <span className={s.attachments} title={t("attachments.count", { count: card.attachmentCount })}>
            <PaperclipIcon size={13} />
            {card.attachmentCount}
          </span>
        )}
        {card.checklistTotal > 0 && (
          <span className={cx(s.meta, card.checklistDone === card.checklistTotal && s.metaDone)} title={t("tasks.checklists")}>
            ☑ {card.checklistDone}/{card.checklistTotal}
          </span>
        )}
        {card.commentCount > 0 && (
          <span className={s.meta} title={t("tasks.tabCommentsActivity")}>
            💬 {card.commentCount}
          </span>
        )}
        {card.trackedMinutes > 0 && (
          <span className={s.meta} title={t("tasks.tracked")}>
            ⏱ {formatMinutes(card.trackedMinutes)}
          </span>
        )}
        <Spacer />
        {card.storyPoints != null && (
          <span className={s.points} title={t("tasks.storyPoints")}>
            {card.storyPoints}
          </span>
        )}
        <Avatar user={assignee} size={24} />
      </div>
    </article>
  );
}
