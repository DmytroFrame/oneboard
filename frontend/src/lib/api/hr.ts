import { request } from "@/lib/api";
import type { CompanyRole, User } from "@/types";
import type { Candidate, CandidateFilters, CandidateInput, CandidateStatus, HRStats } from "@/types/hr";

export const hrApi = {
  candidates: (f: CandidateFilters, offset: number, limit = 30) => {
    const p = new URLSearchParams({ offset: String(offset), limit: String(limit) });
    if (f.q.trim()) p.set("q", f.q.trim());
    if (f.status) p.set("status", f.status);
    if (f.attribute) p.set("attribute", f.attribute);
    if (f.specialization) p.set("specialization", f.specialization);
    if (f.recruiter) p.set("recruiter", String(f.recruiter));
    return request<{ candidates: Candidate[]; total: number }>("GET", `/candidates?${p}`);
  },
  candidate: (id: number) => request<Candidate>("GET", `/candidates/${id}`),
  update: (id: number, input: CandidateInput) => request<Candidate>("PUT", `/candidates/${id}`, input),
  setStatus: (id: number, status: CandidateStatus) => request<Candidate>("PUT", `/candidates/${id}/status`, { status }),
  reject: (id: number, reason: string) => request<Candidate>("PUT", `/candidates/${id}/rejection`, { reason }),
  hire: (id: number, role: CompanyRole) => request<User>("POST", `/candidates/${id}/hire`, { role }),
  stats: (year: number, from: number, to: number) => request<HRStats>("GET", `/hr/statistics?year=${year}&from=${from}&to=${to}`),
  deleteEvent: (id: number) => request<void>("DELETE", `/hr/events/${id}`),
};
