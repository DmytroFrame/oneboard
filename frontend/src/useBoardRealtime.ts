import { useEffect, useRef, useState } from "react";
import { BoardSocket, type Anchor, type BoardEvent, type Peer } from "./realtime";

export interface RemotePeer extends Peer {
  anchor: Anchor | null; // last cursor position (null = pointer left the board)
  drag: { cardId: number; anchor: Anchor | null; at: number } | null; // card they are dragging right now
}

// Browsers repeat dragover while a drag is in progress, so a drag silent for this long has ended
// (e.g. its "end" message was lost, or the dragged element was re-rendered and never got dragend).
const DRAG_STALE_MS = 3000;

/**
 * Connects to the board's real-time channel. Structural changes (join/leave/focus/drag start-end)
 * re-render; cursor moves only update `live` (read by LiveCursors on every animation frame).
 */
export function useBoardRealtime(projectId: number, onEvent: (e: BoardEvent) => void) {
  const [socket, setSocket] = useState<BoardSocket | null>(null);
  const [peers, setPeers] = useState<RemotePeer[]>([]);
  const [connected, setConnected] = useState(false);
  const live = useRef(new Map<string, RemotePeer>());
  const onEventRef = useRef(onEvent);
  onEventRef.current = onEvent;
  const clearDragRef = useRef<(cardId: number) => void>(() => {});

  useEffect(() => {
    const s = new BoardSocket(projectId);
    const map = live.current;
    const publish = () => setPeers([...map.values()]);
    const upsert = (p: Peer) => {
      const prev = map.get(p.connId);
      map.set(p.connId, { connId: p.connId, userId: p.userId, cardId: p.cardId ?? null, anchor: prev?.anchor ?? null, drag: prev?.drag ?? null });
    };

    const offs = [
      s.on("welcome", (m) => {
        map.clear();
        (m.peers as Peer[]).forEach((p) => p.connId !== m.connId && upsert(p));
        setConnected(true);
        publish();
      }),
      s.on("disconnect", () => {
        map.clear();
        setConnected(false);
        publish();
      }),
      s.on("join", (m) => (upsert(m.peer), publish())),
      s.on("leave", (m) => (map.delete(m.connId), publish())),
      s.on("focus", (m) => {
        const p = map.get(m.connId);
        if (p) map.set(m.connId, { ...p, cardId: m.cardId ?? null });
        publish();
      }),
      s.on("cursor", (m) => {
        const p = map.get(m.connId);
        if (p) p.anchor = m.anchor ?? null; // no re-render: LiveCursors polls
      }),
      s.on("drag", (m) => {
        const p = map.get(m.connId);
        if (!p) return;
        const wasDragging = p.drag?.cardId ?? null;
        p.drag = m.cardId ? { cardId: m.cardId, anchor: m.anchor ?? null, at: Date.now() } : null;
        if (m.anchor) p.anchor = m.anchor;
        if (wasDragging !== (m.cardId ?? null)) publish();
      }),
      s.on("event", (m) => onEventRef.current(m.event as BoardEvent)),
    ];
    const sweep = setInterval(() => {
      let changed = false;
      for (const p of map.values()) {
        if (p.drag && Date.now() - p.drag.at > DRAG_STALE_MS) {
          p.drag = null;
          changed = true;
        }
      }
      if (changed) publish();
    }, 1000);
    clearDragRef.current = (cardId: number) => {
      let changed = false;
      for (const p of map.values()) {
        if (p.drag?.cardId === cardId) {
          p.drag = null;
          changed = true;
        }
      }
      if (changed) publish();
    };
    setSocket(s);
    return () => {
      clearInterval(sweep);
      offs.forEach((off) => off());
      s.close();
      map.clear();
      setSocket(null);
      setPeers([]);
      setConnected(false);
    };
  }, [projectId]);

  /** Forget any peer's drag of this card (called when its move is committed). */
  const clearDrag = (cardId: number) => clearDragRef.current(cardId);

  return { socket, peers, live, connected, clearDrag };
}
