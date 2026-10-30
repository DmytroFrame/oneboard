import { getAccessToken, getRealtimeConnId, setRealtimeConnId } from "./api";
import type { Card } from "./types";

/** Where a cursor is, relative to an element every client has, so it lands right on any screen size. */
export interface Anchor {
  kind: "card" | "column" | "board";
  id?: number;
  fx: number; // 0..1 across the element
  fy: number; // 0..1 down the element
}

export interface Peer {
  connId: string;
  userId: number;
  cardId: number | null;
}

export interface BoardEvent {
  type: "card.created" | "card.updated" | "card.moved" | "card.deleted" | "columns.changed" | "members.changed" | "project.deleted";
  actorId: number;
  origin: string; // connection (tab) whose request caused it; "" if unknown
  card?: Card;
  cardId?: number;
  columnId?: number;
  fromColumnId?: number;
  afterId?: number | null;
  beforeId?: number | null;
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
type Message = { type: string; [key: string]: any };
type Handler = (m: Message) => void;

/** A change to the signed-in user's access, delivered on their user channel. */
export interface UserEvent {
  type: "access" | "org.members";
  reason: "added" | "removed" | "role" | "renamed" | "deleted" | "";
  orgId: number;
  projectId: number;
  name: string;
  actorId: number;
}

/**
 * A real-time WebSocket: one per open board, plus one per tab for the user's own channel
 * (projectId 0). Reconnects with backoff; listeners re-sync on "welcome", which arrives after
 * every (re)connect.
 */
export class BoardSocket {
  connId: string | null = null;
  private ws: WebSocket | null = null;
  private handlers = new Map<string, Set<Handler>>();
  private closed = false;
  private attempt = 0;

  constructor(private projectId: number) {
    void this.connect();
  }

  private async connect() {
    if (this.closed) return;
    const token = await getAccessToken();
    if (this.closed) return;
    if (!token) return this.retry();

    const ws = new WebSocket(`${location.protocol === "https:" ? "wss" : "ws"}://${location.host}/api/ws`);
    this.ws = ws;
    ws.onopen = () => ws.send(JSON.stringify({ type: "auth", token, projectId: this.projectId }));
    ws.onmessage = (e) => {
      const m = JSON.parse(e.data) as Message;
      if (m.type === "welcome") {
        this.connId = m.connId;
        this.attempt = 0;
        // Board connections tag REST requests (see api.ts); the user channel does not.
        if (this.projectId) setRealtimeConnId(m.connId);
      }
      // Access to the board is gone: reconnecting would only be refused again.
      if (m.type === "error" && m.code === "project_not_found") this.closed = true;
      this.handlers.get(m.type)?.forEach((h) => h(m));
    };
    ws.onclose = () => {
      if (this.ws !== ws) return;
      this.ws = null;
      this.releaseConnId();
      this.handlers.get("disconnect")?.forEach((h) => h({ type: "disconnect" }));
      this.retry();
    };
  }

  private retry() {
    if (this.closed) return;
    const delay = Math.min(10_000, 500 * 2 ** this.attempt++);
    setTimeout(() => void this.connect(), delay);
  }

  get ready() {
    return this.ws?.readyState === WebSocket.OPEN && this.connId !== null;
  }

  /** Fire-and-forget; dropped while disconnected (state is re-sent after "welcome"). */
  send(msg: Message) {
    if (this.ready) this.ws!.send(JSON.stringify(msg));
  }

  on(type: string, handler: Handler): () => void {
    let set = this.handlers.get(type);
    if (!set) this.handlers.set(type, (set = new Set()));
    set.add(handler);
    return () => set!.delete(handler);
  }

  private releaseConnId() {
    if (this.connId && getRealtimeConnId() === this.connId) setRealtimeConnId(null);
    this.connId = null;
  }

  close() {
    this.releaseConnId();
    this.closed = true;
    this.ws?.close();
    this.ws = null;
  }
}

/** Computes the anchor for a pointer position: the card or column under it, else the board itself. */
export function anchorAt(target: Element | null, x: number, y: number, board: HTMLElement): Anchor {
  const card = target?.closest<HTMLElement>("[data-card-id]");
  const column = card ? null : target?.closest<HTMLElement>("[data-column-id]");
  const el = card ?? column ?? board;
  const r = el.getBoundingClientRect();
  return {
    kind: card ? "card" : column ? "column" : "board",
    id: card ? Number(card.dataset.cardId) : column ? Number(column.dataset.columnId) : undefined,
    fx: r.width ? (x - r.left) / r.width : 0,
    fy: r.height ? (y - r.top) / r.height : 0,
  };
}

/** Finds the element an anchor refers to on this screen (null if that card isn't loaded here). */
export function anchorElement(anchor: Anchor, board: HTMLElement): HTMLElement | null {
  if (anchor.kind === "card") return board.querySelector(`[data-card-id="${anchor.id}"]`);
  if (anchor.kind === "column") return board.querySelector(`[data-column-id="${anchor.id}"]`);
  return board;
}
