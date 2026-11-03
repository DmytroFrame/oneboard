import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "../Button/Button";
import { FormError } from "../FormError/FormError";
import { Modal, ModalActions, ModalContent } from "../Modal/Modal";
import s from "./ConfirmDialog.module.scss";

interface Props {
  title: string;
  text?: ReactNode;
  confirmLabel: string;
  /** Destructive actions get a red button. */
  danger?: boolean;
  /** Extra fields (e.g. a reason) between the text and the buttons. */
  children?: ReactNode;
  onConfirm: () => Promise<void> | void;
  onClose: () => void;
}

/** "Are you sure?" with a busy state and the error shown in place, instead of window.confirm. */
export function ConfirmDialog({ title, text, confirmLabel, danger, children, onConfirm, onClose }: Props) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function confirm() {
    setBusy(true);
    setError("");
    try {
      await onConfirm();
      onClose();
    } catch (err) {
      setError((err as Error).message);
      setBusy(false);
    }
  }

  return (
    <Modal title={title} onClose={onClose}>
      <ModalContent className={s.content}>
        {text && <p className={s.text}>{text}</p>}
        {children}
        {error && <FormError>{error}</FormError>}
        <ModalActions>
          <Button type="button" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button type="button" variant={danger ? "dangerSolid" : "primary"} disabled={busy} onClick={confirm}>
            {confirmLabel}
          </Button>
        </ModalActions>
      </ModalContent>
    </Modal>
  );
}
