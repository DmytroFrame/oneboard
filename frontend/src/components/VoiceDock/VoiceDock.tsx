import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Avatar } from "@/components/Avatar/Avatar";
import { Button } from "@/components/ui/Button/Button";
import { IconButton } from "@/components/ui/IconButton/IconButton";
import { ChevronDownIcon, HangUpIcon, MicIcon } from "@/components/ui/icons";
import { Muted } from "@/components/ui/Muted/Muted";
import { useVoiceProblemText } from "@/hooks/useVoiceProblemText";
import { cx } from "@/lib/cx";
import { useVoice, voice, type VoiceParticipant } from "@/lib/voice";
import type { User } from "@/types";
import s from "./VoiceDock.module.scss";

interface Props {
  projectId: number;
  me: User;
  usersById: Map<number, User>;
}

/** Bottom-right corner of a board: who is in its voice room, and joining / controlling the call. */
export function VoiceDock({ projectId, me, usersById }: Props) {
  const { t } = useTranslation();
  const state = useVoice();
  const problemText = useVoiceProblemText();
  const { call, speaking, links, devices, deviceId, problem } = state;
  const here = call?.projectId === projectId ? call : null;
  const participants = state.rooms[projectId] ?? [];
  const [open, setOpen] = useState(false);

  // Open the panel when joining here; problems for this board are shown in it.
  useEffect(() => {
    if (here) setOpen(true);
  }, [here?.projectId]); // eslint-disable-line react-hooks/exhaustive-deps
  const myProblem = problem?.projectId === projectId ? problem : null;
  useEffect(() => {
    if (!myProblem) return;
    setOpen(true);
    const timer = setTimeout(() => voice.clearProblem(), 8000);
    return () => clearTimeout(timer);
  }, [myProblem]);

  const userOf = (p: VoiceParticipant): User => usersById.get(p.userId) ?? (p.userId === me.id ? me : { id: p.userId, name: p.name, email: "", color: "#8E8E93", avatarUrl: null });
  const isMe = (p: VoiceParticipant) => p.userId === me.id;
  const talking = (p: VoiceParticipant) => (isMe(p) && here ? speaking.has("self") : speaking.has(p.connId));
  const preview = participants.slice(0, 4);

  const join = () => {
    voice.resumeAudio();
    void voice.join(projectId);
  };

  return (
    <div className={cx(s.dock, here && s.inCall)} onClick={() => voice.resumeAudio()}>
      {open && (
        <section className={s.panel} aria-label={t("voice.title")}>
          <header className={s.head}>
            <div>
              <h3>{t("voice.title")}</h3>
              <span className={s.status}>
                {here
                  ? here.status === "connected"
                    ? t("voice.connected")
                    : here.status === "joining"
                      ? t("voice.joining")
                      : t("voice.reconnecting")
                  : t("voice.inRoom", { count: participants.length })}
              </span>
            </div>
            <IconButton aria-label={t("voice.collapse")} onClick={() => setOpen(false)}>
              <ChevronDownIcon />
            </IconButton>
          </header>

          {myProblem && (
            <div className={s.problem} role="alert">
              {problemText(myProblem.reason)}
            </div>
          )}

          {participants.length === 0 ? (
            <Muted as="p" className={s.empty}>
              {t("voice.empty")}
            </Muted>
          ) : (
            <ul className={s.people}>
              {participants.map((p) => {
                const self = isMe(p);
                const pending = here && !self && links[p.connId] !== "connected";
                return (
                  <li key={p.connId} className={cx(s.person, talking(p) && s.speaking, pending && s.pending)}>
                    <Avatar user={userOf(p)} size={32} className={s.personAvatar} />
                    <span className={s.personName}>
                      {userOf(p).name}
                      {self && <small> {t("voice.you")}</small>}
                    </span>
                    {p.muted && (
                      <span className={s.muted} title={t("voice.muted")}>
                        <MicIcon off size={14} />
                      </span>
                    )}
                  </li>
                );
              })}
            </ul>
          )}

          {here ? (
            <div className={s.controls}>
              {devices.length > 1 && (
                <select
                  className={s.device}
                  aria-label={t("voice.microphone")}
                  value={devices.some((d) => d.deviceId === deviceId) ? deviceId : ""}
                  onChange={(e) => void voice.setDevice(e.target.value)}
                >
                  <option value="">{t("voice.defaultMic")}</option>
                  {devices.map((d, i) => (
                    <option key={d.deviceId} value={d.deviceId}>
                      {d.label || t("voice.micN", { n: i + 1 })}
                    </option>
                  ))}
                </select>
              )}
              <div className={s.buttons}>
                <Button type="button" className={cx(here.muted && s.mutedOn)} aria-pressed={here.muted} onClick={() => voice.toggleMute()}>
                  <MicIcon off={here.muted} />
                  {here.muted ? t("voice.unmute") : t("voice.mute")}
                </Button>
                <Button type="button" variant="dangerSolid" onClick={() => voice.leave()}>
                  <HangUpIcon />
                  {t("voice.leave")}
                </Button>
              </div>
            </div>
          ) : (
            <Button type="button" variant="primary" block disabled={call?.status === "joining"} onClick={join}>
              <MicIcon />
              {call ? t("voice.switchHere") : t("voice.join")}
            </Button>
          )}
        </section>
      )}

      <button
        type="button"
        className={cx(s.fab, here && s.fabLive, here && speaking.has("self") && s.fabSpeaking)}
        aria-label={here ? t("voice.title") : t("voice.join")}
        aria-expanded={open}
        title={here ? t("voice.title") : t("voice.join")}
        onClick={() => setOpen((v) => !v)}
      >
        <MicIcon off={here?.muted} size={22} />
      </button>
      {!open && preview.length > 0 && (
        <button type="button" className={s.peek} onClick={() => setOpen(true)} aria-label={t("voice.inRoom", { count: participants.length })}>
          {preview.map((p) => (
            <span key={p.connId} className={cx(s.peekItem, talking(p) && s.speaking)}>
              <Avatar user={userOf(p)} size={26} className={s.peekAvatar} />
            </span>
          ))}
          {participants.length > preview.length && <span className={s.peekMore}>+{participants.length - preview.length}</span>}
        </button>
      )}
    </div>
  );
}
