import { request } from "@/lib/api";
import type { Card, ColumnStatus, Tag } from "@/types";
import type { CardComment, Checklist, ChecklistItem, HistoryEntry, Milestone, Planning, Sprint, TaskDetails, Version } from "@/types/tasks";

export type BulkTarget = "column" | "backlog" | "milestone" | "sprint" | "version";

export const tasksApi = {
  setDetails: (cardId: number, d: TaskDetails) => request<Card>("PUT", `/cards/${cardId}/details`, d),
  setTemplate: (cardId: number, template: boolean) => request<Card>("PUT", `/cards/${cardId}/template`, { template }),
  templates: (projectId: number) => request<Card[]>("GET", `/projects/${projectId}/templates`),
  useTemplate: (templateId: number, columnId: number) => request<Card>("POST", `/templates/${templateId}/use`, { columnId }),
  history: (cardId: number, offset = 0) => request<HistoryEntry[]>("GET", `/cards/${cardId}/history?offset=${offset}`),

  tags: (projectId: number) => request<Tag[]>("GET", `/projects/${projectId}/tags`),
  createTag: (projectId: number, name: string, color: string) => request<Tag>("POST", `/projects/${projectId}/tags`, { name, color }),
  updateTag: (id: number, name: string, color: string) => request<Tag>("PUT", `/tags/${id}`, { name, color }),
  deleteTag: (id: number) => request<void>("DELETE", `/tags/${id}`),

  checklists: (cardId: number) => request<Checklist[]>("GET", `/cards/${cardId}/checklists`),
  createChecklist: (cardId: number, title: string) => request<Checklist>("POST", `/cards/${cardId}/checklists`, { title }),
  updateChecklist: (id: number, title: string, collapsed: boolean) => request<Checklist>("PUT", `/checklists/${id}`, { title, collapsed }),
  deleteChecklist: (id: number) => request<void>("DELETE", `/checklists/${id}`),
  addItem: (checklistId: number, text: string) => request<ChecklistItem>("POST", `/checklists/${checklistId}/items`, { text }),
  updateItem: (id: number, text: string, done: boolean) => request<ChecklistItem>("PUT", `/checklist-items/${id}`, { text, done }),
  deleteItem: (id: number) => request<void>("DELETE", `/checklist-items/${id}`),

  comments: (cardId: number) => request<CardComment[]>("GET", `/cards/${cardId}/comments`),
  addComment: (cardId: number, body: string) => request<CardComment>("POST", `/cards/${cardId}/comments`, { body }),
  updateComment: (id: number, body: string) => request<CardComment>("PUT", `/card-comments/${id}`, { body }),
  deleteComment: (id: number) => request<void>("DELETE", `/card-comments/${id}`),

  setColumnStatus: (columnId: number, status: ColumnStatus) => request<void>("PUT", `/columns/${columnId}/status`, { status }),
  bulkMove: (columnId: number, target: BulkTarget, targetId: number | null) =>
    request<{ moved: number; limit: number }>("POST", `/columns/${columnId}/bulk-move`, { target, targetId }),

  planning: async (projectId: number): Promise<Planning> => {
    const [milestones, sprints, versions] = await Promise.all([
      request<Milestone[]>("GET", `/projects/${projectId}/milestones`),
      request<Sprint[]>("GET", `/projects/${projectId}/sprints`),
      request<Version[]>("GET", `/projects/${projectId}/versions`),
    ]);
    return { milestones, sprints, versions };
  },
  saveMilestone: (m: Partial<Milestone> & { projectId: number }) =>
    m.id ? request<Milestone>("PUT", `/milestones/${m.id}`, m) : request<Milestone>("POST", `/projects/${m.projectId}/milestones`, m),
  saveSprint: (s: Partial<Sprint> & { projectId: number }) =>
    s.id ? request<Sprint>("PUT", `/sprints/${s.id}`, s) : request<Sprint>("POST", `/projects/${s.projectId}/sprints`, s),
  saveVersion: (v: Partial<Version> & { projectId: number }) =>
    v.id ? request<Version>("PUT", `/versions/${v.id}`, v) : request<Version>("POST", `/projects/${v.projectId}/versions`, v),
  deletePlanning: (kind: "milestones" | "sprints" | "versions", id: number) => request<void>("DELETE", `/${kind}/${id}`),
};
