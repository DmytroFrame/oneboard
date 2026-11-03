import type { CompanyRole, User } from "@/types";
import type { VacationType } from "@/types/time";

export type ReportGroup = "project" | "user" | "milestone" | "task" | "day";

export interface TimeReportFilter {
  from: string;
  to: string;
  projects: number[];
  users: number[];
  milestone: number | null;
  billable: "" | "true" | "false";
}

export interface TimeSum {
  id: number;
  name: string;
  day?: string;
  minutes: number;
  billableMinutes: number;
  count: number;
}

export interface PersonProjectTime {
  id: number;
  name: string;
  billable: boolean;
  minutes: number;
  billableMinutes: number;
  benchMinutes: number;
}

/** One person in the billability and bench reports; derived values come from the server. */
export interface PersonReport {
  user: User;
  role: CompanyRole;
  normMinutes: number;
  absence: Record<VacationType, number>;
  minutes: number;
  billableMinutes: number;
  benchMinutes: number;
  notBillableByNorm: number;
  notTrackedMinutes: number;
  overNormMinutes: number;
  projects: PersonProjectTime[];
}

export type StatsPeriod = "week" | "month" | "year";

export interface TaskStats {
  buckets: { start: string; created: number; done: number; stays: number; avgMinutes: number }[];
  stays: number;
  avgMinutes: number;
}
