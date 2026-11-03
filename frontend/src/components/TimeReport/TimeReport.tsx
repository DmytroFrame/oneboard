import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { presetPeriod, ReportPeriod, type Period } from "@/components/ReportPeriod/ReportPeriod";
import { Banner } from "@/components/ui/Banner/Banner";
import { Button } from "@/components/ui/Button/Button";
import { Muted } from "@/components/ui/Muted/Muted";
import { Segmented } from "@/components/ui/Segmented/Segmented";
import { Table } from "@/components/ui/Table/Table";
import { useAuth } from "@/context/auth";
import { useLayout } from "@/layouts/AppLayout/useLayout";
import { peopleApi } from "@/lib/api/people";
import { reportsApi } from "@/lib/api/reports";
import { tasksApi } from "@/lib/api/tasks";
import { can } from "@/lib/companyRoles";
import { formatDate, formatMinutes } from "@/lib/format";
import type { User } from "@/types";
import type { ReportGroup, TimeReportFilter, TimeSum } from "@/types/reports";
import type { Milestone } from "@/types/tasks";
import s from "./TimeReport.module.scss";

const GROUPS: ReportGroup[] = ["project", "user", "milestone", "task", "day"];

/** Sums of logged time by project, person, milestone, task or day, with a CSV export of the logs. */
export function TimeReport() {
  const { t, i18n } = useTranslation();
  const { user } = useAuth();
  const { projects } = useLayout();
  const [period, setPeriod] = useState<Period>(() => presetPeriod("thisMonth"));
  const [projectId, setProjectId] = useState(0);
  const [userId, setUserId] = useState(0);
  const [milestone, setMilestone] = useState(0);
  const [billable, setBillable] = useState<TimeReportFilter["billable"]>("");
  const [group, setGroup] = useState<ReportGroup>("project");
  const [people, setPeople] = useState<User[]>([]);
  const [milestones, setMilestones] = useState<Milestone[]>([]);
  const [rows, setRows] = useState<TimeSum[] | null>(null);
  const [error, setError] = useState("");
  const crew = can(user, "viewCrewBlock");

  const filter = useMemo<TimeReportFilter>(
    () => ({ from: period.from, to: period.to, projects: projectId ? [projectId] : [], users: userId ? [userId] : [], milestone: milestone || null, billable }),
    [period.from, period.to, projectId, userId, milestone, billable],
  );

  useEffect(() => {
    if (!crew) return;
    peopleApi
      .list({ q: "", quick: "", roles: [], specialization: "", grade: "", englishLevel: "", inactive: false, sort: "name", order: "asc" }, 0, 200)
      .then((r) => setPeople(r.people))
      .catch(() => {});
  }, [crew]);
  useEffect(() => {
    setMilestone(0);
    if (projectId) tasksApi.planning(projectId).then((p) => setMilestones(p.milestones)).catch(() => setMilestones([]));
    else setMilestones([]);
  }, [projectId]);
  useEffect(() => {
    setRows(null);
    reportsApi
      .time(filter, group)
      .then((r) => (setRows(r), setError("")))
      .catch((err) => (setRows([]), setError(err.message)));
  }, [filter, group]);

  const total = rows?.reduce((a, r) => ({ minutes: a.minutes + r.minutes, billable: a.billable + r.billableMinutes, count: a.count + r.count }), { minutes: 0, billable: 0, count: 0 });
  const name = (r: TimeSum) =>
    r.day ? formatDate(r.day, i18n.language, { weekday: "short", day: "numeric", month: "short" }) : group === "milestone" && !r.id ? t("reports.noMilestone") : r.name;

  return (
    <>
      <div className={s.filters}>
        <ReportPeriod value={period} onChange={setPeriod} />
        <select value={projectId} onChange={(e) => setProjectId(Number(e.target.value))} aria-label={t("reports.project")}>
          <option value={0}>{t("reports.allProjects")}</option>
          {projects.map((p) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </select>
        {milestones.length > 0 && (
          <select value={milestone} onChange={(e) => setMilestone(Number(e.target.value))} aria-label={t("reports.milestone")}>
            <option value={0}>{t("reports.allMilestones")}</option>
            {milestones.map((m) => (
              <option key={m.id} value={m.id}>
                {m.name}
              </option>
            ))}
          </select>
        )}
        {crew && (
          <select value={userId} onChange={(e) => setUserId(Number(e.target.value))} aria-label={t("reports.person")}>
            <option value={0}>{t("reports.everyone")}</option>
            {people.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </select>
        )}
        <select value={billable} onChange={(e) => setBillable(e.target.value as TimeReportFilter["billable"])} aria-label={t("reports.billable")}>
          <option value="">{t("reports.anyBillable")}</option>
          <option value="true">{t("reports.billableOnly")}</option>
          <option value="false">{t("reports.nonBillableOnly")}</option>
        </select>
        <Button size="sm" onClick={() => reportsApi.exportTime(filter).catch((err) => setError(err.message))}>
          {t("reports.export")}
        </Button>
      </div>
      <Segmented label={t("reports.groupBy")} value={group} onChange={setGroup} options={GROUPS.map((g) => ({ id: g, label: t(`reports.groups.${g}`) }))} />
      {error && <Banner>{error}</Banner>}
      {rows && rows.length === 0 && !error && <Muted as="p">{t("reports.empty")}</Muted>}
      {rows && rows.length > 0 && (
        <Table>
          <thead>
            <tr>
              <th>{t(`reports.groups.${group}`)}</th>
              <th className={s.num}>{t("reports.logs")}</th>
              <th className={s.num}>{t("reports.billableTime")}</th>
              <th className={s.num}>{t("reports.nonBillableTime")}</th>
              <th className={s.num}>{t("reports.total")}</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => (
              <tr key={`${r.id}-${r.name}`}>
                <td>{name(r)}</td>
                <td className={s.num}>{r.count}</td>
                <td className={s.num}>{formatMinutes(r.billableMinutes)}</td>
                <td className={s.num}>{formatMinutes(r.minutes - r.billableMinutes)}</td>
                <td className={s.num}>
                  <strong>{formatMinutes(r.minutes)}</strong>
                </td>
              </tr>
            ))}
          </tbody>
          <tfoot>
            <tr>
              <td>{t("reports.total")}</td>
              <td className={s.num}>{total!.count}</td>
              <td className={s.num}>{formatMinutes(total!.billable)}</td>
              <td className={s.num}>{formatMinutes(total!.minutes - total!.billable)}</td>
              <td className={s.num}>{formatMinutes(total!.minutes)}</td>
            </tr>
          </tfoot>
        </Table>
      )}
    </>
  );
}
