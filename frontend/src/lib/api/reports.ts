import { ApiError, authHeaders, getAccessToken, request } from "@/lib/api";
import { i18n, translateError } from "@/i18n";
import type { CompanyRole } from "@/types";
import type { PersonReport, ReportGroup, StatsPeriod, TaskStats, TimeReportFilter, TimeSum } from "@/types/reports";

function timeQuery(f: TimeReportFilter) {
  const p = new URLSearchParams({ from: f.from, to: f.to });
  if (f.projects.length) p.set("projects", f.projects.join(","));
  if (f.users.length) p.set("users", f.users.join(","));
  if (f.milestone) p.set("milestone", String(f.milestone));
  if (f.billable) p.set("billable", f.billable);
  return p;
}

export const reportsApi = {
  time: (f: TimeReportFilter, group: ReportGroup) => {
    const p = timeQuery(f);
    p.set("group", group);
    return request<TimeSum[]>("GET", `/reports/time?${p}`);
  },
  people: (from: string, to: string, roles: CompanyRole[]) =>
    request<PersonReport[]>("GET", `/reports/people?from=${from}&to=${to}${roles.length ? `&roles=${roles.join(",")}` : ""}`),
  taskStats: (projectId: number, period: StatsPeriod, columns: number[]) =>
    request<TaskStats>("GET", `/projects/${projectId}/statistics?period=${period}${columns.length ? `&columns=${columns.join(",")}` : ""}`),

  /** Downloads the time report's logs as CSV (fetched with the token, then saved from a blob). */
  async exportTime(f: TimeReportFilter) {
    const token = await getAccessToken();
    const res = await fetch(`/api/reports/time/export?${timeQuery(f)}`, { headers: authHeaders(token) });
    if (!res.ok) {
      const data = await res.json().catch(() => null);
      throw new ApiError(res.status, translateError(data?.code, data?.error ?? i18n.t("errors.unknown", { status: res.status })), data?.code);
    }
    const url = URL.createObjectURL(await res.blob());
    const a = document.createElement("a");
    a.href = url;
    a.download = `time-report_${f.from}_${f.to}.csv`;
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  },
};
