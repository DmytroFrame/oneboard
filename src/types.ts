export type Status = "todo" | "in_progress" | "review" | "done";
export type Priority = "low" | "medium" | "high" | "critical";

export interface User {
  id: number;
  name: string;
  email: string;
  color: string;
}

export type Role = "owner" | "member";

export interface Member extends User {
  role: Role;
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
  key: string;
  title: string;
  description: string;
  status: Status;
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
  status?: Status;
}

export const COLUMNS: { id: Status; title: string }[] = [
  { id: "todo", title: "До виконання" },
  { id: "in_progress", title: "В роботі" },
  { id: "review", title: "На перевірці" },
  { id: "done", title: "Готово" },
];

export const PRIORITIES: { id: Priority; title: string }[] = [
  { id: "low", title: "Низький" },
  { id: "medium", title: "Середній" },
  { id: "high", title: "Високий" },
  { id: "critical", title: "Критичний" },
];
