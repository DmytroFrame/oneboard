import type { FormEvent, ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { useEscape } from "@/hooks/useEscape";
import { cx } from "@/lib/cx";
import { IconButton } from "../IconButton/IconButton";
import s from "./Modal.module.scss";

// Building blocks for dialogs. `Modal` is the ready-made one (title + close button + Escape);
// dialogs with their own header (the card editor) compose the parts directly.

/** The dimmed layer under a dialog; pressing on it (not on the dialog) closes. */
export function ModalBackdrop({ onClose, children }: { onClose: () => void; children?: ReactNode }) {
  return (
    <div className={s.backdrop} onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
      {children}
    </div>
  );
}

interface DialogProps {
  /** narrow: forms and confirmations; wide: the card editor, project settings. */
  size?: "narrow" | "wide";
  /** Rendered as a <form> when given. */
  onSubmit?: (e: FormEvent<HTMLFormElement>) => void;
  className?: string;
  children: ReactNode;
}

export function ModalDialog({ size = "wide", onSubmit, className, children }: DialogProps) {
  const cls = cx(s.dialog, size === "narrow" && s.narrow, className);
  if (onSubmit) {
    return (
      <form className={cls} onSubmit={onSubmit}>
        {children}
      </form>
    );
  }
  return (
    <div className={cls} role="dialog">
      {children}
    </div>
  );
}

export function ModalHeader({ children }: { children: ReactNode }) {
  return <div className={s.header}>{children}</div>;
}

export function ModalCloseButton({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation();
  return (
    <IconButton onClick={onClose} aria-label={t("common.close")}>
      ✕
    </IconButton>
  );
}

/** Padded body; with onSubmit it is the dialog's form. */
export function ModalContent({ onSubmit, className, children }: { onSubmit?: (e: FormEvent<HTMLFormElement>) => void; className?: string; children: ReactNode }) {
  if (onSubmit) {
    return (
      <form className={cx(s.content, s.form, className)} onSubmit={onSubmit}>
        {children}
      </form>
    );
  }
  return <div className={cx(s.content, className)}>{children}</div>;
}

/** Right-aligned buttons at the end of a form. */
export function ModalActions({ children }: { children: ReactNode }) {
  return <div className={s.actions}>{children}</div>;
}

/** A bar under the content, separated by a line (sticky on phones). */
export function ModalFooter({ children }: { children: ReactNode }) {
  return <div className={s.footer}>{children}</div>;
}

interface ModalProps {
  title: string;
  onClose: () => void;
  wide?: boolean;
  children: ReactNode;
}

export function Modal({ title, onClose, wide, children }: ModalProps) {
  useEscape(onClose);
  return (
    <ModalBackdrop onClose={onClose}>
      <ModalDialog size={wide ? "wide" : "narrow"}>
        <ModalHeader>
          <span className={s.title}>{title}</span>
          <ModalCloseButton onClose={onClose} />
        </ModalHeader>
        {children}
      </ModalDialog>
    </ModalBackdrop>
  );
}
