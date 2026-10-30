import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import type { User } from "../types";
import { useVoice, voice, type VoiceParticipant, type VoiceProblem } from "../voice";
import { Avatar } from "./Avatar";

// ---------- icons ----------

export function MicIcon({ off = false, size = 18 }: { off?: boolean; size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <rect x="9" y="2" width="6" height="12" rx="3" />
      <path d="M5 10v1a7 7 0 0 0 14 0v-1M12 18v4" />
      {off && <path d="M3 3l18 18" />}
    </svg>
  );
}

export function HeadphonesIcon({ size = 14 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" aria-hidden="true">
      <path d="M3 18v-6a9 9 0 0 1 18 0v6" />
      <path d="M21 19a2 2 0 0 1-2 2h-1v-6h3zM3 19a2 2 0 0 0 2 2h1v-6H3z" />
    </svg>
  );
}

function HangUpIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M3.5 14.5c4.7-4.2 12.3-4.2 17 0l-2 2.6-3.2-1.3v-2.4a10 10 0 0 0-6.6 0v2.4l-3.2 1.3z" />
    </svg>
  );
}

/** Translates a voice problem: our own reasons, or a server error code. */
export function useProblemText() {
  const { t, i18n } = useTranslation();
  return (reason: VoiceProblem) => {
    const own = `voice.problems.${reason}`;
    if (i18n.exists(own)) return t(own as "voice.problems.micDenied");
    const server = `errors.${reason}`;
    return i18n.exists(server) ? t(server as "errors.voice_room_full") : t("voice.problems.failed");
  };
}

// ---------- room badge (sidebar, Home) ----------

/** "🎧 2" next to a project whose voice room has people in it. */
export function VoiceRoomBadge({ projectId }: { projectId: number }) {
  const { t } = useTranslation();
  const { rooms, call } = useVoice();
  const count = new Set((rooms[projectId] ?? []).map((p) => p.userId)).size;
  if (!count) return null;
  const mine = call?.projectId === projectId;
  return (
    <span className={`voice-badge${mine ? " voice-badge--mine" : ""}`} title={t("voice.inRoom", { count })}>
      <HeadphonesIcon size={12} />
      {count}
    </span>
  );
}

// ---------- the board's dock ----------

interface DockProps {
  projectId: number;
  me: User;
  usersById: Map<number, User>;
}

/** Bottom-right corner of a board: who is in its voice room, and joining / controlling the call. */
export function VoiceDock({ projectId, me, usersById }: DockProps) {
  const { t } = useTranslation();
  const state = useVoice();
  const problemText = useProblemText();
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
    <div className={`voice-dock${open ? " voice-dock--open" : ""}${here ? " voice-dock--in-call" : ""}`} onClick={() => voice.resumeAudio()}>
      {open && (
        <section className="voice-panel" aria-label={t("voice.title")}>
          <header className="voice-panel-head">
            <div>
              <h3>{t("voice.title")}</h3>
              <span className="voice-status">
                {here
                  ? here.status === "connected"
                    ? t("voice.connected")
                    : here.status === "joining"
                      ? t("voice.joining")
                      : t("voice.reconnecting")
                  : t("voice.inRoom", { count: participants.length })}
              </span>
            </div>
            <button type="button" className="icon-btn" aria-label={t("voice.collapse")} onClick={() => setOpen(false)}>
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
                <path d="M6 9l6 6 6-6" />
              </svg>
            </button>
          </header>

          {myProblem && (
            <div className="voice-problem" role="alert">
              {problemText(myProblem.reason)}
            </div>
          )}

          {participants.length === 0 ? (
            <p className="voice-empty muted">{t("voice.empty")}</p>
          ) : (
            <ul className="voice-people">
              {participants.map((p) => {
                const self = isMe(p);
                const link = links[p.connId];
                const pending = here && !self && link !== "connected";
                return (
                  <li key={p.connId} className={`voice-person${talking(p) ? " voice-person--speaking" : ""}${pending ? " voice-person--pending" : ""}`}>
                    <Avatar user={userOf(p)} size={32} />
                    <span className="voice-person-name">
                      {userOf(p).name}
                      {self && <small> {t("voice.you")}</small>}
                    </span>
                    {p.muted && (
                      <span className="voice-person-muted" title={t("voice.muted")}>
                        <MicIcon off size={14} />
                      </span>
                    )}
                  </li>
                );
              })}
            </ul>
          )}

          {here ? (
            <div className="voice-controls">
              {devices.length > 1 && (
                <select
                  className="voice-device"
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
              <div className="voice-buttons">
                <button
                  type="button"
                  className={`btn btn-ghost voice-mute${here.muted ? " voice-mute--on" : ""}`}
                  aria-pressed={here.muted}
                  onClick={() => voice.toggleMute()}
                >
                  <MicIcon off={here.muted} />
                  {here.muted ? t("voice.unmute") : t("voice.mute")}
                </button>
                <button type="button" className="btn btn-danger-solid voice-leave" onClick={() => voice.leave()}>
                  <HangUpIcon />
                  {t("voice.leave")}
                </button>
              </div>
            </div>
          ) : (
            <button type="button" className="btn btn-primary voice-join" disabled={call?.status === "joining"} onClick={join}>
              <MicIcon />
              {call ? t("voice.switchHere") : t("voice.join")}
            </button>
          )}
        </section>
      )}

      <button
        type="button"
        className={`voice-fab${here ? " voice-fab--live" : ""}${here && speaking.has("self") ? " voice-fab--speaking" : ""}`}
        aria-label={here ? t("voice.title") : t("voice.join")}
        aria-expanded={open}
        title={here ? t("voice.title") : t("voice.join")}
        onClick={() => setOpen((v) => !v)}
      >
        <MicIcon off={here?.muted} size={22} />
      </button>
      {!open && preview.length > 0 && (
        <button type="button" className="voice-peek" onClick={() => setOpen(true)} aria-label={t("voice.inRoom", { count: participants.length })}>
          {preview.map((p) => (
            <span key={p.connId} className={talking(p) ? "voice-peek-speaking" : undefined}>
              <Avatar user={userOf(p)} size={26} />
            </span>
          ))}
          {participants.length > preview.length && <span className="voice-peek-more">+{participants.length - preview.length}</span>}
        </button>
      )}
    </div>
  );
}

// ---------- app-wide call bar ----------

/** Shown on every page except the call's own board, so the call can be controlled from anywhere. */
export function CallBar({ projectName, onBoard }: { projectName: (id: number) => string | undefined; onBoard: (id: number) => boolean }) {
  const { t } = useTranslation();
  const { call, speaking } = useVoice();
  if (!call || onBoard(call.projectId)) return null;
  const name = projectName(call.projectId) ?? "";
  return (
    <div className={`call-bar${speaking.has("self") ? " call-bar--speaking" : ""}`} role="region" aria-label={t("voice.title")}>
      <span className={`call-bar-dot call-bar-dot--${call.status}`} />
      <Link to={`/projects/${call.projectId}`} className="call-bar-name">
        {call.status === "connected" ? t("voice.inCall", { name }) : t("voice.reconnecting")}
      </Link>
      <button type="button" className="icon-btn" aria-pressed={call.muted} aria-label={call.muted ? t("voice.unmute") : t("voice.mute")} onClick={() => voice.toggleMute()}>
        <MicIcon off={call.muted} />
      </button>
      <button type="button" className="icon-btn call-bar-leave" aria-label={t("voice.leave")} onClick={() => voice.leave()}>
        <HangUpIcon />
      </button>
    </div>
  );
}
