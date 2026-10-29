import type { Card, CardFilters, CardInput, CardPage, Column, Member, Project, User } from "./types";

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(`/api${path}`, {
    method,
    credentials: "same-origin",
    headers: body !== undefined && !(body instanceof FormData) ? { "Content-Type": "application/json" } : undefined,
    body: body instanceof FormData ? body : body !== undefined ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) {
    let message = `Помилка ${res.status}`;
    try {
      const data = await res.json();
      if (data?.error) message = data.error;
    } catch {
      // response has no JSON body
    }
    throw new ApiError(res.status, message);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export const api = {
  me: () => request<User>("GET", "/auth/me"),
  login: (email: string, password: string) => request<User>("POST", "/auth/login", { email, password }),
  register: (name: string, email: string, password: string) =>
    request<User>("POST", "/auth/register", { name, email, password }),
  logout: () => request<void>("POST", "/auth/logout"),

  users: () => request<User[]>("GET", "/users"),
  uploadAvatar: (image: Blob) => {
    const form = new FormData();
    form.append("avatar", image, "avatar");
    return request<User>("PUT", "/users/me/avatar", form);
  },
  removeAvatar: () => request<User>("DELETE", "/users/me/avatar"),

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
