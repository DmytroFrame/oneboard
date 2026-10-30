import * as Y from "yjs";
import { applyAwarenessUpdate, Awareness, encodeAwarenessUpdate } from "y-protocols/awareness";
import type { BoardSocket } from "./realtime";

const REMOTE = Symbol("remote");
const COMPACT_EVERY = 100; // snapshot the server-side log every N of our own updates

const toB64 = (u: Uint8Array) => {
  let s = "";
  for (let i = 0; i < u.length; i += 0x8000) s += String.fromCharCode(...u.subarray(i, i + 0x8000));
  return btoa(s);
};
const fromB64 = (s: string) => Uint8Array.from(atob(s), (c) => c.charCodeAt(0));

/**
 * The shared (CRDT) document of one card, synced through the board socket:
 *  - "description": the rich-text fragment the editor binds to,
 *  - "fields": a map with the title.
 * The server keeps a numbered log of updates; we track which entries we have to tell it what it may
 * compact. Awareness carries the remote carets shown in the editor.
 */
export class CardDocProvider {
  readonly doc = new Y.Doc();
  readonly awareness = new Awareness(this.doc);
  readonly fields = this.doc.getMap<string>("fields");
  synced = false;

  private contiguous = 0; // all log entries up to this sequence are in our document
  private seen = new Set<number>();
  private pending: Uint8Array[] = [];
  private offs: (() => void)[] = [];

  constructor(
    private socket: BoardSocket,
    private cardId: number,
    private callbacks: { onSeed: () => void; onSynced: () => void },
  ) {
    this.doc.on("update", this.onLocalUpdate);
    this.awareness.on("update", this.onAwareness);

    const mine = (m: { [key: string]: unknown }) => m.cardId === cardId;
    this.offs.push(
      socket.on("doc.init", (m) => mine(m) && this.init(m.base, m.updates, m.seed)),
      socket.on("doc.update", (m) => {
        if (!mine(m)) return;
        Y.applyUpdate(this.doc, fromB64(m.update), REMOTE);
        this.mark(m.seq);
      }),
      socket.on("doc.ack", (m) => {
        if (!mine(m)) return;
        this.mark(m.seq);
        if (m.seq % COMPACT_EVERY === 0) this.compact();
      }),
      socket.on("awareness", (m) => mine(m) && applyAwarenessUpdate(this.awareness, fromB64(m.data), REMOTE)),
      // After a reconnect: fetch the log again; our offline edits are re-sent once synced.
      socket.on("welcome", () => this.open()),
    );
    if (socket.ready) this.open();
  }

  private open() {
    this.synced = false;
    this.socket.send({ type: "doc.open", cardId: this.cardId });
  }

  private init(base: number, updates: string[], seed: boolean) {
    Y.transact(this.doc, () => updates.forEach((u) => Y.applyUpdate(this.doc, fromB64(u), REMOTE)), REMOTE);
    this.contiguous = base + updates.length;
    this.seen.clear();
    const hadLocal = this.pending.length > 0 || this.doc.store.clients.has(this.doc.clientID);
    this.pending = [];
    this.synced = true;
    if (seed) this.callbacks.onSeed();
    else if (hadLocal) this.send(Y.encodeStateAsUpdate(this.doc)); // merge anything typed while offline
    this.callbacks.onSynced();
    this.broadcastAwareness();
  }

  private mark(seq: number) {
    this.seen.add(seq);
    while (this.seen.has(this.contiguous + 1)) this.seen.delete(++this.contiguous);
  }

  private compact() {
    this.socket.send({ type: "doc.compact", cardId: this.cardId, upto: this.contiguous, state: toB64(Y.encodeStateAsUpdate(this.doc)) });
  }

  private send(update: Uint8Array) {
    this.socket.send({ type: "doc.update", cardId: this.cardId, update: toB64(update) });
  }

  private onLocalUpdate = (update: Uint8Array, origin: unknown) => {
    if (origin === REMOTE) return;
    if (this.synced) this.send(update);
    else this.pending.push(update);
  };

  private onAwareness = ({ added, updated, removed }: { added: number[]; updated: number[]; removed: number[] }, origin: unknown) => {
    if (origin === REMOTE) return;
    const changed = [...added, ...updated, ...removed];
    this.socket.send({ type: "awareness", cardId: this.cardId, data: toB64(encodeAwarenessUpdate(this.awareness, changed)) });
  };

  private broadcastAwareness() {
    this.socket.send({ type: "awareness", cardId: this.cardId, data: toB64(encodeAwarenessUpdate(this.awareness, [this.doc.clientID])) });
  }

  /** Whether a doc update came from a peer (vs. this tab). */
  static isRemote(origin: unknown) {
    return origin === REMOTE;
  }

  destroy() {
    this.awareness.setLocalState(null); // tells peers our caret is gone
    this.offs.forEach((off) => off());
    this.doc.off("update", this.onLocalUpdate);
    this.awareness.off("update", this.onAwareness);
    this.awareness.destroy();
    this.doc.destroy();
  }
}
