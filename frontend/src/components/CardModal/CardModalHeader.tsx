import { useTranslation } from "react-i18next";
import { Avatar } from "@/components/Avatar/Avatar";
import { ModalCloseButton, ModalHeader } from "@/components/ui/Modal/Modal";
import { Spacer } from "@/components/ui/Spacer/Spacer";
import type { User } from "@/types";
import s from "./CardModal.module.scss";

/** The card's key, who else has it open, the autosave status. */
export function CardModalHeader({ title, peers, onClose, status }: { title: string; peers: User[]; onClose: () => void; status?: string }) {
  const { t } = useTranslation();
  return (
    <ModalHeader>
      <span className={s.key}>{title}</span>
      {peers.length > 0 && (
        <span className={s.peers} title={t("live.alsoHere")}>
          <span className={s.peersLabel}>{t("live.alsoHere")}</span>
          {peers.map((u) => (
            <Avatar key={u.id} user={u} size={24} online />
          ))}
        </span>
      )}
      <Spacer />
      {status && <span className={s.saveStatus}>{status}</span>}
      <ModalCloseButton onClose={onClose} />
    </ModalHeader>
  );
}
