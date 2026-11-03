import { request } from "@/lib/api";
import type { CommentCategory, ProjectComment, WikiAccess, WikiDraft, WikiGroup, WikiItem, WikiPage, WikiVersion } from "@/types/wiki";

export const wikiApi = {
  list: () => request<WikiItem[]>("GET", "/wiki/articles"),
  home: () => request<{ recent: WikiItem[]; changed: WikiItem[] }>("GET", "/wiki/home"),
  search: (q: string) => request<WikiItem[]>("GET", `/wiki/search?q=${encodeURIComponent(q)}`),
  get: (id: number) => request<WikiPage>("GET", `/wiki/articles/${id}`),
  create: (d: WikiDraft) => request<WikiPage>("POST", "/wiki/articles", d),
  update: (id: number, d: WikiDraft) => request<WikiPage>("PUT", `/wiki/articles/${id}`, d),
  setArchived: (id: number, archived: boolean) => request<WikiPage>("PUT", `/wiki/articles/${id}/archived`, { archived }),
  access: (id: number) => request<WikiAccess>("GET", `/wiki/articles/${id}/access`),
  setAccess: (id: number, a: WikiAccess) => request<WikiPage>("PUT", `/wiki/articles/${id}/access`, a),
  versions: (id: number) => request<WikiVersion[]>("GET", `/wiki/articles/${id}/versions`),
  version: (id: number) => request<WikiVersion>("GET", `/wiki/versions/${id}`),
  project: (projectId: number) => request<WikiItem[]>("GET", `/projects/${projectId}/wiki`),

  groups: () => request<WikiGroup[]>("GET", "/wiki/groups"),
  saveGroup: (g: { id?: number; name: string; memberIds: number[] }) =>
    g.id ? request<WikiGroup>("PUT", `/wiki/groups/${g.id}`, g) : request<WikiGroup>("POST", "/wiki/groups", g),
  groupUsage: (id: number) => request<{ articles: number }>("GET", `/wiki/groups/${id}/usage`),
  deleteGroup: (id: number) => request<void>("DELETE", `/wiki/groups/${id}`),

  comments: (projectId: number, category: CommentCategory | "") =>
    request<ProjectComment[]>("GET", `/projects/${projectId}/comments${category ? `?category=${category}` : ""}`),
  createComment: (projectId: number, c: Pick<ProjectComment, "category" | "title" | "body">) =>
    request<ProjectComment>("POST", `/projects/${projectId}/comments`, c),
  updateComment: (id: number, c: Pick<ProjectComment, "category" | "title" | "body">) => request<ProjectComment>("PUT", `/project-comments/${id}`, c),
  deleteComment: (id: number) => request<void>("DELETE", `/project-comments/${id}`),
};
