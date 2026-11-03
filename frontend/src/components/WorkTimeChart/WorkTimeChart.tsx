import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/Button/Button";
import { Muted } from "@/components/ui/Muted/Muted";
import { Panel } from "@/components/ui/Panel/Panel";
import { Segmented } from "@/components/ui/Segmented/Segmented";
import { useAuth } from "@/context/auth";
import { api } from "@/lib/api";
import { addDays, isoDay, mondayOf, timeApi, TIMELOGS_CHANGED } from "@/lib/api/time";
import { can } from "@/lib/companyRoles";
import { formatDate, formatMinutes } from "@/lib/format";
import type { DayWork } from "@/types";
import type { DayStats } from "@/types/time";
import s from "./WorkTimeChart.module.scss";

const W = 560;
const H = 180;
const PAD = { top: 12, right: 8, bottom: 26, left: 34 };

/** Billable vs other hours per day of a week (stacked bars), for the viewer or the whole team. */
export function WorkTimeChart() {
  const { t, i18n } = useTranslation();
  const { user } = useAuth();
  const today = isoDay(new Date());
  const [monday, setMonday] = useState(() => mondayOf(today));
  const [scope, setScope] = useState<"me" | "team">("me");
  const [days, setDays] = useState<DayWork[]>([]);
  const [todayStats, setTodayStats] = useState<DayStats | null>(null);
  const [hover, setHover] = useState<number | null>(null);
  const canTeam = can(user, "editWorklogs");

  useEffect(() => {
    const load = () => {
      api.workTime(monday, scope === "team").then(setDays).catch(() => setDays([]));
      timeApi.week(today).then((w) => setTodayStats(w.stats.days.find((d) => d.day === today) ?? null)).catch(() => {});
    };
    load();
    window.addEventListener(TIMELOGS_CHANGED, load);
    return () => window.removeEventListener(TIMELOGS_CHANGED, load);
  }, [monday, scope, today]);

  const max = Math.max(8 * 60, ...days.map((d) => d.billableMinutes + d.otherMinutes));
  const stepH = max > 40 * 60 ? 20 : max > 16 * 60 ? 4 : 2; // gridline step in hours
  const top = Math.ceil(max / 60 / stepH) * stepH * 60;
  const plotH = H - PAD.top - PAD.bottom;
  const slot = (W - PAD.left - PAD.right) / 7;
  const barW = Math.min(36, slot * 0.55);
  const y = (min: number) => PAD.top + plotH - (min / top) * plotH;
  const ticks = Array.from({ length: top / 60 / stepH + 1 }, (_, i) => i * stepH * 60);
  const total = days.reduce((a, d) => a + d.billableMinutes + d.otherMinutes, 0);
  const billable = days.reduce((a, d) => a + d.billableMinutes, 0);
  const weekday = (d: string) => formatDate(d, i18n.language, { weekday: "short" });
  const hovered = hover !== null ? days[hover] : null;

  return (
    <Panel compact className={s.panel}>
      <div className={s.head}>
        <h2>{t("company.workTime")}</h2>
        {canTeam && (
          <Segmented
            label={t("company.workTime")}
            value={scope}
            onChange={setScope}
            options={[
              { id: "me", label: t("company.mine") },
              { id: "team", label: t("company.team") },
            ]}
          />
        )}
      </div>
      {todayStats && (
        <p className={s.today}>
          {t("company.today", { logged: formatMinutes(todayStats.minutes), norm: formatMinutes(todayStats.normMinutes) })}
        </p>
      )}
      <div className={s.nav}>
        <Button size="sm" onClick={() => setMonday(addDays(monday, -7))} aria-label={t("time.prevWeek")}>
          ←
        </Button>
        <span className={s.range}>
          {formatDate(monday, i18n.language, { day: "numeric", month: "short" })} – {formatDate(addDays(monday, 6), i18n.language, { day: "numeric", month: "short" })}
        </span>
        <Button size="sm" onClick={() => setMonday(addDays(monday, 7))} aria-label={t("time.nextWeek")}>
          →
        </Button>
        <span className={s.legend}>
          <span><i className={s.billable} /> {t("company.billable")} · {formatMinutes(billable)}</span>
          <span><i className={s.other} /> {t("company.other")} · {formatMinutes(total - billable)}</span>
        </span>
      </div>
      <div className={s.chart}>
        <svg viewBox={`0 0 ${W} ${H}`} role="img" aria-label={t("company.workTime")}>
          {ticks.map((m) => (
            <g key={m}>
              <line x1={PAD.left} x2={W - PAD.right} y1={y(m)} y2={y(m)} className={s.grid} />
              <text x={PAD.left - 6} y={y(m) + 4} className={s.tick} textAnchor="end">
                {m / 60}h
              </text>
            </g>
          ))}
          {days.map((d, i) => {
            const cx = PAD.left + slot * i + slot / 2;
            const x = cx - barW / 2;
            const gap = d.billableMinutes && d.otherMinutes ? 2 : 0;
            const yb = y(d.billableMinutes);
            const yo = y(d.billableMinutes + d.otherMinutes);
            return (
              <g key={d.day} onMouseEnter={() => setHover(i)} onMouseLeave={() => setHover(null)}>
                <rect x={PAD.left + slot * i} y={PAD.top} width={slot} height={plotH} className={s.hit} />
                {d.billableMinutes > 0 && <path d={bar(x, yb, barW, y(0) - yb, d.otherMinutes === 0)} className={s.billableBar} />}
                {d.otherMinutes > 0 && <path d={bar(x, yo, barW, yb - yo - gap, true)} className={s.otherBar} />}
                <text x={cx} y={H - 8} textAnchor="middle" className={d.day === today ? s.todayTick : s.tick}>
                  {weekday(d.day)}
                </text>
              </g>
            );
          })}
        </svg>
        {hovered && (
          <div className={s.tooltip} style={{ left: `${((PAD.left + slot * hover! + slot / 2) / W) * 100}%` }}>
            <strong>{formatDate(hovered.day, i18n.language, { weekday: "long", day: "numeric", month: "long" })}</strong>
            <span><i className={s.billable} /> {t("company.billable")}: {formatMinutes(hovered.billableMinutes)}</span>
            <span><i className={s.other} /> {t("company.other")}: {formatMinutes(hovered.otherMinutes)}</span>
          </div>
        )}
      </div>
      {total === 0 && <Muted as="p" className={s.empty}>{t("company.workTimeEmpty")}</Muted>}
    </Panel>
  );
}

/** A bar with 4px rounded top corners (only when it is the top segment). */
function bar(x: number, y: number, w: number, h: number, roundTop: boolean) {
  if (h <= 0) return "";
  const r = roundTop ? Math.min(4, h, w / 2) : 0;
  return `M${x},${y + h} V${y + r} Q${x},${y} ${x + r},${y} H${x + w - r} Q${x + w},${y} ${x + w},${y + r} V${y + h} Z`;
}
