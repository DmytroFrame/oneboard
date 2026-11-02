import type { Editor } from "@tiptap/core";
import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Attachments } from "@/components/Attachments/Attachments";
import { DescriptionEditor } from "@/components/DescriptionEditor/DescriptionEditor";
import { Button } from "@/components/ui/Button/Button";
import { FieldLabel } from "@/components/ui/FieldLabel/FieldLabel";
import { FormError } from "@/components/ui/FormError/FormError";
import { ModalDialog, ModalFooter } from "@/components/ui/Modal/Modal";
import { Spacer } from "@/components/ui/Spacer/Spacer";
import { CardDocProvider } from "@/lib/cardDoc";
import type { Card, CardInput } from "@/types";
import type { CardModalProps } from "./CardModal";
import { CardModalHeader } from "./CardModalHeader";
import { CardSidebar } from "./CardSidebar";
import s from "./CardModal.module.scss";

const AUTOSAVE_DELAY = 800;

/**
 * An existing card, edited together: title and description live in a shared CRDT document
 * (everyone sees keystrokes and carets live) and are autosaved; the other fields save on change
 * and reach the others as board events.
 */
export function ExistingCard(props: CardModalProps & { card: Card }) {
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
    <ModalDialog>
      <CardModalHeader title={card.key} peers={props.peersHere} onClose={onClose} status={statusText} />
      <div className={s.body}>
        <div className={s.main}>
          <input
            className={s.titleInput}
            value={title}
            disabled={!synced}
            onChange={(e) => {
              setTitle(e.target.value);
              provider?.fields.set("title", e.target.value);
            }}
            placeholder={t("card.titlePlaceholder")}
            maxLength={200}
          />
          <FieldLabel className={s.mainLabel}>{t("card.description")}</FieldLabel>
          {provider && (
            <DescriptionEditor
              key={card.id}
              provider={provider}
              user={{ name: currentUser.name, color: currentUser.color }}
              editable={synced}
              onReady={onEditorReady}
            />
          )}
          {error && <FormError>{error}</FormError>}
          <Attachments
            card={card}
            currentUserId={currentUser.id}
            isProjectOwner={props.isProjectOwner}
            onChange={(list) => props.onAttachmentsChange(card, list)}
          />
        </div>
        <CardSidebar
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
      <ModalFooter>
        {props.canDelete && (
          <Button type="button" variant="danger" onClick={remove} disabled={busy}>
            {t("common.delete")}
          </Button>
        )}
        <Spacer />
        <Button type="button" variant="primary" onClick={onClose}>
          {t("common.close")}
        </Button>
      </ModalFooter>
    </ModalDialog>
  );
}
