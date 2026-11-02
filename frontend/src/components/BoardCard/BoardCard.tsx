import type { DragEvent } from "react";
import { useTranslation } from "react-i18next";
import { Avatar } from "@/components/Avatar/Avatar";
import { PriorityIcon } from "@/components/PriorityIcon/PriorityIcon";
import { PaperclipIcon } from "@/components/ui/icons";
import { PlayBadge } from "@/components/ui/PlayBadge/PlayBadge";
import { Spacer } from "@/components/ui/Spacer/Spacer";
import { cx } from "@/lib/cx";
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
        <Spacer />
        <Avatar user={assignee} size={24} />
      </div>
    </article>
  );
}
