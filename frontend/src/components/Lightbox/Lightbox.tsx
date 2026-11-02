import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { buttonClass } from "@/components/ui/Button/Button";
import { IconButton } from "@/components/ui/IconButton/IconButton";
import type { Attachment } from "@/types";
import s from "./Lightbox.module.scss";

/** A full-screen image or video preview. */
export function Lightbox({ attachment, onClose }: { attachment: Attachment; onClose: () => void }) {
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
    <div className={s.lightbox} onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
      <div className={s.bar}>
        <span>{attachment.fileName}</span>
        <a className={buttonClass({ size: "sm" })} href={attachment.downloadUrl}>
          {t("attachments.download")}
        </a>
        <IconButton className={s.close} onClick={onClose} aria-label={t("common.close")}>
          ✕
        </IconButton>
      </div>
      {attachment.kind === "image" ? <img src={attachment.url} alt={attachment.fileName} /> : <video src={attachment.url} controls autoPlay playsInline />}
    </div>
  );
}
