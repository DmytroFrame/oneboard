import { useEffect, useRef, useState, type DragEvent } from "react";
import { useTranslation } from "react-i18next";
import { api } from "../api";
import type { Attachment, Card } from "../types";

const MAX_BYTES = 50 << 20;

export function formatSize(bytes: number, locale: string) {
  const fmt = (n: number) => new Intl.NumberFormat(locale, { maximumFractionDigits: n < 10 ? 1 : 0 }).format(n);
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${fmt(bytes / 1024)} KB`;
  return `${fmt(bytes / 1024 / 1024)} MB`;
}

const extension = (name: string) => (name.includes(".") ? name.split(".").pop()!.slice(0, 4).toUpperCase() : "FILE");

export function PaperclipIcon({ size = 14 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" aria-hidden="true">
      <path d="m21.4 11.6-9.2 9.2a6 6 0 0 1-8.5-8.5l9.2-9.2a4 4 0 0 1 5.7 5.7l-9.2 9.2a2 2 0 0 1-2.8-2.8l8.5-8.5" />
    </svg>
  );
}

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
      className={`attachments${dragOver ? " attachments--drop" : ""}`}
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
      <div className="attachments-head">
        <span className="field-label">
          <PaperclipIcon /> {t("attachments.title")} {items && items.length > 0 && <span className="badge">{items.length}</span>}
        </span>
        <input ref={inputRef} type="file" multiple hidden onChange={(e) => (upload([...(e.target.files ?? [])]), (e.target.value = ""))} />
        <button type="button" className="btn btn-ghost btn-sm" onClick={() => inputRef.current?.click()}>
          {t("attachments.attach")}
        </button>
      </div>

      {dragOver && <div className="attachments-dropzone">{t("attachments.dropHere")}</div>}

      {items && items.length === 0 && uploading.length === 0 && !dragOver && <div className="attachments-empty">{t("attachments.empty")}</div>}

      <div className="attachment-grid">
        {items?.map((a) => (
          <div className="attachment" key={a.id}>
            <button type="button" className={`attachment-preview attachment-preview--${a.kind}`} onClick={() => (a.kind === "file" ? window.open(a.downloadUrl) : setPreview(a))}>
              {a.kind === "image" && <img src={a.thumbUrl ?? a.url} alt="" loading="lazy" />}
              {a.kind === "video" && (
                <>
                  <video src={`${a.url}#t=0.1`} preload="metadata" muted playsInline />
                  <span className="play-badge">▶</span>
                </>
              )}
              {a.kind === "file" && <span className="file-ext">{extension(a.fileName)}</span>}
            </button>
            <div className="attachment-meta">
              <span className="attachment-name" title={a.fileName}>
                {a.fileName}
              </span>
              <small>{formatSize(a.size, i18n.language)}</small>
            </div>
            <div className="attachment-actions">
              <a className="icon-btn icon-btn--sm" href={a.downloadUrl} title={t("attachments.download")} aria-label={t("attachments.download")}>
                ↓
              </a>
              {(isProjectOwner || a.uploaderId === currentUserId) && (
                <button type="button" className="icon-btn icon-btn--sm" onClick={() => remove(a)} title={t("common.delete")} aria-label={t("common.delete")}>
                  ✕
                </button>
              )}
            </div>
          </div>
        ))}
        {uploading.map((u) => (
          <div className="attachment attachment--uploading" key={u.key}>
            <div className="attachment-preview">
              <span className="spinner" />
            </div>
            <div className="attachment-meta">
              <span className="attachment-name">{u.name}</span>
              <small>{t("attachments.uploading")}</small>
            </div>
          </div>
        ))}
      </div>
      {error && <div className="form-error">{error}</div>}

      {preview && <Lightbox attachment={preview} onClose={() => setPreview(null)} />}
    </section>
  );
}

function Lightbox({ attachment, onClose }: { attachment: Attachment; onClose: () => void }) {
  const { t } = useTranslation();
  useEffect(() => {
    // Capture phase + stopImmediatePropagation: Escape closes only the preview, not the card modal under it.
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return;
      e.stopImmediatePropagation();
      onClose();
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [onClose]);

  return (
    <div className="lightbox" onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
      <div className="lightbox-bar">
        <span>{attachment.fileName}</span>
        <a className="btn btn-ghost btn-sm" href={attachment.downloadUrl}>
          {t("attachments.download")}
        </a>
        <button type="button" className="icon-btn" onClick={onClose} aria-label={t("common.close")}>
          ✕
        </button>
      </div>
      {attachment.kind === "image" ? (
        <img src={attachment.url} alt={attachment.fileName} />
      ) : (
        <video src={attachment.url} controls autoPlay playsInline />
      )}
    </div>
  );
}
