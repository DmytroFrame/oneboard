import type { CompanyRole, User } from "@/types";

// Mirrors domain/hr.go.
export const CANDIDATE_STATUSES = [
  "new", "hr_interview", "english_check", "client_interview", "onboarding", "in_team", "blacklist", "ping_again", "contractor",
] as const;
export type CandidateStatus = (typeof CANDIDATE_STATUSES)[number];

export const CANDIDATE_ATTRIBUTES = ["hr_interview", "english_check", "ping_again", "test_task_done", "part_time", "full_time", "promising"] as const;
export type CandidateAttribute = (typeof CANDIDATE_ATTRIBUTES)[number];

export const REJECTION_REASONS = [
  "no_commercial_experience", "low_english", "salary_too_high", "stopped_responding", "wants_office",
  "failed_interview", "declined_offer", "toxic_behaviour", "overestimates_skills",
] as const;
export type RejectionReason = (typeof REJECTION_REASONS)[number];

export interface Candidate {
  user: User;
  active: boolean;
  specialization: string;
  grade: string;
  status: CandidateStatus;
  attributes: CandidateAttribute[];
  channel: string;
  minSalary: number | null;
  maxSalary: number | null;
  minRate: number | null;
  maxRate: number | null;
  hiringCommission: number | null;
  commissionReceiver: string;
  trelloLink: string;
  recruiter: User | null;
  rejectionReason: RejectionReason | "";
}

export type CandidateInput = Omit<Candidate, "user" | "active" | "specialization" | "grade" | "recruiter" | "rejectionReason"> & {
  recruiterId: number | null;
};

export interface CandidateFilters {
  q: string;
  status: CandidateStatus | "";
  attribute: CandidateAttribute | "";
  specialization: string;
  recruiter: number;
}

export interface HREvent {
  id: number;
  user: User;
  role: CompanyRole;
  kind: "hired" | "dismissed" | "rejected";
  day: string;
  fromCandidate: boolean;
  reason: string;
}

export interface HRStats {
  months: { month: number; hired: number; dismissed: number }[];
  closedSuccess: number;
  closedFail: number;
  hired: HREvent[];
  dismissed: HREvent[];
}
