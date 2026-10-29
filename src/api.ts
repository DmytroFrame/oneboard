import type { Card, CardInput, Member, Project, Status, User } from "./types";

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
    headers: body !== undefined ? { "Content-Type": "application/json" } : undefined,
    body: body !== undefined ? JSON.stringify(body) : undefined,
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

  projects: () => request<Project[]>("GET", "/projects"),
  createProject: (name: string, key: string) => request<Project>("POST", "/projects", { name, key }),
  renameProject: (id: number, name: string) => request<Project>("PUT", `/projects/${id}`, { name }),
  deleteProject: (id: number) => request<void>("DELETE", `/projects/${id}`),
  members: (projectId: number) => request<Member[]>("GET", `/projects/${projectId}/members`),
  addMember: (projectId: number, userId: number) =>
    request<Member[]>("POST", `/projects/${projectId}/members`, { userId }),
  removeMember: (projectId: number, userId: number) =>
    request<void>("DELETE", `/projects/${projectId}/members/${userId}`),

  cards: (projectId: number) => request<Card[]>("GET", `/projects/${projectId}/cards`),
  createCard: (projectId: number, input: CardInput) => request<Card>("POST", `/projects/${projectId}/cards`, input),
  updateCard: (id: number, input: CardInput) => request<Card>("PUT", `/cards/${id}`, input),
  moveCard: (id: number, status: Status, beforeId: number | null) =>
    request<Card>("POST", `/cards/${id}/move`, { status, beforeId }),
  deleteCard: (id: number) => request<void>("DELETE", `/cards/${id}`),
};
