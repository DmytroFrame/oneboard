import type { Editor } from "@tiptap/core";
import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { CardDocProvider } from "../cardDoc";
import type { BoardSocket } from "../realtime";
import { PRIORITIES, type Attachment, type Card, type CardInput, type Column, type Priority, type User } from "../types";
import { Attachments } from "./Attachments";
import { Avatar } from "./Avatar";
import { DescriptionEditor } from "./DescriptionEditor";

const AUTOSAVE_DELAY = 800;

interface Props {
  card: Card | null; // null = creating a new card
  initialColumnId: number;
  columns: Column[];
  users: User[];
  currentUser: User;
  canDelete: boolean;
  isProjectOwner: boolean;
  socket: BoardSocket | null;
  peersHere: User[]; // other people who have this card open right now
  onClose: () => void;
  onAttachmentsChange: (card: Card, attachments: Attachment[]) => void;
  onCreate: (input: CardInput, columnId: number) => Promise<void>;
  onUpdate: (card: Card, input: CardInput) => Promise<void>;
  onMove: (card: Card, columnId: number) => Promise<void>;
  onDelete: (card: Card) => Promise<void>;
}

export function CardModal(props: Props) {
  const { onClose } = props;
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      // Escape first closes the editor's slash menu, then the modal.
      if (e.key === "Escape" && !document.querySelector(".slash-menu")) onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div className="modal-backdrop" onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
      {props.card ? <ExistingCard {...props} card={props.card} /> : <NewCard {...props} />}
    </div>
  );
}

function Header({ title, peers, onClose, status }: { title: string; peers: User[]; onClose: () => void; status?: string }) {
  const { t } = useTranslation();
  return (
    <div className="modal-header">
      <span className="modal-key">{title}</span>
      {peers.length > 0 && (
        <span className="modal-peers" title={t("live.alsoHere")}>
          <span className="modal-peers-label">{t("live.alsoHere")}</span>
          {peers.map((u) => (
            <Avatar key={u.id} user={u} size={24} online />
          ))}
        </span>
      )}
      <div className="spacer" />
      {status && <span className="save-status">{status}</span>}
      <button type="button" className="icon-btn" onClick={onClose} aria-label={t("common.close")}>
        ✕
      </button>
    </div>
  );
}

interface SideProps {
  columns: Column[];
  users: User[];
  columnId: number;
  priority: Priority;
  assigneeId: number | null;
  author: User | undefined;
  card: Card | null;
  onColumn: (id: number) => void;
  onPriority: (p: Priority) => void;
  onAssignee: (id: number | null) => void;
}

function Side({ columns, users, columnId, priority, assigneeId, author, card, onColumn, onPriority, onAssignee }: SideProps) {
  const { t, i18n } = useTranslation();
  const dateFmt = useMemo(() => new Intl.DateTimeFormat(i18n.language, { dateStyle: "medium", timeStyle: "short" }), [i18n.language]);
  const assignee = users.find((u) => u.id === assigneeId);
  return (
    <aside className="modal-side">
      <label className="field">
        {t("card.column")}
        <select value={columnId} onChange={(e) => onColumn(Number(e.target.value))}>
          {columns.map((c) => (
            <option key={c.id} value={c.id}>
              {c.name}
            </option>
          ))}
        </select>
      </label>
      <label className="field">
        {t("card.assignee")}
        <div className="select-with-avatar">
          <Avatar user={assignee} size={24} />
          <select value={assigneeId ?? ""} onChange={(e) => onAssignee(e.target.value ? Number(e.target.value) : null)}>
            <option value="">{t("common.unassigned")}</option>
            {users.map((u) => (
              <option key={u.id} value={u.id}>
                {u.name}
              </option>
            ))}
          </select>
        </div>
      </label>
      <label className="field">
        {t("card.priority")}
        <select value={priority} onChange={(e) => onPriority(e.target.value as Priority)}>
          {PRIORITIES.map((p) => (
            <option key={p} value={p}>
              {t(`priority.${p}`)}
            </option>
          ))}
        </select>
      </label>
      <div className="field">
        {t("card.author")}
        <div className="person">
          <Avatar user={author} size={24} />
          <span>{author?.name ?? t("common.formerMember")}</span>
        </div>
      </div>
      {card && (
        <div className="meta">
          <div>{t("card.created", { date: dateFmt.format(new Date(card.createdAt)) })}</div>
          <div>{t("card.updated", { date: dateFmt.format(new Date(card.updatedAt)) })}</div>
        </div>
      )}
    </aside>
  );
}

/** A new card: a plain form with a local (non-shared) editor. */
function NewCard({ initialColumnId, columns, users, currentUser, onClose, onCreate }: Props) {
  const { t } = useTranslation();
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [columnId, setColumnId] = useState(initialColumnId);
  const [priority, setPriority] = useState<Priority>("medium");
  const [assigneeId, setAssigneeId] = useState<number | null>(currentUser.id);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await onCreate({ title, description, priority, assigneeId }, columnId);
      onClose();
    } catch (err) {
      setError((err as Error).message);
      setBusy(false);
    }
  }

  return (
    <form className="modal" onSubmit={submit}>
      <Header title={t("card.newTask")} peers={[]} onClose={onClose} />
      <div className="modal-body">
        <div className="modal-main">
          <input className="title-input" value={title} onChange={(e) => setTitle(e.target.value)} placeholder={t("card.titlePlaceholder")} maxLength={200} required autoFocus />
          <div className="field-label">{t("card.description")}</div>
          <DescriptionEditor user={{ name: currentUser.name, color: currentUser.color }} initialMarkdown="" onChange={setDescription} />
          {error && <div className="form-error">{error}</div>}
          <p className="muted attachments-hint">{t("attachments.saveFirst")}</p>
        </div>
        <Side
          columns={columns}
          users={users}
          columnId={columnId}
          priority={priority}
          assigneeId={assigneeId}
          author={currentUser}
          card={null}
          onColumn={setColumnId}
          onPriority={setPriority}
          onAssignee={setAssigneeId}
        />
      </div>
      <div className="modal-footer">
        <div className="spacer" />
        <button type="button" className="btn btn-ghost" onClick={onClose}>
          {t("common.cancel")}
        </button>
        <button className="btn btn-primary" disabled={busy}>
          {t("common.create")}
        </button>
      </div>
    </form>
  );
}

/**
 * An existing card, edited together: title and description live in a shared CRDT document
 * (everyone sees keystrokes and carets live) and are autosaved; the other fields save on change
 * and reach the others as board events.
 */
function ExistingCard(props: Props & { card: Card }) {
  const { card, socket, currentUser, users, columns, onClose, onUpdate, onMove } = props;
  const { t } = useTranslation();
  const [provider, setProvider] = useState<CardDocProvider | null>(null);
  const [synced, setSynced] = useState(false);
  const [title, setTitle] = useState(card.title);
  const [status, setStatus] = useState<"idle" | "saving" | "saved">("idle");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const editorRef = useRef<Editor | null>(null);
  const seedPending = useRef(false);
  const seeding = useRef(false);
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const cardRef = useRef(card);
  cardRef.current = card;

  // Fields other than text follow the card as it changes (also when someone else changes them).
  const [priority, setPriority] = useState(card.priority);
  const [assigneeId, setAssigneeId] = useState(card.assigneeId);
  useEffect(() => setPriority(card.priority), [card.priority]);
  useEffect(() => setAssigneeId(card.assigneeId), [card.assigneeId]);

  const seed = useCallback(() => {
    const editor = editorRef.current;
    if (!editor || !provider) {
      seedPending.current = true;
      return;
    }
    seedPending.current = false;
    seeding.current = true;
    provider.fields.set("title", cardRef.current.title);
    editor.commands.setContent(cardRef.current.description, { contentType: "markdown" });
    seeding.current = false;
  }, [provider]);
  const seedRef = useRef(seed);
  seedRef.current = seed;

  useEffect(() => {
    if (!socket) return;
    const p = new CardDocProvider(socket, card.id, {
      onSeed: () => seedRef.current(),
      onSynced: () => setSynced(true),
    });
    const onFields = () => {
      const v = p.fields.get("title");
      if (typeof v === "string") setTitle(v);
    };
    p.fields.observe(onFields);
    setProvider(p);
    return () => {
      p.fields.unobserve(onFields);
      p.destroy();
      setProvider(null);
      setSynced(false);
    };
  }, [socket, card.id]);

  const save = useCallback(async () => {
    const editor = editorRef.current;
    if (!provider || !editor) return;
    const current = cardRef.current;
    const nextTitle = (provider.fields.get("title") ?? current.title).trim();
    if (!nextTitle) return; // an empty title would be rejected; keep typing
    setStatus("saving");
    setError("");
    try {
      await onUpdate(current, {
        title: nextTitle,
        description: editor.getMarkdown(),
        priority: current.priority,
        assigneeId: current.assigneeId,
      });
      setStatus("saved");
    } catch (err) {
      setError((err as Error).message);
      setStatus("idle");
    }
  }, [provider, onUpdate]);

  // Autosave our own edits (remote ones are saved by whoever typed them).
  useEffect(() => {
    if (!provider) return;
    const onUpdate = (_u: Uint8Array, origin: unknown) => {
      if (CardDocProvider.isRemote(origin) || seeding.current || !provider.synced) return;
      if (saveTimer.current) clearTimeout(saveTimer.current);
      setStatus("saving");
      saveTimer.current = setTimeout(() => void save(), AUTOSAVE_DELAY);
    };
    provider.doc.on("update", onUpdate);
    return () => provider.doc.off("update", onUpdate);
  }, [provider, save]);

  // Flush a pending save when the modal closes.
  const saveRef = useRef(save);
  saveRef.current = save;
  useEffect(
    () => () => {
      if (saveTimer.current) {
        clearTimeout(saveTimer.current);
        void saveRef.current();
      }
    },
    [],
  );

  const onEditorReady = useCallback((editor: Editor) => {
    editorRef.current = editor;
    if (seedPending.current) seedRef.current();
  }, []);

  async function saveField(patch: Partial<CardInput>) {
    const editor = editorRef.current;
    setError("");
    try {
      await onUpdate(cardRef.current, {
        title: (provider?.fields.get("title") ?? card.title).trim() || card.title,
        description: editor && synced ? editor.getMarkdown() : card.description,
        priority,
        assigneeId,
        ...patch,
      });
    } catch (err) {
      setError((err as Error).message);
    }
  }

  async function remove() {
    if (!confirm(t("card.deleteConfirm", { key: card.key }))) return;
    setBusy(true);
    try {
      await props.onDelete(card);
      onClose();
    } catch (err) {
      setError((err as Error).message);
      setBusy(false);
    }
  }

  const statusText = !socket?.ready && !synced ? t("live.offline") : status === "saving" ? t("live.saving") : status === "saved" ? `✓ ${t("live.saved")}` : undefined;

  return (
    <div className="modal" role="dialog">
      <Header title={card.key} peers={props.peersHere} onClose={onClose} status={statusText} />
      <div className="modal-body">
        <div className="modal-main">
          <input
            className="title-input"
            value={title}
            disabled={!synced}
            onChange={(e) => {
              setTitle(e.target.value);
              provider?.fields.set("title", e.target.value);
            }}
            placeholder={t("card.titlePlaceholder")}
            maxLength={200}
          />
          <div className="field-label">{t("card.description")}</div>
          {provider && (
            <DescriptionEditor
              key={card.id}
              provider={provider}
              user={{ name: currentUser.name, color: currentUser.color }}
              editable={synced}
              onReady={onEditorReady}
            />
          )}
          {error && <div className="form-error">{error}</div>}
          <Attachments
            card={card}
            currentUserId={currentUser.id}
            isProjectOwner={props.isProjectOwner}
            onChange={(list) => props.onAttachmentsChange(card, list)}
          />
        </div>
        <Side
          columns={columns}
          users={users}
          columnId={card.columnId}
          priority={priority}
          assigneeId={assigneeId}
          author={users.find((u) => u.id === card.authorId)}
          card={card}
          onColumn={(id) => void onMove(card, id).catch((err) => setError((err as Error).message))}
          onPriority={(p) => (setPriority(p), void saveField({ priority: p }))}
          onAssignee={(id) => (setAssigneeId(id), void saveField({ assigneeId: id }))}
        />
      </div>
      <div className="modal-footer">
        {props.canDelete && (
          <button type="button" className="btn btn-danger" onClick={remove} disabled={busy}>
            {t("common.delete")}
          </button>
        )}
        <div className="spacer" />
        <button type="button" className="btn btn-primary" onClick={onClose}>
          {t("common.close")}
        </button>
      </div>
    </div>
  );
}
