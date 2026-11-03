import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ProjectPage } from "@/components/ProjectPage/ProjectPage";
import { Banner } from "@/components/ui/Banner/Banner";
import { Chip } from "@/components/ui/Chip/Chip";
import { Muted } from "@/components/ui/Muted/Muted";
import { Panel } from "@/components/ui/Panel/Panel";
import { Segmented } from "@/components/ui/Segmented/Segmented";
import { Table } from "@/components/ui/Table/Table";
import { api } from "@/lib/api";
import { reportsApi } from "@/lib/api/reports";
import { cx } from "@/lib/cx";
import { formatDate } from "@/lib/format";
import type { Column, Project } from "@/types";
import type { StatsPeriod, TaskStats } from "@/types/reports";
import s from "./ProjectStatistics.module.scss";

export default function ProjectStatisticsRoute() {
  return <ProjectPage>{(project) => <StatisticsView key={project.id} project={project} />}</ProjectPage>;
}

type Unit = "hours" | "days";

/** Tasks created and finished per period, and how long tasks stay in the chosen columns. */
function StatisticsView({ project }: { project: Project }) {
  const { t, i18n } = useTranslation();
  const [period, setPeriod] = useState<StatsPeriod>("week");
  const [unit, setUnit] = useState<Unit>("days");
  const [columns, setColumns] = useState<Column[]>([]);
  const [chosen, setChosen] = useState<number[]>([]);
  const [stats, setStats] = useState<TaskStats | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    api.columns(project.id).then(setColumns).catch(() => {});
  }, [project.id]);
  useEffect(() => {
    reportsApi
      .taskStats(project.id, period, chosen)
      .then((r) => (setStats(r), setError("")))
      .catch((err) => setError(err.message));
  }, [project.id, period, chosen]);

  const duration = (minutes: number) => {
    if (!minutes) return "—";
    const v = unit === "hours" ? minutes / 60 : minutes / 60 / 24;
    return t(`statistics.${unit}`, { value: new Intl.NumberFormat(i18n.language, { maximumFractionDigits: 1 }).format(v) });
  };
  const label = (start: string) =>
    period === "year"
      ? start.slice(0, 4)
      : formatDate(start, i18n.language, period === "month" ? { month: "long", year: "numeric" } : { day: "numeric", month: "short" });
  const toggle = (id: number) => setChosen((c) => (c.includes(id) ? c.filter((x) => x !== id) : [...c, id]));

  return (
    <>
      <Panel compact className={s.controls}>
        <div className={s.row}>
          <Segmented
            label={t("statistics.period")}
            value={period}
            onChange={setPeriod}
            options={(["week", "month", "year"] as StatsPeriod[]).map((id) => ({ id, label: t(`statistics.periods.${id}`) }))}
          />
          <Segmented label={t("statistics.unit")} value={unit} onChange={setUnit} options={[{ id: "hours", label: t("statistics.unitHours") }, { id: "days", label: t("statistics.unitDays") }]} />
        </div>
        <div className={s.row}>
          <span className={s.label}>{t("statistics.columns")}</span>
          {columns.map((c) => (
            <button key={c.id} type="button" className={cx(s.column, chosen.includes(c.id) && s.on)} aria-pressed={chosen.includes(c.id)} onClick={() => toggle(c.id)}>
              {c.name}
            </button>
          ))}
          {chosen.length === 0 && <Muted>{t("statistics.columnsDefault")}</Muted>}
        </div>
      </Panel>
      {error && <Banner>{error}</Banner>}
      {stats && (
        <>
          <div className={s.overall}>
            <span>{t("statistics.overall")}</span>
            <strong>{duration(stats.avgMinutes)}</strong>
            <Chip>{t("statistics.tasks", { count: stats.stays })}</Chip>
          </div>
          <Table>
            <thead>
              <tr>
                <th>{t("statistics.periodCol")}</th>
                <th className={s.num}>{t("statistics.created")}</th>
                <th className={s.num}>{t("statistics.done")}</th>
                <th className={s.num}>{t("statistics.left")}</th>
                <th className={s.num}>{t("statistics.average")}</th>
              </tr>
            </thead>
            <tbody>
              {[...stats.buckets].reverse().map((b) => (
                <tr key={b.start}>
                  <td>{label(b.start)}</td>
                  <td className={s.num}>{b.created}</td>
                  <td className={s.num}>{b.done}</td>
                  <td className={s.num}>{b.stays}</td>
                  <td className={s.num}>{duration(b.avgMinutes)}</td>
                </tr>
              ))}
            </tbody>
          </Table>
        </>
      )}
    </>
  );
}
