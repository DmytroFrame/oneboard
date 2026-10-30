import { i18n, translateError } from "./i18n";
import type { Card, CardFilters, CardInput, CardPage, Column, CurrentUser, Member, Project, Settings, User } from "./types";

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
    public code?: string,
  ) {
    super(message);
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  let res: Response;
  try {
    res = await fetch(`/api${path}`, {
      method,
      credentials: "same-origin",
      headers: body !== undefined && !(body instanceof FormData) ? { "Content-Type": "application/json" } : undefined,
      body: body instanceof FormData ? body : body !== undefined ? JSON.stringify(body) : undefined,
    });
  } catch {
    throw new ApiError(0, i18n.t("errors.network"), "network");
  }
  if (!res.ok) {
    // The server sends {error: English message, code}; show the translation of the code.
    let message = i18n.t("errors.unknown", { status: res.status });
    let code: string | undefined;
    try {
      const data = await res.json();
      code = data?.code;
      message = translateError(code, data?.error ?? message);
    } catch {
      // response has no JSON body
    }
    throw new ApiError(res.status, message, code);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export const api = {
  me: () => request<CurrentUser>("GET", "/auth/me"),
  login: (email: string, password: string) => request<CurrentUser>("POST", "/auth/login", { email, password }),
  register: (name: string, email: string, password: string) =>
    request<CurrentUser>("POST", "/auth/register", { name, email, password }),
  logout: () => request<void>("POST", "/auth/logout"),

  users: () => request<User[]>("GET", "/users"),
  uploadAvatar: (image: Blob) => {
    const form = new FormData();
    form.append("avatar", image, "avatar");
    return request<CurrentUser>("PUT", "/users/me/avatar", form);
  },
  removeAvatar: () => request<CurrentUser>("DELETE", "/users/me/avatar"),
  updateSettings: (settings: Settings) => request<CurrentUser>("PUT", "/users/me/settings", settings),

  projects: () => request<Project[]>("GET", "/projects"),
  createProject: (name: string, key: string) => request<Project>("POST", "/projects", { name, key }),
  renameProject: (id: number, name: string) => request<Project>("PUT", `/projects/${id}`, { name }),
  deleteProject: (id: number) => request<void>("DELETE", `/projects/${id}`),
  members: (projectId: number) => request<Member[]>("GET", `/projects/${projectId}/members`),
  addMember: (projectId: number, userId: number) =>
    request<Member[]>("POST", `/projects/${projectId}/members`, { userId }),
  removeMember: (projectId: number, userId: number) =>
    request<void>("DELETE", `/projects/${projectId}/members/${userId}`),

  columns: (projectId: number) => request<Column[]>("GET", `/projects/${projectId}/columns`),
  createColumn: (projectId: number, name: string) => request<Column[]>("POST", `/projects/${projectId}/columns`, { name }),
  renameColumn: (id: number, name: string) => request<Column[]>("PUT", `/columns/${id}`, { name }),
  moveColumn: (id: number, index: number) => request<Column[]>("POST", `/columns/${id}/move`, { index }),
  deleteColumn: (id: number, moveTo: number | null) =>
    request<Column[]>("DELETE", `/columns/${id}${moveTo ? `?moveTo=${moveTo}` : ""}`),

  cards: (projectId: number, columnId: number, filters: CardFilters, cursor: string | null, limit = 20) => {
    const params = new URLSearchParams({ column: String(columnId), limit: String(limit) });
    if (cursor) params.set("cursor", cursor);
    if (filters.q.trim()) params.set("q", filters.q.trim());
    if (filters.assignees.length) params.set("assignees", filters.assignees.join(","));
    if (filters.unassigned) params.set("unassigned", "1");
    return request<CardPage>("GET", `/projects/${projectId}/cards?${params}`);
  },
  createCard: (projectId: number, input: CardInput) => request<Card>("POST", `/projects/${projectId}/cards`, input),
  updateCard: (id: number, input: CardInput) => request<Card>("PUT", `/cards/${id}`, input),
  moveCard: (id: number, columnId: number, place: { beforeId?: number; afterId?: number } = {}) =>
    request<Card>("POST", `/cards/${id}/move`, { columnId, ...place }),
  deleteCard: (id: number) => request<void>("DELETE", `/cards/${id}`),
};
