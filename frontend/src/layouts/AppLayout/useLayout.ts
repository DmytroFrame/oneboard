import { useOutletContext } from "react-router-dom";
import type { Organization, Project } from "@/types";

/** What the app shell shares with its pages (passed as the router outlet's context). */
export interface LayoutContext {
  projects: Project[];
  orgs: Organization[];
  /** Reloads organizations and projects (counts, names, membership). */
  reload: () => Promise<void>;
  openCreateProject: (orgId?: number) => void;
  openCreateOrg: () => void;
  /** Bumped when an organization's member list changed elsewhere; pages showing it reload. */
  orgMembersSignal: { orgId: number; n: number };
}

export function useLayout() {
  return useOutletContext<LayoutContext>();
}
