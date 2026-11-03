import { useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/Button/Button";
import { Field } from "@/components/ui/Field/Field";
import { FormError } from "@/components/ui/FormError/FormError";
import { Modal, ModalActions, ModalContent } from "@/components/ui/Modal/Modal";
import { Muted } from "@/components/ui/Muted/Muted";
import { useAuth } from "@/context/auth";
import { api } from "@/lib/api";
import { notifyTimelogsChanged, timeApi } from "@/lib/api/time";
import { cx } from "@/lib/cx";
import type { Card, Member, Project } from "@/types";
import type { Timelog } from "@/types/time";
import s from "./TimelogModal.module.scss";

interface Props {
  projects: Project[]; // where the user may log (active projects they are in)
  /** Editing an entry, or defaults for a new one. */
  log?: Timelog;
  defaults?: { projectId?: number; card?: Card; day?: string; userId?: number };
  onSaved: (log: Timelog) => void;
  onClose: () => void;
}

const MINUTES = Array.from({ length: 12 }, (_, i) => i * 5);

/** Adds or edits a time log: project, task (found or created on the spot), day, time, comment. */
export function TimelogModal({ projects, log, defaults, onSaved, onClose }: Props) {
  const { t } = useTranslation();
  const { user } = useAuth();
  const [projectId, setProjectId] = useState(log?.projectId ?? defaults?.projectId ?? projects[0]?.id ?? 0);
  const [card, setCard] = useState<Pick<Card, "id" | "key" | "title"> | null>(
    log ? { id: log.cardId, key: log.cardKey, title: log.cardTitle } : (defaults?.card ?? null),
  );
  const [query, setQuery] = useState("");
  const [found, setFound] = useState<Card[]>([]);
  const [creating, setCreating] = useState(false);
  const [newTitle, setNewTitle] = useState("");
  const [day, setDay] = useState(log?.day ?? defaults?.day ?? new Date().toISOString().slice(0, 10));
  const [hours, setHours] = useState(log ? Math.floor(log.minutes / 60) : 1);
  const [minutes, setMinutes] = useState(log ? log.minutes % 60 : 0);
  const [comment, setComment] = useState(log?.comment ?? "");
  const [userId, setUserId] = useState(log?.userId ?? defaults?.userId ?? user!.id);
  const [members, setMembers] = useState<Member[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const canLogFor = ["OWNER", "ADMIN", "MANAGER"].includes(user!.role) || user!.additionRoles.includes("LEAD");

  useEffect(() => {
    if (!projectId || !canLogFor) return;
    api.members(projectId).then(setMembers).catch(() => {});
  }, [projectId, canLogFor]);

  useEffect(() => {
    if (!projectId || card) return;
    const timer = setTimeout(() => timeApi.findTasks(projectId, query).then(setFound).catch(() => {}), 250);
    return () => clearTimeout(timer);
  }, [projectId, query, card]);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      let cardId = card?.id;
      if (!cardId && creating) {
        // A task created on the spot goes to the first column that isn't the backlog.
        const cols = await api.columns(projectId);
        const col = cols.find((c) => c.status !== "backlog") ?? cols[0];
        if (!col) throw new Error(t("time.chooseTask"));
        const created = await api.createCard(projectId, { title: newTitle, description: "", priority: "medium", assigneeId: userId, columnId: col.id });
        cardId = created.id;
      }
      if (!cardId) throw new Error(t("time.chooseTask"));
      const input = { cardId, day, minutes: hours * 60 + minutes, comment, userId: userId === user!.id ? undefined : userId };
      const saved = log ? await timeApi.update(log.id, input) : await timeApi.create(input);
      notifyTimelogsChanged();
      onSaved(saved);
      onClose();
    } catch (err) {
      setError((err as Error).message);
      setBusy(false);
    }
  }

  return (
    <Modal title={log ? t("time.editEntry") : t("time.addEntry")} onClose={onClose}>
      <ModalContent onSubmit={submit}>
        {projects.length === 0 ? (
          <Muted as="p">{t("time.noProjects")}</Muted>
        ) : (
          <Field label={t("time.project")}>
            <select value={projectId} onChange={(e) => (setProjectId(Number(e.target.value)), setCard(null), setCreating(false))}>
              {projects.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name} ({p.key})
                </option>
              ))}
            </select>
          </Field>
        )}
        {canLogFor && members.length > 0 && (
          <Field label={t("time.user")}>
            <select value={userId} onChange={(e) => setUserId(Number(e.target.value))}>
              {members.map((m) => (
                <option key={m.id} value={m.id}>
                  {m.id === user!.id ? `${m.name} (${t("time.me")})` : m.name}
                </option>
              ))}
            </select>
          </Field>
        )}
        <Field as="div" label={t("time.task")}>
          {card ? (
            <div className={s.picked}>
              <span>
                <strong>{card.key}</strong> {card.title}
              </span>
              <Button type="button" size="sm" variant="link" onClick={() => setCard(null)}>
                ✕
              </Button>
            </div>
          ) : creating ? (
            <div className={s.picked}>
              <input value={newTitle} onChange={(e) => setNewTitle(e.target.value)} placeholder={t("time.newTaskTitle")} maxLength={200} required autoFocus />
              <Button type="button" size="sm" variant="link" onClick={() => setCreating(false)}>
                ✕
              </Button>
            </div>
          ) : (
            <>
              <input value={query} onChange={(e) => setQuery(e.target.value)} placeholder={t("time.searchTask")} />
              <ul className={s.results}>
                {found.map((c) => (
                  <li key={c.id}>
                    <button type="button" className={cx(s.result)} onClick={() => setCard(c)}>
                      <strong>{c.key}</strong> {c.title}
                    </button>
                  </li>
                ))}
              </ul>
              <Button type="button" size="sm" onClick={() => setCreating(true)}>
                + {t("time.createTask")}
              </Button>
            </>
          )}
        </Field>
        <div className={s.row}>
          <Field label={t("time.date")}>
            <input type="date" value={day} onChange={(e) => setDay(e.target.value)} required />
          </Field>
          <Field label={t("time.hours")}>
            <input type="number" min={0} max={24} value={hours} onChange={(e) => setHours(Math.max(0, Number(e.target.value)))} />
          </Field>
          <Field label={t("time.minutes")}>
            <select value={minutes} onChange={(e) => setMinutes(Number(e.target.value))}>
              {MINUTES.map((m) => (
                <option key={m} value={m}>
                  {m}
                </option>
              ))}
            </select>
          </Field>
        </div>
        <Field label={t("time.comment")}>
          <textarea rows={2} value={comment} onChange={(e) => setComment(e.target.value)} maxLength={2000} />
        </Field>
        {error && <FormError>{error}</FormError>}
        <ModalActions>
          <Button type="button" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button variant="primary" disabled={busy || projects.length === 0}>
            {t("common.save")}
          </Button>
        </ModalActions>
      </ModalContent>
    </Modal>
  );
}
