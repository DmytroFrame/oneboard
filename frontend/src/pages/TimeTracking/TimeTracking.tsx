import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { Page } from "@/components/Page/Page";
import { TimelogModal } from "@/components/TimelogModal/TimelogModal";
import { Banner } from "@/components/ui/Banner/Banner";
import { Button } from "@/components/ui/Button/Button";
import { Chip } from "@/components/ui/Chip/Chip";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog/ConfirmDialog";
import { Muted } from "@/components/ui/Muted/Muted";
import { Table } from "@/components/ui/Table/Table";
import { useAuth } from "@/context/auth";
import { useLayout } from "@/layouts/AppLayout/useLayout";
import { peopleApi } from "@/lib/api/people";
import { addDays, isoDay, mondayOf, notifyTimelogsChanged, TIMELOGS_CHANGED, timeApi } from "@/lib/api/time";
import { can } from "@/lib/companyRoles";
import { cx } from "@/lib/cx";
import { formatDate, formatMinutes } from "@/lib/format";
import type { Person } from "@/types/people";
import type { Timelog, WeekStats } from "@/types/time";
import s from "./TimeTracking.module.scss";

/** The weekly timesheet: what was logged on which task each day, against the norm. */
export default function TimeTracking() {
  const { t, i18n } = useTranslation();
  const { user } = useAuth();
  const { projects } = useLayout();
  const [monday, setMonday] = useState(() => mondayOf(isoDay(new Date())));
  const [userId, setUserId] = useState(user!.id);
  const [people, setPeople] = useState<Person[]>([]);
  const [logs, setLogs] = useState<Timelog[]>([]);
  const [stats, setStats] = useState<WeekStats | null>(null);
  const [editing, setEditing] = useState<{ log?: Timelog; day?: string } | null>(null);
  const [removing, setRemoving] = useState<Timelog | null>(null);
  const [missing, setMissing] = useState<string[]>([]);
  const [error, setError] = useState("");
  const seeOthers = can(user, "editWorklogs");
  const activeProjects = projects.filter((p) => p.active && p.role);

  const load = useCallback(() => {
    timeApi
      .week(monday, userId === user!.id ? undefined : userId)
      .then((w) => (setLogs(w.logs), setStats(w.stats)))
      .catch((err) => setError(err.message));
    timeApi.missing().then((m) => setMissing(m.days)).catch(() => {});
  }, [monday, userId, user]);
  useEffect(load, [load]);
  useEffect(() => {
    window.addEventListener(TIMELOGS_CHANGED, load);
    return () => window.removeEventListener(TIMELOGS_CHANGED, load);
  }, [load]);
  useEffect(() => {
    if (seeOthers) peopleApi.list({ q: "", quick: "", roles: [], specialization: "", grade: "", englishLevel: "", inactive: false, sort: "name", order: "asc" }, 0, 100)
      .then((r) => setPeople(r.people)).catch(() => {});
  }, [seeOthers]);

  const days = useMemo(() => Array.from({ length: 7 }, (_, i) => addDays(monday, i)), [monday]);
  const rows = useMemo(() => {
    const byCard = new Map<number, { log: Timelog; perDay: number[]; total: number }>();
    for (const l of logs) {
      const row = byCard.get(l.cardId) ?? { log: l, perDay: Array(7).fill(0), total: 0 };
      const i = days.indexOf(l.day);
      if (i >= 0) row.perDay[i] += l.minutes;
      row.total += l.minutes;
      byCard.set(l.cardId, row);
    }
    return [...byCard.values()];
  }, [logs, days]);

  const weekday = (day: string) => formatDate(day, i18n.language, { weekday: "short", day: "numeric", month: "numeric" });
  const mine = userId === user!.id;

  return (
    <Page>
      <div className={s.head}>
        <h1>{t("time.title")}</h1>
        <div className={s.nav}>
          <Button size="sm" onClick={() => setMonday(addDays(monday, -7))}>
            ← {t("time.prevWeek")}
          </Button>
          <Button size="sm" onClick={() => setMonday(mondayOf(isoDay(new Date())))}>
            {t("time.thisWeek")}
          </Button>
          <Button size="sm" onClick={() => setMonday(addDays(monday, 7))}>
            {t("time.nextWeek")} →
          </Button>
          {seeOthers && (
            <select value={userId} onChange={(e) => setUserId(Number(e.target.value))} aria-label={t("time.user")}>
              {[user!, ...people.filter((p) => p.id !== user!.id)].map((p) => (
                <option key={p.id} value={p.id}>
                  {p.id === user!.id ? `${p.name} (${t("time.me")})` : p.name}
                </option>
              ))}
            </select>
          )}
          <Button variant="primary" onClick={() => setEditing({})}>
            {t("time.addEntry")}
          </Button>
        </div>
      </div>

      <Muted as="p" className={s.rules}>
        {t("time.rules")}
      </Muted>
      {mine && missing.length > 0 && (
        <Banner>
          {t("time.missing")}. {t("time.missingDays", { days: missing.map((d) => formatDate(d, i18n.language, { day: "numeric", month: "short" })).join(", ") })}
        </Banner>
      )}
      {error && <Banner onDismiss={() => setError("")}>{error}</Banner>}

      <Table>
        <thead>
          <tr>
            <th>{t("time.task")}</th>
            {days.map((d, i) => {
              const st = stats?.days[i];
              return (
                <th key={d} className={cx(s.dayHead, (st?.holiday || st?.absence || i >= 5) && s.off)}>
                  {weekday(d)}
                  {st?.holiday && <div className={s.dayNote}>{st.holiday}</div>}
                  {st?.absence && <div className={s.dayNote}>{t(`vacations.types.${st.absence}`)}</div>}
                  {!st?.holiday && !st?.absence && i >= 5 && <div className={s.dayNote}>{t("time.weekend")}</div>}
                </th>
              );
            })}
            <th>{t("time.total")}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.log.cardId}>
              <td className={s.task}>
                <Link to={`/projects/${r.log.projectId}?card=${r.log.cardId}`}>
                  <strong>{r.log.cardKey}</strong> {r.log.cardTitle}
                </Link>
                <small>{r.log.projectName}</small>
              </td>
              {r.perDay.map((m, i) => (
                <td key={i} className={s.cell}>
                  {m ? (
                    <button type="button" className={s.cellBtn} onClick={() => setEditing({ day: days[i] })}>
                      {formatMinutes(m)}
                    </button>
                  ) : (
                    ""
                  )}
                </td>
              ))}
              <td className={s.cell}>
                <strong>{formatMinutes(r.total)}</strong>
              </td>
            </tr>
          ))}
        </tbody>
        {stats && (
          <tfoot>
            <tr>
              <td>{t("time.total")}</td>
              {stats.days.map((d) => (
                <td key={d.day} className={cx(s.cell, d.normMinutes > 0 && d.minutes < d.normMinutes && s.short)}>
                  {d.minutes ? formatMinutes(d.minutes) : "—"}
                  {d.normMinutes > 0 && <div className={s.dayNote}>/ {formatMinutes(d.normMinutes)}</div>}
                </td>
              ))}
              <td className={s.cell}>{formatMinutes(stats.minutes)}</td>
            </tr>
          </tfoot>
        )}
      </Table>
      {logs.length === 0 && <Muted as="p">{t("time.empty")}</Muted>}
      {stats && (
        <div className={s.summary}>
          <Chip tone="accent">
            {t("time.total")}: {formatMinutes(stats.minutes)}
          </Chip>
          <Chip tone="success">
            {t("time.billable")}: {formatMinutes(stats.billableMinutes)}
          </Chip>
          <Chip>
            {t("time.norm")}: {formatMinutes(stats.normMinutes)}
          </Chip>
          {stats.overtimeMinutes > 0 && <Chip tone="warning">{t("time.overtime", { time: formatMinutes(stats.overtimeMinutes) })}</Chip>}
        </div>
      )}

      {logs.length > 0 && (
        <div className={s.entries}>
          {logs.map((l) => (
            <div key={l.id} className={s.entry}>
              <span className={s.entryDay}>{weekday(l.day)}</span>
              <span className={s.entryTask}>
                <strong>{l.cardKey}</strong> {l.cardTitle}
                {l.comment && <small> — {l.comment}</small>}
              </span>
              <span className={s.entryTime}>{formatMinutes(l.minutes)}</span>
              <Button size="sm" variant="link" onClick={() => setEditing({ log: l })}>
                ✎
              </Button>
              <Button size="sm" variant="danger" onClick={() => setRemoving(l)}>
                ✕
              </Button>
            </div>
          ))}
        </div>
      )}

      {editing && (
        <TimelogModal
          projects={activeProjects}
          log={editing.log}
          defaults={{ day: editing.day ?? (days.includes(isoDay(new Date())) ? isoDay(new Date()) : monday), userId }}
          onSaved={load}
          onClose={() => setEditing(null)}
        />
      )}
      {removing && (
        <ConfirmDialog
          title={t("time.deleteConfirm")}
          confirmLabel={t("time.deleteEntry")}
          danger
          onClose={() => setRemoving(null)}
          onConfirm={async () => {
            await timeApi.remove(removing.id);
            notifyTimelogsChanged();
          }}
        />
      )}
    </Page>
  );
}
