import { useTranslation } from "react-i18next";
import type { VoiceProblem } from "@/lib/voice";

/** Translates a voice problem: our own reasons, or a server error code. */
export function useVoiceProblemText() {
  const { t, i18n } = useTranslation();
  return (reason: VoiceProblem) => {
    const own = `voice.problems.${reason}`;
    if (i18n.exists(own)) return t(own as "voice.problems.micDenied");
    const server = `errors.${reason}`;
    return i18n.exists(server) ? t(server as "errors.voice_room_full") : t("voice.problems.failed");
  };
}
