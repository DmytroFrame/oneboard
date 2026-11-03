import type { SpecialType } from "./index";

export interface TaskDetails {
  storyPoints: number | null;
  budgetHours: number | null;
  billable: boolean;
  specialType: SpecialType;
  milestoneId: number | null;
  sprintId: number | null;
  versionId: number | null;
  tagIds: number[];
}

export interface ChecklistItem {
  id: number;
  checklistId: number;
  text: string;
  done: boolean;
}

export interface Checklist {
  id: number;
  cardId: number;
  title: string;
  collapsed: boolean;
  items: ChecklistItem[];
}

export interface CardComment {
  id: number;
  cardId: number;
  authorId: number | null;
  authorName: string;
  body: string;
  createdAt: string;
  updatedAt: string;
}

export interface HistoryEntry {
  id: number;
  actorId: number | null;
  actorName: string;
  field: string;
  old: string;
  new: string;
  createdAt: string;
}

export interface Milestone {
  id: number;
  projectId: number;
  name: string;
  description: string;
  startDate: string | null;
  endDate: string | null;
  cardCount: number;
  doneCount: number;
}

export type SprintStatus = "new" | "current" | "past";

export interface Sprint {
  id: number;
  projectId: number;
  name: string;
  goal: string;
  startDate: string | null;
  endDate: string | null;
  status: SprintStatus;
  cardCount: number;
  doneCount: number;
}

export type VersionStatus = "in_development" | "upcoming" | "released" | "archived";

export interface Version {
  id: number;
  projectId: number;
  name: string;
  description: string;
  releaseDate: string | null;
  status: VersionStatus;
  cardCount: number;
  doneCount: number;
}

export interface Planning {
  milestones: Milestone[];
  sprints: Sprint[];
  versions: Version[];
}
