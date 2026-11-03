import type { CompanyRole, CurrentUser, Permission } from "@/types";

// Company roles decide which parts of the app someone sees. Permissions come from the server
// (CurrentUser.permissions), so the matrix itself is not repeated here; this file only maps roles to
// sections of the app, which is UI navigation and has no server-side counterpart.

export const COMPANY_ROLES: CompanyRole[] = [
  "OWNER", "ADMIN", "MANAGER", "RECRUITER", "HR", "SALES", "EMPLOYEE", "CONTRACTOR", "ACCOUNTANT", "CANDIDATE",
];

/** Areas of the app, each guarded as a whole. */
export type Section = "home" | "projects" | "team" | "profile" | "settings" | "time" | "vacations" | "reports" | "candidates" | "hrStatistics" | "wiki" | "backwoods";

// Candidates only see their own profile; everyone else uses the boards. Further sections (time
// tracking, team, vacations, reports, wiki…) are added here as they are built.
const STAFF: Section[] = ["home", "projects", "time", "vacations", "reports", "team", "candidates", "hrStatistics", "wiki", "profile", "settings"];
const SECTIONS: Record<CompanyRole, Section[]> = {
  OWNER: STAFF,
  ADMIN: STAFF,
  MANAGER: STAFF,
  RECRUITER: STAFF,
  HR: STAFF,
  SALES: STAFF,
  EMPLOYEE: STAFF,
  ACCOUNTANT: STAFF,
  CONTRACTOR: ["home", "projects", "time", "vacations", "reports", "profile", "settings"],
  CANDIDATE: ["profile", "settings"],
};

export function can(user: CurrentUser | null | undefined, permission: Permission) {
  return !!user?.permissions.includes(permission);
}

export function canAccess(user: CurrentUser | null | undefined, section: Section) {
  // Gated by a permission rather than the role's sections (docs/omiteam-integration.md, decision 6).
  // Platform admins run the installation's tools; organization owners and admins its data checks.
  if (section === "backwoods") return !!user?.platformAdmin || can(user, "openBackwoods");
  if (section === "candidates") return can(user, "viewCandidates");
  if (section === "hrStatistics") return can(user, "accessStatistics");
  // Without an organization ("" role) only settings and the home page (to create one) are open.
  if (user && !user.role) return section === "settings" || section === "home";
  return !!user && (SECTIONS[user.role as CompanyRole] ?? []).includes(section);
}

/** Where "home" is for this user: the dashboard, or the profile for candidates. */
export function homeRoute(user: CurrentUser | null | undefined) {
  return canAccess(user, "home") ? "/home" : "/profile";
}
