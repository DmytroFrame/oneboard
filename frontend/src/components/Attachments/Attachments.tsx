import { useEffect, useRef, useState, type DragEvent } from "react";
import { useTranslation } from "react-i18next";
import { Lightbox } from "@/components/Lightbox/Lightbox";
import { Badge } from "@/components/ui/Badge/Badge";
import { Button } from "@/components/ui/Button/Button";
import { FieldLabel } from "@/components/ui/FieldLabel/FieldLabel";
import { FormError } from "@/components/ui/FormError/FormError";
import { IconButton, iconButtonClass } from "@/components/ui/IconButton/IconButton";
import { PaperclipIcon } from "@/components/ui/icons";
import { PlayBadge } from "@/components/ui/PlayBadge/PlayBadge";
import { Spinner } from "@/components/ui/Spinner/Spinner";
import { api } from "@/lib/api";
import { cx } from "@/lib/cx";
import { fileExtension, formatSize } from "@/lib/format";
import type { Attachment, Card } from "@/types";
import s from "./Attachments.module.scss";

const MAX_BYTES = 50 << 20;

interface Props {
  card: Card;
  currentUserId: number;
  isProjectOwner: boolean;
  onChange: (attachments: Attachment[]) => void;
}

/** Files of a card: upload (button or drag & drop), image/video previews, download, delete. */
export function Attachments({ card, currentUserId, isProjectOwner, onChange }: Props) {
  const { t, i18n } = useTranslation();
  const [items, setItems] = useState<Attachment[] | null>(null);
  const [uploading, setUploading] = useState<{ key: string; name: string }[]>([]);
  const [error, setError] = useState("");
  const [dragOver, setDragOver] = useState(false);
  const [preview, setPreview] = useState<Attachment | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  useEffect(() => {
    api
      .attachments(card.id)
      .then(setItems)
      .catch((err) => setError((err as Error).message));
  }, [card.id]);

  // Someone else added or removed a file (the card's count changed): reload the list.
  useEffect(() => {
    if (items && uploading.length === 0 && card.attachmentCount !== items.length) {
      api.attachments(card.id).then(setItems).catch(() => {});
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [card.attachmentCount]);

  // Report every change, including the first load (which also corrects a stale count on the card).
  useEffect(() => {
    if (items) onChangeRef.current(items);
  }, [items]);

  function upload(files: File[]) {
    setError("");
    for (const file of files) {
      if (file.size > MAX_BYTES) {
        setError(t("attachments.tooLarge", { name: file.name }));
        continue;
      }
      const key = `${file.name}-${file.size}-${Math.random()}`;
      setUploading((u) => [...u, { key, name: file.name }]);
      api
        .uploadAttachment(card.id, file)
        .then((a) => setItems((prev) => [...(prev ?? []), a]))
        .catch((err) => setError((err as Error).message))
        .finally(() => setUploading((u) => u.filter((x) => x.key !== key)));
    }
  }

  async function remove(a: Attachment) {
    if (!confirm(t("attachments.confirmDelete", { name: a.fileName }))) return;
    try {
      await api.deleteAttachment(a.id);
      setItems((prev) => (prev ?? []).filter((x) => x.id !== a.id));
    } catch (err) {
      setError((err as Error).message);
    }
  }

  const hasFiles = (e: DragEvent) => e.dataTransfer.types.includes("Files");

  return (
    <section
      className={cx(s.attachments, dragOver && s.drop)}
      onDragOver={(e) => {
        if (!hasFiles(e)) return;
        e.preventDefault();
        setDragOver(true);
      }}
      onDragLeave={(e) => !e.currentTarget.contains(e.relatedTarget as Node) && setDragOver(false)}
      onDrop={(e) => {
        if (!hasFiles(e)) return;
        e.preventDefault();
        setDragOver(false);
        upload([...e.dataTransfer.files]);
      }}
    >
      <div className={s.head}>
        <FieldLabel>
          <PaperclipIcon /> {t("attachments.title")} {items && items.length > 0 && <Badge>{items.length}</Badge>}
        </FieldLabel>
        <input ref={inputRef} type="file" multiple hidden onChange={(e) => (upload([...(e.target.files ?? [])]), (e.target.value = ""))} />
        <Button type="button" size="sm" onClick={() => inputRef.current?.click()}>
          {t("attachments.attach")}
        </Button>
      </div>

      {dragOver && <div className={s.dropzone}>{t("attachments.dropHere")}</div>}

      {items && items.length === 0 && uploading.length === 0 && !dragOver && <div className={s.empty}>{t("attachments.empty")}</div>}

      <div className={s.grid}>
        {items?.map((a) => (
          <div className={s.item} key={a.id}>
            <button type="button" className={cx(s.preview, a.kind === "file" && s.previewFile)} onClick={() => (a.kind === "file" ? window.open(a.downloadUrl) : setPreview(a))}>
              {a.kind === "image" && <img src={a.thumbUrl ?? a.url} alt="" loading="lazy" />}
              {a.kind === "video" && (
                <>
                  <video src={`${a.url}#t=0.1`} preload="metadata" muted playsInline />
                  <PlayBadge />
                </>
              )}
              {a.kind === "file" && <span className={s.ext}>{fileExtension(a.fileName)}</span>}
            </button>
            <div className={s.meta}>
              <span className={s.name} title={a.fileName}>
                {a.fileName}
              </span>
              <small>{formatSize(a.size, i18n.language)}</small>
            </div>
            <div className={s.actions}>
              <a className={iconButtonClass({ size: "sm", className: s.action })} href={a.downloadUrl} title={t("attachments.download")} aria-label={t("attachments.download")}>
                ↓
              </a>
              {(isProjectOwner || a.uploaderId === currentUserId) && (
                <IconButton size="sm" className={s.action} onClick={() => remove(a)} title={t("common.delete")} aria-label={t("common.delete")}>
                  ✕
                </IconButton>
              )}
            </div>
          </div>
        ))}
        {uploading.map((u) => (
          <div className={cx(s.item, s.uploading)} key={u.key}>
            <div className={s.preview}>
              <Spinner />
            </div>
            <div className={s.meta}>
              <span className={s.name}>{u.name}</span>
              <small>{t("attachments.uploading")}</small>
            </div>
          </div>
        ))}
      </div>
      {error && <FormError>{error}</FormError>}

      {preview && <Lightbox attachment={preview} onClose={() => setPreview(null)} />}
    </section>
  );
}
