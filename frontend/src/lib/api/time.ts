import { request } from "@/lib/api";
import type { Card, User } from "@/types";
import type { Holiday, Timelog, TimelogInput, Vacation, VacationInput, VacationStats, VacationStatus, WeekStats } from "@/types/time";

export const timeApi = {
  week: (date: string, userId?: number) =>
    request<{ logs: Timelog[]; stats: WeekStats }>("GET", `/timelogs/week?date=${date}${userId ? `&user=${userId}` : ""}`),
  missing: () => request<{ days: string[] }>("GET", "/timelogs/missing"),
  create: (input: TimelogInput) => request<Timelog>("POST", "/timelogs", input),
  update: (id: number, input: TimelogInput) => request<Timelog>("PUT", `/timelogs/${id}`, input),
  remove: (id: number) => request<void>("DELETE", `/timelogs/${id}`),
  forCard: (cardId: number, offset = 0) => request<Timelog[]>("GET", `/cards/${cardId}/timelogs?offset=${offset}`),
  findTasks: (projectId: number, q: string) => request<Card[]>("GET", `/projects/${projectId}/task-search?q=${encodeURIComponent(q)}`),

  holidays: (year: number) => request<Holiday[]>("GET", `/holidays?year=${year}`),
  saveHoliday: (h: Omit<Holiday, "id"> & { id?: number }) =>
    h.id ? request<Holiday>("PUT", `/holidays/${h.id}`, h) : request<Holiday>("POST", "/holidays", h),
  deleteHoliday: (id: number) => request<void>("DELETE", `/holidays/${id}`),

  reviewers: () => request<User[]>("GET", "/vacations/reviewers"),
  myVacations: (year: number) => request<Vacation[]>("GET", `/vacations/mine?year=${year}`),
  toReview: () => request<Vacation[]>("GET", "/vacations/review"),
  thisWeek: () => request<Vacation[]>("GET", "/vacations/week"),
  stats: (year: number, userId?: number) => request<VacationStats>("GET", `/vacations/stats?year=${year}${userId ? `&user=${userId}` : ""}`),
  allVacations: (f: { year: number; month?: number; status?: string; type?: string; name?: string }) => {
    const p = new URLSearchParams({ year: String(f.year) });
    if (f.month) p.set("month", String(f.month));
    if (f.status) p.set("status", f.status);
    if (f.type) p.set("type", f.type);
    if (f.name?.trim()) p.set("name", f.name.trim());
    return request<Vacation[]>("GET", `/vacations?${p}`);
  },
  createVacation: (v: VacationInput) => request<Vacation>("POST", "/vacations", v),
  updateVacation: (id: number, v: VacationInput) => request<Vacation>("PUT", `/vacations/${id}`, v),
  setReviewers: (id: number, reviewerIds: number[]) => request<Vacation>("PUT", `/vacations/${id}/reviewers`, { reviewerIds }),
  decide: (id: number, status: Exclude<VacationStatus, "pending">) => request<Vacation>("PUT", `/vacations/${id}/status`, { status }),
  deleteVacation: (id: number) => request<void>("DELETE", `/vacations/${id}`),
};

/** Told by anything that logs time, so the menu's "missing days" dot and open weeks refresh. */
export const TIMELOGS_CHANGED = "oneboard:timelogs-changed";
export const notifyTimelogsChanged = () => window.dispatchEvent(new Event(TIMELOGS_CHANGED));

/** Local calendar date as YYYY-MM-DD. */
export function isoDay(d: Date) {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

export function addDays(day: string, n: number) {
  const [y, m, d] = day.split("-").map(Number);
  return isoDay(new Date(y!, m! - 1, d! + n));
}

export function mondayOf(day: string) {
  const [y, m, d] = day.split("-").map(Number);
  const date = new Date(y!, m! - 1, d!);
  const wd = (date.getDay() + 6) % 7;
  return addDays(day, -wd);
}
