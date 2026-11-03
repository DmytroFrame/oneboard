import { authHeaders, getAccessToken, request } from "@/lib/api";
import type { CompanyRole, User } from "@/types";
import type { DirectoryItem, DirectoryKind, Feedback, Person, PersonComment, PersonDocument, PersonFilters } from "@/types/people";

export type ProfileInput = Pick<
  Person,
  "name" | "nameCyrillic" | "birthday" | "startDate" | "trialUntil" | "workingHours" | "billable" | "details"
> & { finance: Person["finance"] };

export const peopleApi = {
  list: (f: PersonFilters, offset: number, limit: number) => {
    const p = new URLSearchParams({ offset: String(offset), limit: String(limit), sort: f.sort, order: f.order });
    if (f.q.trim()) p.set("q", f.q.trim());
    if (f.quick) p.set("quick", f.quick);
    if (f.roles.length) p.set("roles", f.roles.join(","));
    if (f.specialization) p.set("specialization", f.specialization);
    if (f.grade) p.set("grade", f.grade);
    if (f.englishLevel) p.set("englishLevel", f.englishLevel);
    if (f.inactive) p.set("inactive", "1");
    return request<{ people: Person[]; total: number }>("GET", `/people?${p}`);
  },
  get: (id: number) => request<Person>("GET", `/people/${id}`),
  update: (id: number, input: ProfileInput) => request<Person>("PUT", `/people/${id}`, input),
  create: (name: string, email: string, role: CompanyRole) => request<User>("POST", "/people", { name, email, role }),

  changeRole: (id: number, role: CompanyRole, additionRoles: string[] | null) =>
    request<User>("PUT", `/users/${id}/role`, { role, additionRoles }),
  resetPassword: (id: number) => request<void>("POST", `/users/${id}/password-reset`),
  dismiss: (id: number, reason: string) => request<User>("POST", `/users/${id}/dismiss`, { reason }),
  setActive: (id: number, active: boolean) => request<User>("POST", `/users/${id}/${active ? "activate" : "deactivate"}`),

  comments: (id: number) => request<PersonComment[]>("GET", `/people/${id}/comments`),
  addComment: (id: number, kind: string, body: string, anonymous: boolean) =>
    request<PersonComment>("POST", `/people/${id}/comments`, { kind, body, anonymous }),
  deleteComment: (id: number) => request<void>("DELETE", `/person-comments/${id}`),
  feedback: (id: number) => request<Feedback[]>("GET", `/people/${id}/feedback`),
  addFeedback: (id: number, f: Pick<Feedback, "startDoing" | "stopDoing" | "keepDoing" | "anonymous">) =>
    request<Feedback>("POST", `/people/${id}/feedback`, f),

  documents: (id: number) => request<PersonDocument[]>("GET", `/people/${id}/documents`),
  addDocument: (id: number, form: FormData) => request<PersonDocument>("POST", `/people/${id}/documents`, form),
  updateDocument: (doc: PersonDocument) => request<PersonDocument>("PUT", `/documents/${doc.id}`, doc),
  deleteDocument: (id: number) => request<void>("DELETE", `/documents/${id}`),
  /** Downloads a document's file (it needs the bearer token, so it can't be a plain link). */
  downloadDocument: async (doc: PersonDocument) => {
    const token = await getAccessToken();
    const res = await fetch(`/api/documents/${doc.id}/file`, { headers: authHeaders(token) });
    if (!res.ok) throw new Error(res.statusText);
    const url = URL.createObjectURL(await res.blob());
    const a = document.createElement("a");
    a.href = url;
    a.download = doc.fileName || doc.name;
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 10_000);
  },

  directory: (kind: DirectoryKind) => request<DirectoryItem[]>("GET", `/directories/${kind}`),
  addDirectoryItem: (kind: DirectoryKind, name: string) => request<DirectoryItem>("POST", `/directories/${kind}`, { name }),
  renameDirectoryItem: (id: number, name: string) => request<DirectoryItem>("PUT", `/directory-items/${id}`, { name }),
  deleteDirectoryItem: (id: number) => request<void>("DELETE", `/directory-items/${id}`),
};
