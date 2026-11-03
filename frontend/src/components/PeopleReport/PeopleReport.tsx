import { Fragment, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { presetPeriod, ReportPeriod, type Period } from "@/components/ReportPeriod/ReportPeriod";
import { Banner } from "@/components/ui/Banner/Banner";
import { Muted } from "@/components/ui/Muted/Muted";
import { Segmented } from "@/components/ui/Segmented/Segmented";
import { Table } from "@/components/ui/Table/Table";
import { reportsApi } from "@/lib/api/reports";
import { COMPANY_ROLES } from "@/lib/companyRoles";
import { cx } from "@/lib/cx";
import { formatMinutes } from "@/lib/format";
import type { CompanyRole } from "@/types";
import type { PersonReport } from "@/types/reports";
import s from "./PeopleReport.module.scss";

type Mode = "billability" | "bench";
type Quick = "all" | "noLogs" | "trackedBench" | "notAllTracked";

const pct = (part: number, whole: number) => (whole ? `${Math.round((part / whole) * 100)}%` : "—");
const absenceOf = (r: PersonReport) => r.absence.vacation + r.absence.sick_day + r.absence.day_off;

/** Billability (billable share of logged time) or bench (norm that is neither billable nor absence) by person. */
export function PeopleReport({ mode }: { mode: Mode }) {
  const { t } = useTranslation();
  const [period, setPeriod] = useState<Period>(() => presetPeriod("thisMonth"));
  const [role, setRole] = useState<CompanyRole | "">("");
  const [q, setQ] = useState("");
  const [by, setBy] = useState<"people" | "projects">("people");
  const [quick, setQuick] = useState<Quick>("all");
  const [open, setOpen] = useState<Set<number>>(new Set());
  const [rows, setRows] = useState<PersonReport[] | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    setRows(null);
    reportsApi
      .people(period.from, period.to, role ? [role] : [])
      .then((r) => (setRows(r), setError("")))
      .catch((err) => (setRows([]), setError(err.message)));
  }, [period.from, period.to, role]);

  const shown = useMemo(() => {
    const needle = q.trim().toLowerCase();
    return (rows ?? []).filter((r) => {
      if (needle && !r.user.name.toLowerCase().includes(needle)) return false;
      if (mode === "bench" && quick === "noLogs") return r.minutes === 0;
      if (mode === "bench" && quick === "trackedBench") return r.benchMinutes > 0;
      if (mode === "bench" && quick === "notAllTracked") return r.notTrackedMinutes > 0;
      return true;
    });
  }, [rows, q, quick, mode]);

  const projects = useMemo(() => {
    const byId = new Map<number, { id: number; name: string; minutes: number; billable: number }>();
    for (const r of shown)
      for (const p of r.projects) {
        if (!p.billable) continue; // only projects counted in billability (OmiTeam's isBillable)
        const row = byId.get(p.id) ?? { id: p.id, name: p.name, minutes: 0, billable: 0 };
        row.minutes += p.minutes;
        row.billable += p.billableMinutes;
        byId.set(p.id, row);
      }
    return [...byId.values()].sort((a, b) => b.minutes - a.minutes);
  }, [shown]);

  const sum = (f: (r: PersonReport) => number) => shown.reduce((a, r) => a + f(r), 0);
  const toggle = (id: number) => setOpen((o) => (o.has(id) ? (o.delete(id), new Set(o)) : new Set(o).add(id)));

  return (
    <>
      <div className={s.filters}>
        <ReportPeriod value={period} onChange={setPeriod} />
        <select value={role} onChange={(e) => setRole(e.target.value as CompanyRole | "")} aria-label={t("reports.role")}>
          <option value="">{t("reports.allRoles")}</option>
          {COMPANY_ROLES.filter((r) => r !== "CANDIDATE").map((r) => (
            <option key={r} value={r}>
              {t(`companyRoles.${r}`)}
            </option>
          ))}
        </select>
        <input className={s.search} type="search" value={q} placeholder={t("reports.searchPerson")} onChange={(e) => setQ(e.target.value)} />
        <Link className={s.switch} to={mode === "bench" ? "/reports/billability" : "/reports/bench"}>
          {mode === "bench" ? t("reports.toBillability") : t("reports.toBench")} →
        </Link>
      </div>
      {mode === "billability" ? (
        <Segmented label={t("reports.groupBy")} value={by} onChange={setBy} options={[{ id: "people", label: t("reports.byPeople") }, { id: "projects", label: t("reports.byProjects") }]} />
      ) : (
        <Segmented
          label={t("reports.quick")}
          value={quick}
          onChange={setQuick}
          options={(["all", "noLogs", "trackedBench", "notAllTracked"] as Quick[]).map((id) => ({ id, label: t(`reports.quickFilters.${id}`) }))}
        />
      )}
      {error && <Banner>{error}</Banner>}
      {rows && shown.length === 0 && !error && <Muted as="p">{t("reports.empty")}</Muted>}

      {mode === "billability" && by === "projects" && projects.length > 0 && (
        <Table>
          <thead>
            <tr>
              <th>{t("reports.project")}</th>
              <th className={s.num}>{t("reports.billableTime")}</th>
              <th className={s.num}>{t("reports.nonBillableTime")}</th>
              <th className={s.num}>{t("reports.total")}</th>
              <th className={s.num}>{t("reports.billability")}</th>
            </tr>
          </thead>
          <tbody>
            {projects.map((p) => (
              <tr key={p.id}>
                <td>
                  <Link to={`/projects/${p.id}`}>{p.name}</Link>
                </td>
                <td className={s.num}>{formatMinutes(p.billable)}</td>
                <td className={s.num}>{formatMinutes(p.minutes - p.billable)}</td>
                <td className={s.num}>{formatMinutes(p.minutes)}</td>
                <td className={s.num}>
                  <strong>{pct(p.billable, p.minutes)}</strong>
                </td>
              </tr>
            ))}
          </tbody>
        </Table>
      )}

      {mode === "billability" && by === "people" && shown.length > 0 && (
        <Table>
          <thead>
            <tr>
              <th>{t("reports.person")}</th>
              <th className={s.num}>{t("reports.billableTime")}</th>
              <th className={s.num}>{t("reports.nonBillableTime")}</th>
              <th className={s.num}>{t("reports.total")}</th>
              <th className={s.num}>{t("reports.billability")}</th>
            </tr>
          </thead>
          <tbody>
            {shown.map((r) => (
              <Fragment key={r.user.id}>
                <tr className={s.clickable} onClick={() => toggle(r.user.id)} aria-expanded={open.has(r.user.id)}>
                  <td>
                    <span className={s.caret}>{r.projects.length ? (open.has(r.user.id) ? "▾" : "▸") : ""}</span>
                    {r.user.name} <small className={s.role}>{t(`companyRoles.${r.role}`)}</small>
                  </td>
                  <td className={s.num}>{formatMinutes(r.billableMinutes)}</td>
                  <td className={s.num}>{formatMinutes(r.minutes - r.billableMinutes)}</td>
                  <td className={s.num}>{formatMinutes(r.minutes)}</td>
                  <td className={s.num}>
                    <strong>{pct(r.billableMinutes, r.minutes)}</strong>
                  </td>
                </tr>
                {open.has(r.user.id) &&
                  r.projects.map((p) => (
                    <tr key={p.id} className={s.detail}>
                      <td>{p.name}</td>
                      <td className={s.num}>{formatMinutes(p.billableMinutes)}</td>
                      <td className={s.num}>{formatMinutes(p.minutes - p.billableMinutes)}</td>
                      <td className={s.num}>{formatMinutes(p.minutes)}</td>
                      <td className={s.num}>{pct(p.billableMinutes, p.minutes)}</td>
                    </tr>
                  ))}
              </Fragment>
            ))}
          </tbody>
          <tfoot>
            <tr>
              <td>{t("reports.team")}</td>
              <td className={s.num}>{formatMinutes(sum((r) => r.billableMinutes))}</td>
              <td className={s.num}>{formatMinutes(sum((r) => r.minutes - r.billableMinutes))}</td>
              <td className={s.num}>{formatMinutes(sum((r) => r.minutes))}</td>
              <td className={s.num}>{pct(sum((r) => r.billableMinutes), sum((r) => r.minutes))}</td>
            </tr>
          </tfoot>
        </Table>
      )}

      {mode === "bench" && shown.length > 0 && (
        <>
          <Table>
            <thead>
              <tr>
                <th>{t("reports.person")}</th>
                <th className={s.num}>{t("reports.norm")}</th>
                <th className={s.num}>{t("reports.billableTime")}</th>
                <th className={s.num}>{t("reports.absences")}</th>
                <th className={s.num}>{t("reports.withoutBillable")}</th>
                <th className={s.num}>{t("reports.benchPct")}</th>
                <th className={s.num}>{t("reports.trackedBench")}</th>
                <th className={s.num}>{t("reports.notTracked")}</th>
                <th className={s.num}>{t("reports.overNorm")}</th>
              </tr>
            </thead>
            <tbody>
              {shown.map((r) => (
                <tr key={r.user.id}>
                  <td>
                    {r.user.name} <small className={s.role}>{t(`companyRoles.${r.role}`)}</small>
                  </td>
                  <td className={s.num}>{formatMinutes(r.normMinutes)}</td>
                  <td className={s.num}>{formatMinutes(r.billableMinutes)}</td>
                  <td className={s.num} title={t("reports.absenceBreakdown", { vacation: formatMinutes(r.absence.vacation), sick: formatMinutes(r.absence.sick_day), dayOff: formatMinutes(r.absence.day_off) })}>
                    {formatMinutes(absenceOf(r))}
                  </td>
                  <td className={cx(s.num, r.notBillableByNorm < 0 && s.negative)}>{formatSigned(r.notBillableByNorm)}</td>
                  <td className={s.num}>
                    <strong>{pct(r.notBillableByNorm, r.normMinutes)}</strong>
                  </td>
                  <td className={s.num}>{formatMinutes(r.benchMinutes)}</td>
                  <td className={cx(s.num, r.notTrackedMinutes > 0 && s.warn)}>{formatMinutes(r.notTrackedMinutes)}</td>
                  <td className={s.num}>{formatMinutes(r.overNormMinutes)}</td>
                </tr>
              ))}
            </tbody>
            <tfoot>
              <tr>
                <td>{t("reports.team")}</td>
                <td className={s.num}>{formatMinutes(sum((r) => r.normMinutes))}</td>
                <td className={s.num}>{formatMinutes(sum((r) => r.billableMinutes))}</td>
                <td className={s.num}>{formatMinutes(sum(absenceOf))}</td>
                <td className={s.num}>{formatSigned(sum((r) => r.notBillableByNorm))}</td>
                <td className={s.num}>{pct(sum((r) => r.notBillableByNorm), sum((r) => r.normMinutes))}</td>
                <td className={s.num}>{formatMinutes(sum((r) => r.benchMinutes))}</td>
                <td className={s.num}>{formatMinutes(sum((r) => r.notTrackedMinutes))}</td>
                <td className={s.num}>{formatMinutes(sum((r) => r.overNormMinutes))}</td>
              </tr>
            </tfoot>
          </Table>
          <Muted as="p" className={s.formula}>
            {t("reports.benchFormula")}
          </Muted>
        </>
      )}
    </>
  );
}

const formatSigned = (m: number) => (m < 0 ? `−${formatMinutes(-m)}` : formatMinutes(m));
