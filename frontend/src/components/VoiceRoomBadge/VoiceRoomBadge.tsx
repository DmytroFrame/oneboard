import { useTranslation } from "react-i18next";
import { HeadphonesIcon } from "@/components/ui/icons";
import { cx } from "@/lib/cx";
import { useVoice } from "@/lib/voice";
import s from "./VoiceRoomBadge.module.scss";

/** "🎧 2" next to a project whose voice room has people in it (sidebar, Home). */
export function VoiceRoomBadge({ projectId }: { projectId: number }) {
  const { t } = useTranslation();
  const { rooms, call } = useVoice();
  const count = new Set((rooms[projectId] ?? []).map((p) => p.userId)).size;
  if (!count) return null;
  const mine = call?.projectId === projectId;
  return (
    <span className={cx(s.badge, mine && s.mine)} title={t("voice.inRoom", { count })}>
      <HeadphonesIcon size={12} />
      {count}
    </span>
  );
}
