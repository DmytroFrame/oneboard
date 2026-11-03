export interface Timelog {
  id: number;
  userId: number;
  userName: string;
  cardId: number;
  cardKey: string;
  cardTitle: string;
  projectId: number;
  projectName: string;
  day: string; // YYYY-MM-DD
  minutes: number;
  comment: string;
  billable: boolean;
  createdBy: number | null;
}

export interface DayStats {
  day: string;
  minutes: number;
  billableMinutes: number;
  normMinutes: number;
  holiday: string;
  absence: VacationType | "";
}

export interface WeekStats {
  days: DayStats[];
  minutes: number;
  billableMinutes: number;
  normMinutes: number;
  overtimeMinutes: number;
}

export interface TimelogInput {
  userId?: number;
  cardId: number;
  day: string;
  minutes: number;
  comment: string;
}

export interface Holiday {
  id: number;
  day: string;
  nameLatin: string;
  nameCyrillic: string;
}

export type VacationType = "vacation" | "sick_day" | "day_off";
export const VACATION_TYPES: VacationType[] = ["vacation", "sick_day", "day_off"];
export type VacationStatus = "pending" | "approved" | "declined";

export interface Vacation {
  id: number;
  userId: number;
  userName: string;
  type: VacationType;
  start: string;
  end: string;
  days: number;
  comment: string;
  status: VacationStatus;
  decidedBy: number | null;
  decidedAt: string | null;
  createdAt: string;
  reviewerIds: number[];
}

export interface VacationInput {
  type: VacationType;
  start: string;
  end: string;
  comment: string;
  reviewerIds: number[];
}

export interface VacationStats {
  year: number;
  taken: Record<VacationType, number>;
  left: Record<VacationType, number>;
  allowance: Record<VacationType, number>;
}
