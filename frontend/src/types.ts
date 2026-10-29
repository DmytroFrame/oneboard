export type Priority = "low" | "medium" | "high" | "critical";

export interface User {
  id: number;
  name: string;
  email: string;
  color: string;
  avatarUrl: string | null;
}

export type Role = "owner" | "member";

export interface Member extends User {
  role: Role;
  assignedCount: number;
}

export interface Column {
  id: number;
  projectId: number;
  name: string;
  position: number;
  cardCount: number;
}

export interface Project {
  id: number;
  name: string;
  key: string;
  ownerId: number;
  role: Role;
  memberCount: number;
  cardCount: number;
  createdAt: string;
}

export interface Card {
  id: number;
  projectId: number;
  columnId: number;
  key: string;
  title: string;
  description: string;
  priority: Priority;
  position: number;
  authorId: number;
  assigneeId: number | null;
  createdAt: string;
  updatedAt: string;
}

export interface CardInput {
  title: string;
  description: string;
  priority: Priority;
  assigneeId: number | null;
  columnId?: number;
}

export interface CardPage {
  cards: Card[];
  nextCursor: string | null;
  total: number;
}

export interface CardFilters {
  q: string;
  assignees: number[];
  unassigned: boolean;
}

export const PRIORITIES: { id: Priority; title: string }[] = [
  { id: "low", title: "Низький" },
  { id: "medium", title: "Середній" },
  { id: "high", title: "Високий" },
  { id: "critical", title: "Критичний" },
];
