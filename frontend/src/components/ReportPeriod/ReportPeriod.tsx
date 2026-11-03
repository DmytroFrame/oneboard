import { useTranslation } from "react-i18next";
import { addDays, isoDay, mondayOf } from "@/lib/api/time";
import s from "./ReportPeriod.module.scss";

export type PeriodPreset = "thisWeek" | "lastWeek" | "nextWeek" | "thisMonth" | "lastMonth" | "nextMonth" | "custom";
const PRESETS: PeriodPreset[] = ["thisWeek", "lastWeek", "nextWeek", "thisMonth", "lastMonth", "nextMonth", "custom"];

export interface Period {
  preset: PeriodPreset;
  from: string;
  to: string;
}

/** The dates of a preset period around today. */
export function presetPeriod(preset: Exclude<PeriodPreset, "custom">): Period {
  const today = isoDay(new Date());
  if (preset.endsWith("Week")) {
    const shift = preset === "lastWeek" ? -7 : preset === "nextWeek" ? 7 : 0;
    const from = addDays(mondayOf(today), shift);
    return { preset, from, to: addDays(from, 6) };
  }
  const now = new Date();
  const month = now.getMonth() + (preset === "lastMonth" ? -1 : preset === "nextMonth" ? 1 : 0);
  return { preset, from: isoDay(new Date(now.getFullYear(), month, 1)), to: isoDay(new Date(now.getFullYear(), month + 1, 0)) };
}

interface Props {
  value: Period;
  onChange: (p: Period) => void;
}

/** Reports' period: a preset week or month, or any dates (up to three months, checked by the server). */
export function ReportPeriod({ value, onChange }: Props) {
  const { t } = useTranslation();
  return (
    <div className={s.period}>
      <select
        value={value.preset}
        aria-label={t("reports.period")}
        onChange={(e) => {
          const preset = e.target.value as PeriodPreset;
          onChange(preset === "custom" ? { ...value, preset } : presetPeriod(preset));
        }}
      >
        {PRESETS.map((p) => (
          <option key={p} value={p}>
            {t(`reports.periods.${p}`)}
          </option>
        ))}
      </select>
      {value.preset === "custom" && (
        <>
          <input type="date" value={value.from} aria-label={t("reports.from")} onChange={(e) => e.target.value && onChange({ ...value, from: e.target.value })} />
          <span aria-hidden="true">–</span>
          <input type="date" value={value.to} aria-label={t("reports.to")} onChange={(e) => e.target.value && onChange({ ...value, to: e.target.value })} />
        </>
      )}
    </div>
  );
}
