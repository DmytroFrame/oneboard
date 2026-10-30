import { useEffect, useRef, type CSSProperties, type RefObject } from "react";
import { anchorElement } from "../realtime";
import type { Card, User } from "../types";
import type { RemotePeer } from "../useBoardRealtime";
import { Avatar } from "./Avatar";

interface Props {
  boardRef: RefObject<HTMLDivElement | null>;
  peers: RemotePeer[];
  live: RefObject<Map<string, RemotePeer>>;
  usersById: Map<number, User>;
  findCard: (id: number) => Card | undefined;
}

/**
 * Other people's pointers over the board, Figma-style. Positions are resolved every frame from
 * anchors (card/column + relative offset), so they follow scrolling and differing layouts.
 * A peer with a card open shows as their avatar sitting on that card; a peer dragging a card
 * drags a ghost of it.
 */
export function LiveCursors({ boardRef, peers, live, usersById, findCard }: Props) {
  const nodes = useRef(new Map<string, HTMLDivElement>());
  const ghosts = useRef(new Map<string, HTMLDivElement>());

  useEffect(() => {
    let frame = 0;
    const tick = () => {
      const board = boardRef.current;
      if (board) {
        const b = board.getBoundingClientRect();
        for (const [connId, el] of nodes.current) {
          const p = live.current.get(connId);
          let x: number | null = null;
          let y: number | null = null;
          let mode = "arrow";
          const focusEl = p?.cardId && !p.drag ? board.querySelector<HTMLElement>(`[data-card-id="${p.cardId}"]`) : null;
          if (focusEl) {
            // "Inside" the task: an avatar bubble on the card's corner.
            const r = focusEl.getBoundingClientRect();
            x = r.right - b.left - 16;
            y = r.top - b.top + 4;
            mode = "focus";
          } else if (p?.anchor) {
            const target = anchorElement(p.anchor, board);
            if (target) {
              const r = target.getBoundingClientRect();
              x = r.left - b.left + p.anchor.fx * r.width;
              y = r.top - b.top + p.anchor.fy * r.height;
            }
          }
          if (x === null || y === null || (p?.cardId && mode !== "focus" && !p.drag)) {
            el.style.opacity = "0";
          } else {
            el.style.opacity = "1";
            el.style.transform = `translate(${x}px, ${y}px)`;
          }
          el.dataset.mode = mode;

          const ghost = ghosts.current.get(connId);
          if (ghost) {
            ghost.style.opacity = p?.drag && x !== null ? "1" : "0";
            if (x !== null && y !== null) ghost.style.transform = `translate(${x + 14}px, ${y + 18}px) rotate(-2deg)`;
          }
        }
      }
      frame = requestAnimationFrame(tick);
    };
    frame = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(frame);
  }, [boardRef, live]);

  return (
    <div className="live-layer" aria-hidden="true">
      {peers.map((p) => {
        const u = usersById.get(p.userId);
        const color = u?.color ?? "#888";
        const dragged = p.drag ? findCard(p.drag.cardId) : undefined;
        return (
          <div key={p.connId}>
            {dragged && (
              <div
                className="peer-ghost"
                ref={(el) => {
                  if (el) ghosts.current.set(p.connId, el);
                  else ghosts.current.delete(p.connId);
                }}
                style={{ borderColor: color }}
              >
                <div className="card-title">{dragged.title}</div>
                <span className="card-key">{dragged.key}</span>
              </div>
            )}
            <div
              className="peer-cursor"
              ref={(el) => {
                if (el) nodes.current.set(p.connId, el);
                else nodes.current.delete(p.connId);
              }}
              style={{ "--peer-color": color } as CSSProperties}
            >
              <svg className="peer-arrow" width="18" height="18" viewBox="0 0 18 18">
                <path d="M2 1.5 16 8.2l-6.1 1.6L7 16z" fill={color} stroke="white" strokeWidth="1.4" strokeLinejoin="round" />
              </svg>
              <span className="peer-label">{u?.name ?? "…"}</span>
              <span className="peer-bubble">
                <Avatar user={u} size={26} />
              </span>
            </div>
          </div>
        );
      })}
    </div>
  );
}
