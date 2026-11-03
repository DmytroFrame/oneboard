import { useState } from "react";
import { useTranslation } from "react-i18next";
import type { HRStats } from "@/types/hr";
import s from "./HRChart.module.scss";

const W = 1000;
const H = 240;
const PAD = { top: 12, right: 8, bottom: 26, left: 28 };

/** Hires and dismissals per month, as paired bars. */
export function HRChart({ months, year }: { months: HRStats["months"]; year: number }) {
  const { t, i18n } = useTranslation();
  const [hover, setHover] = useState<number | null>(null);
  const max = Math.max(4, ...months.flatMap((m) => [m.hired, m.dismissed]));
  const step = max > 20 ? 10 : max > 8 ? 4 : 2;
  const top = Math.ceil(max / step) * step;
  const plotH = H - PAD.top - PAD.bottom;
  const slot = (W - PAD.left - PAD.right) / Math.max(1, months.length);
  const barW = Math.min(18, slot * 0.3);
  const y = (v: number) => PAD.top + plotH - (v / top) * plotH;
  const ticks = Array.from({ length: top / step + 1 }, (_, i) => i * step);
  const monthName = (m: number, style: "short" | "long") => new Intl.DateTimeFormat(i18n.language, { month: style }).format(new Date(year, m - 1, 1));
  const bar = (x: number, v: number, cls: string) => {
    if (!v) return null;
    const h = y(0) - y(v);
    const r = Math.min(4, h, barW / 2);
    return <path className={cls} d={`M${x},${y(0)} V${y(v) + r} Q${x},${y(v)} ${x + r},${y(v)} H${x + barW - r} Q${x + barW},${y(v)} ${x + barW},${y(v) + r} V${y(0)} Z`} />;
  };
  const hovered = hover !== null ? months[hover] : null;

  return (
    <div className={s.wrap}>
      <div className={s.legend}>
        <span><i className={s.hiredKey} /> {t("hrStats.hired")}</span>
        <span><i className={s.dismissedKey} /> {t("hrStats.dismissed")}</span>
      </div>
      <div className={s.chart}>
        <svg viewBox={`0 0 ${W} ${H}`} role="img" aria-label={t("hrStats.chartTitle")}>
          {ticks.map((v) => (
            <g key={v}>
              <line x1={PAD.left} x2={W - PAD.right} y1={y(v)} y2={y(v)} className={s.grid} />
              <text x={PAD.left - 6} y={y(v) + 4} textAnchor="end" className={s.tick}>
                {v}
              </text>
            </g>
          ))}
          {months.map((m, i) => {
            const cx = PAD.left + slot * i + slot / 2;
            return (
              <g key={m.month} onMouseEnter={() => setHover(i)} onMouseLeave={() => setHover(null)}>
                <rect x={PAD.left + slot * i} y={PAD.top} width={slot} height={plotH} className={s.hit} />
                {bar(cx - barW - 1, m.hired, s.hired)}
                {bar(cx + 1, m.dismissed, s.dismissed)}
                <text x={cx} y={H - 8} textAnchor="middle" className={s.tick}>
                  {monthName(m.month, "short")}
                </text>
              </g>
            );
          })}
        </svg>
        {hovered && (
          <div className={s.tooltip} style={{ left: `${((PAD.left + slot * hover! + slot / 2) / W) * 100}%` }}>
            <strong>{monthName(hovered.month, "long")}</strong>
            <span><i className={s.hiredKey} /> {t("hrStats.hired")}: {hovered.hired}</span>
            <span><i className={s.dismissedKey} /> {t("hrStats.dismissed")}: {hovered.dismissed}</span>
          </div>
        )}
      </div>
    </div>
  );
}
