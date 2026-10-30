import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { NavLink, Outlet, useNavigate, useOutletContext } from "react-router-dom";
import { api } from "../api";
import { useAuth } from "../auth";
import { Avatar } from "../components/Avatar";
import { Logo } from "../components/Logo";
import { CreateOrgModal, CreateProjectModal } from "../components/ProjectModals";
import { can } from "../orgs";
import type { Organization, Project } from "../types";

export interface LayoutContext {
  projects: Project[];
  orgs: Organization[];
  /** Reloads organizations and projects (counts, names, membership). */
  reload: () => Promise<void>;
  openCreateProject: (orgId?: number) => void;
  openCreateOrg: () => void;
}

export function useLayout() {
  return useOutletContext<LayoutContext>();
}

const linkClass = (base: string) => ({ isActive }: { isActive: boolean }) => `${base}${isActive ? ` ${base}--active` : ""}`;

export default function AppLayout() {
  const { t } = useTranslation();
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const [projects, setProjects] = useState<Project[] | null>(null);
  const [orgs, setOrgs] = useState<Organization[]>([]);
  const [error, setError] = useState("");
  const [creatingProject, setCreatingProject] = useState<{ orgId?: number } | null>(null);
  const [creatingOrg, setCreatingOrg] = useState(false);

  const reload = useCallback(async () => {
    try {
      const [o, p] = await Promise.all([api.organizations(), api.projects()]);
      setOrgs(o);
      setProjects(p);
    } catch (err) {
      setError((err as Error).message);
      setProjects((p) => p ?? []);
    }
  }, []);

  useEffect(() => {
    reload();
  }, [reload]);

  const ctx: LayoutContext = {
    projects: projects ?? [],
    orgs,
    reload,
    openCreateProject: (orgId) => setCreatingProject({ orgId }),
    openCreateOrg: () => setCreatingOrg(true),
  };

  return (
    <div className="app">
      <header className="topbar">
        <Logo />
        <div className="spacer" />
        <div className="topbar-user">
          <Avatar user={user} size={30} onClick={() => navigate("/settings")} />
          <span className="topbar-name">{user!.name}</span>
          <button className="btn btn-ghost btn-sm" onClick={logout}>
            {t("layout.logout")}
          </button>
        </div>
      </header>

      <div className="app-body">
        <aside className="sidebar">
          <NavLink to="/home" className={linkClass("sidebar-settings")}>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M3 10.5 12 3l9 7.5V20a1 1 0 0 1-1 1h-5v-6h-6v6H4a1 1 0 0 1-1-1z" />
            </svg>
            {t("nav.home")}
          </NavLink>

          <div className="sidebar-title sidebar-title--spaced">{t("nav.organizations")}</div>
          <nav className="org-list">
            {orgs.map((o) => (
              <div className="org-group" key={o.id}>
                <NavLink to={`/orgs/${o.id}`} className={linkClass("org-link")}>
                  <span className="org-badge">{o.name.slice(0, 1).toUpperCase()}</span>
                  <span className="org-link-name">{o.name}</span>
                  <span className={`role-chip role-chip--${o.role}`}>{t(`orgRoles.${o.role}`)}</span>
                </NavLink>
                <div className="project-list">
                  {(projects ?? [])
                    .filter((p) => p.organizationId === o.id)
                    .map((p) => (
                      <NavLink key={p.id} to={`/projects/${p.id}`} className={linkClass("project-link")}>
                        <span className="project-badge">{p.key.slice(0, 2)}</span>
                        <span className="project-link-text">
                          <span className="project-link-name">{p.name}</span>
                          <small>{t("layout.projectMeta", { key: p.key, count: p.memberCount })}</small>
                        </span>
                      </NavLink>
                    ))}
                  {can.createProjects(o.role) && (
                    <button className="sidebar-add-project" onClick={() => setCreatingProject({ orgId: o.id })}>
                      {t("nav.newProject")}
                    </button>
                  )}
                </div>
              </div>
            ))}
          </nav>
          <button className="btn btn-ghost btn-sm sidebar-add" onClick={() => setCreatingOrg(true)}>
            {t("nav.newOrganization")}
          </button>

          <div className="spacer" />
          <NavLink to="/settings" className={linkClass("sidebar-settings")}>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <circle cx="12" cy="12" r="3" />
              <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
            </svg>
            {t("layout.settings")}
          </NavLink>
        </aside>

        <main className="app-main">
          {error && (
            <div className="banner-error">
              {error} <button onClick={() => setError("")}>✕</button>
            </div>
          )}
          {projects === null ? <div className="center">{t("common.loading")}</div> : <Outlet context={ctx} />}
        </main>
      </div>

      {creatingProject && (
        <CreateProjectModal
          orgs={orgs.filter((o) => can.createProjects(o.role))}
          defaultOrgId={creatingProject.orgId}
          onClose={() => setCreatingProject(null)}
          onCreated={async (p) => {
            await reload();
            navigate(`/projects/${p.id}`);
          }}
        />
      )}
      {creatingOrg && (
        <CreateOrgModal
          onClose={() => setCreatingOrg(false)}
          onCreated={async (o) => {
            await reload();
            navigate(`/orgs/${o.id}`);
          }}
        />
      )}
    </div>
  );
}
