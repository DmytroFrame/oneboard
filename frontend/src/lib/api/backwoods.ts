import { authHeaders, getAccessToken, request } from "@/lib/api";
import type { BackupState, DBSchema, ProjectFinding, RuntimeVersion } from "@/types/backwoods";

export const backwoodsApi = {
  backups: () => request<BackupState>("GET", "/backwoods/backups"),
  backup: () => request<BackupState>("POST", "/backwoods/backups"),
  schema: () => request<DBSchema>("GET", "/backwoods/schema"),
  versions: () => request<RuntimeVersion[]>("GET", "/backwoods/versions"),
  specialTasks: () => request<ProjectFinding[]>("GET", "/backwoods/special-tasks"),
  commentedProjects: () => request<ProjectFinding[]>("GET", "/backwoods/project-comments"),

  /** Saves a backup's archive (fetched with the token, then saved from a blob). */
  async download(id: number, name: string) {
    const token = await getAccessToken();
    const res = await fetch(`/api/backwoods/backups/${id}/download`, { headers: authHeaders(token) });
    if (!res.ok) throw new Error(String(res.status));
    saveBlob(await res.blob(), name);
  },
};

export function saveBlob(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = name;
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
