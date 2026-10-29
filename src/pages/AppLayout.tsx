import { useCallback, useEffect, useState } from "react";
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
          <Avatar user={user} size={30} />
          <span className="topbar-name">{user!.name}</span>
          <button className="btn btn-ghost btn-sm" onClick={logout}>
            Вийти
          </button>
        </div>
      </header>

      <div className="app-body">
        <aside className="sidebar">
          <div className="sidebar-title">Проєкти</div>
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
                  <small>
                    {p.key} · {p.memberCount} уч.
                  </small>
                </span>
              </NavLink>
            ))}
          </nav>
          <button className="btn btn-ghost btn-sm sidebar-add" onClick={() => setCreating(true)}>
            + Новий проєкт
          </button>
        </aside>

        <main className="app-main">
          {error && (
            <div className="banner-error">
              {error} <button onClick={() => setError("")}>✕</button>
            </div>
          )}
          {projects === null ? <div className="center">Завантаження…</div> : <Outlet context={ctx} />}
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
  const { projects, openCreateProject } = useLayout();
  if (projects.length > 0) {
    const last = lastProject();
    const target = projects.find((p) => p.id === last) ?? projects[0]!;
    return <Navigate to={`/projects/${target.id}`} replace />;
  }
  return (
    <div className="empty-state">
      <div className="empty-icon">▤</div>
      <h2>У вас ще немає проєктів</h2>
      <p>Створіть перший проєкт або попросіть колегу додати вас до свого.</p>
      <button className="btn btn-primary" onClick={openCreateProject}>
        Створити проєкт
      </button>
    </div>
  );
}
