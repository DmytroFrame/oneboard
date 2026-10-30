import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Navigate, NavLink, Outlet, useNavigate, useOutletContext } from "react-router-dom";
import { api } from "../api";
import { useAuth } from "../auth";
import { Avatar } from "../components/Avatar";
import { Logo } from "../components/Logo";
import { CreateProjectModal } from "../components/ProjectModals";
import { ThemeToggle } from "../theme";
import type { Project } from "../types";

const LAST_PROJECT_KEY = "flowboard:lastProject";

export function rememberProject(id: number) {
  try {
    localStorage.setItem(LAST_PROJECT_KEY, String(id));
  } catch {
    // storage unavailable — not critical
  }
}

function lastProject(): number | null {
  try {
    return Number(localStorage.getItem(LAST_PROJECT_KEY)) || null;
  } catch {
    return null;
  }
}

export interface LayoutContext {
  projects: Project[];
  reloadProjects: () => Promise<void>;
  openCreateProject: () => void;
}

export function useLayout() {
  return useOutletContext<LayoutContext>();
}

export default function AppLayout() {
  const { t } = useTranslation();
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const [projects, setProjects] = useState<Project[] | null>(null);
  const [error, setError] = useState("");
  const [creating, setCreating] = useState(false);

  const reloadProjects = useCallback(async () => {
    try {
      setProjects(await api.projects());
    } catch (err) {
      setError((err as Error).message);
      setProjects((p) => p ?? []);
    }
  }, []);

  useEffect(() => {
    reloadProjects();
  }, [reloadProjects]);

  const ctx: LayoutContext = { projects: projects ?? [], reloadProjects, openCreateProject: () => setCreating(true) };

  return (
    <div className="app">
      <header className="topbar">
        <Logo />
        <div className="spacer" />
        <ThemeToggle />
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
          <div className="sidebar-title">{t("layout.projects")}</div>
          <nav className="project-list">
            {(projects ?? []).map((p) => (
              <NavLink
                key={p.id}
                to={`/projects/${p.id}`}
                className={({ isActive }) => `project-link${isActive ? " project-link--active" : ""}`}
              >
                <span className="project-badge">{p.key.slice(0, 2)}</span>
                <span className="project-link-text">
                  <span className="project-link-name">{p.name}</span>
                  <small>{t("layout.projectMeta", { key: p.key, count: p.memberCount })}</small>
                </span>
              </NavLink>
            ))}
          </nav>
          <button className="btn btn-ghost btn-sm sidebar-add" onClick={() => setCreating(true)}>
            {t("layout.newProject")}
          </button>
          <div className="spacer" />
          <NavLink to="/settings" className={({ isActive }) => `sidebar-settings${isActive ? " sidebar-settings--active" : ""}`}>
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

      {creating && (
        <CreateProjectModal
          onClose={() => setCreating(false)}
          onCreated={async (p) => {
            await reloadProjects();
            navigate(`/projects/${p.id}`);
          }}
        />
      )}
    </div>
  );
}

/** /projects: jump to the last opened (or first) project, or show an empty state. */
export function ProjectsHome() {
  const { t } = useTranslation();
  const { projects, openCreateProject } = useLayout();
  if (projects.length > 0) {
    const last = lastProject();
    const target = projects.find((p) => p.id === last) ?? projects[0]!;
    return <Navigate to={`/projects/${target.id}`} replace />;
  }
  return (
    <div className="empty-state">
      <div className="empty-icon">▤</div>
      <h2>{t("layout.emptyTitle")}</h2>
      <p>{t("layout.emptyText")}</p>
      <button className="btn btn-primary" onClick={openCreateProject}>
        {t("layout.createProject")}
      </button>
    </div>
  );
}
