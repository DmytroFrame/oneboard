import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { IconButton } from "@/components/ui/IconButton/IconButton";
import { HangUpIcon, MicIcon } from "@/components/ui/icons";
import { cx } from "@/lib/cx";
import { useVoice, voice } from "@/lib/voice";
import s from "./CallBar.module.scss";

interface Props {
  projectName: (id: number) => string | undefined;
  onBoard: (id: number) => boolean;
}

/** Shown on every page except the call's own board, so the call can be controlled from anywhere. */
export function CallBar({ projectName, onBoard }: Props) {
  const { t } = useTranslation();
  const { call, speaking } = useVoice();
  if (!call || onBoard(call.projectId)) return null;
  const name = projectName(call.projectId) ?? "";
  return (
    <div className={cx(s.bar, speaking.has("self") && s.speaking)} role="region" aria-label={t("voice.title")}>
      <span className={cx(s.dot, call.status !== "connected" && s.dotPending)} />
      <Link to={`/projects/${call.projectId}`} className={s.name}>
        {call.status === "connected" ? t("voice.inCall", { name }) : t("voice.reconnecting")}
      </Link>
      <IconButton aria-pressed={call.muted} aria-label={call.muted ? t("voice.unmute") : t("voice.mute")} onClick={() => voice.toggleMute()}>
        <MicIcon off={call.muted} />
      </IconButton>
      <IconButton className={s.leave} aria-label={t("voice.leave")} onClick={() => voice.leave()}>
        <HangUpIcon />
      </IconButton>
    </div>
  );
}
