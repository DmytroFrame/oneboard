import type { OrgRole } from "./types";

// Mirrors the server's permission matrix (backend/internal/domain/organizations.go);
// the server enforces it, the UI only hides what would be refused.
export const ORG_ROLES: OrgRole[] = ["owner", "admin", "member"];

export const can = {
  invite: (r: OrgRole) => r === "owner" || r === "admin",
  manageRoles: (r: OrgRole) => r === "owner",
  createProjects: (r: OrgRole) => r === "owner" || r === "admin",
  rename: (r: OrgRole) => r === "owner" || r === "admin",
  delete: (r: OrgRole) => r === "owner",
  grant: (r: OrgRole, target: OrgRole) => (r === "owner" ? true : r === "admin" ? target !== "owner" : false),
  remove: (r: OrgRole, target: OrgRole) => (r === "owner" ? true : r === "admin" ? target === "member" : false),
};
