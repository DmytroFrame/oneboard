import { i18n, translateError } from "./i18n";
import type {
  Attachment,
  Card,
  CardFilters,
  CardInput,
  CardPage,
  Column,
  CurrentUser,
  Dashboard,
  Invitation,
  Member,
  Organization,
  OrgMember,
  OrgRole,
  Project,
  Settings,
} from "./types";

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
    public code?: string,
  ) {
    super(message);
  }
}

interface SessionResponse {
  user: CurrentUser;
  accessToken: string;
  expiresAt: string;
}

// ---------- JWT session ----------
// The access token lives only in memory (never in storage, out of reach of other scripts' persistence);
// the refresh token is an HttpOnly cookie scoped to /api/auth that JavaScript cannot read at all.

let accessToken: string | null = null;
let accessExpiresAt = 0;
let refreshing: Promise<CurrentUser | null> | null = null;
let onSessionLost: (() => void) | null = null;

/** Called when the session can't be renewed any more (refresh token expired or revoked). */
export function setSessionLostHandler(handler: () => void) {
  onSessionLost = handler;
}

function setSession(s: SessionResponse | null) {
  accessToken = s?.accessToken ?? null;
  accessExpiresAt = s ? new Date(s.expiresAt).getTime() : 0;
}

/** Trades the refresh cookie for a new access token. Concurrent callers share one request. */
function refreshSession(): Promise<CurrentUser | null> {
  refreshing ??= (async () => {
    try {
      const res = await fetch("/api/auth/refresh", { method: "POST", credentials: "same-origin" });
      if (!res.ok) {
        setSession(null);
        return null;
      }
      const s = (await res.json()) as SessionResponse;
      setSession(s);
      return s.user;
    } catch {
      return null; // network error: keep the current token, the request itself will report it
    } finally {
      refreshing = null;
    }
  })();
  return refreshing;
}

/** A valid access token for the real-time socket (renewed if it is about to expire). */
export async function getAccessToken(): Promise<string | null> {
  if (!accessToken || Date.now() > accessExpiresAt - 30_000) await refreshSession();
  return accessToken;
}

// The open board's real-time connection id. Sent with requests, so the board events they cause can be
// told apart from changes made in other tabs or by other people.
let realtimeConnId: string | null = null;

export function setRealtimeConnId(id: string | null) {
  realtimeConnId = id;
}

export function getRealtimeConnId() {
  return realtimeConnId;
}

const isAuthPath = (path: string) => path.startsWith("/auth/");

async function request<T>(method: string, path: string, body?: unknown, retry = true): Promise<T> {
  // Renew a token that is about to expire instead of sending it and waiting for a 401.
  if (accessToken && !isAuthPath(path) && Date.now() > accessExpiresAt - 30_000) await refreshSession();

  const headers: Record<string, string> = {};
  if (body !== undefined && !(body instanceof FormData)) headers["Content-Type"] = "application/json";
  if (accessToken) headers.Authorization = `Bearer ${accessToken}`;
  if (realtimeConnId) headers["X-Realtime-Conn"] = realtimeConnId;

  let res: Response;
  try {
    res = await fetch(`/api${path}`, {
      method,
      credentials: "same-origin",
      headers,
      body: body instanceof FormData ? body : body !== undefined ? JSON.stringify(body) : undefined,
    });
  } catch {
    throw new ApiError(0, i18n.t("errors.network"), "network");
  }
  if (res.status === 401 && retry && !isAuthPath(path)) {
    if (await refreshSession()) return request<T>(method, path, body, false);
    onSessionLost?.();
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
  /**
   * Restores the session from the refresh cookie on page load: the user, or null when signed out.
   * Throws an ApiError with code "network" when the server can't be reached (offline start).
   */
  restoreSession: async (): Promise<CurrentUser | null> => {
    let res: Response;
    try {
      res = await fetch("/api/auth/refresh", { method: "POST", credentials: "same-origin" });
    } catch {
      throw new ApiError(0, i18n.t("errors.network"), "network");
    }
    if (!res.ok) {
      setSession(null);
      return null;
    }
    const s = (await res.json()) as SessionResponse;
    setSession(s);
    return s.user;
  },
  login: async (email: string, password: string) => {
    const s = await request<SessionResponse>("POST", "/auth/login", { email, password });
    setSession(s);
    return s.user;
  },
  register: async (name: string, email: string, password: string) => {
    const s = await request<SessionResponse>("POST", "/auth/register", { name, email, password });
    setSession(s);
    return s.user;
  },
  logout: async () => {
    try {
      await request<void>("POST", "/auth/logout");
    } finally {
      setSession(null);
    }
  },
  me: () => request<CurrentUser>("GET", "/auth/me"),

  uploadAvatar: (image: Blob) => {
    const form = new FormData();
    form.append("avatar", image, "avatar");
    return request<CurrentUser>("PUT", "/users/me/avatar", form);
  },
  removeAvatar: () => request<CurrentUser>("DELETE", "/users/me/avatar"),
  updateSettings: (settings: Settings) => request<CurrentUser>("PUT", "/users/me/settings", settings),

  dashboard: () => request<Dashboard>("GET", "/dashboard"),

  organizations: () => request<Organization[]>("GET", "/organizations"),
  organization: (id: number) => request<Organization>("GET", `/organizations/${id}`),
  createOrganization: (name: string) => request<Organization>("POST", "/organizations", { name }),
  renameOrganization: (id: number, name: string) => request<Organization>("PUT", `/organizations/${id}`, { name }),
  deleteOrganization: (id: number) => request<void>("DELETE", `/organizations/${id}`),
  orgMembers: (id: number) => request<OrgMember[]>("GET", `/organizations/${id}/members`),
  changeOrgRole: (id: number, userId: number, role: OrgRole) =>
    request<OrgMember[]>("PUT", `/organizations/${id}/members/${userId}`, { role }),
  removeOrgMember: (id: number, userId: number) => request<void>("DELETE", `/organizations/${id}/members/${userId}`),
  invitations: (id: number) => request<Invitation[]>("GET", `/organizations/${id}/invitations`),
  invite: (id: number, email: string, role: OrgRole) =>
    request<{ status: "added" | "invited" }>("POST", `/organizations/${id}/invitations`, { email, role }),
  revokeInvitation: (id: number, invitationId: number) =>
    request<void>("DELETE", `/organizations/${id}/invitations/${invitationId}`),

  projects: () => request<Project[]>("GET", "/projects"),
  createProject: (organizationId: number, name: string, key: string) =>
    request<Project>("POST", "/projects", { organizationId, name, key }),
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

  attachments: (cardId: number) => request<Attachment[]>("GET", `/cards/${cardId}/attachments`),
  uploadAttachment: (cardId: number, file: File) => {
    const form = new FormData();
    form.append("file", file, file.name);
    return request<Attachment>("POST", `/cards/${cardId}/attachments`, form);
  },
  deleteAttachment: (id: number) => request<void>("DELETE", `/attachments/${id}`),
};
