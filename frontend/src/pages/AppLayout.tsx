import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { NavLink, Outlet, useLocation, useNavigate, useOutletContext } from "react-router-dom";
import { api } from "../api";
import { useAuth } from "../auth";
import { Avatar } from "../components/Avatar";
import { Logo } from "../components/Logo";
import { CreateOrgModal, CreateProjectModal } from "../components/ProjectModals";
import { can } from "../orgs";
import { BoardSocket, type UserEvent } from "../realtime";
import type { Organization, Project } from "../types";

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
  const [notice, setNotice] = useState<{ text: string; tone: "info" | "warn" } | null>(null);
  const [orgMembersSignal, setOrgMembersSignal] = useState({ orgId: 0, n: 0 });
  const location = useLocation();
  const pathRef = useRef(location.pathname);
  pathRef.current = location.pathname;
  const projectsRef = useRef(projects);
  projectsRef.current = projects;
  // Phones: the sidebar is a drawer, closed on every navigation.
  const [drawerOpen, setDrawerOpen] = useState(false);
  useEffect(() => setDrawerOpen(false), [location.pathname]);
  useEffect(() => {
    if (!drawerOpen) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setDrawerOpen(false);
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [drawerOpen]);

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

  const [online, setOnline] = useState(navigator.onLine);
  useEffect(() => {
    const on = () => {
      setOnline(true);
      void reload(); // whatever failed while offline
    };
    const off = () => setOnline(false);
    window.addEventListener("online", on);
    window.addEventListener("offline", off);
    return () => {
      window.removeEventListener("online", on);
      window.removeEventListener("offline", off);
    };
  }, [reload]);

  useEffect(() => {
    if (!notice) return;
    const timer = setTimeout(() => setNotice(null), 6000);
    return () => clearTimeout(timer);
  }, [notice]);

  // The user's own real-time channel: access granted or taken away anywhere in the app.
  useEffect(() => {
    const socket = new BoardSocket(0);
    const off = socket.on("user", (m) => {
      const e = m.event as UserEvent;
      if (e.type === "org.members") {
        setOrgMembersSignal((s) => ({ orgId: e.orgId, n: s.n + 1 }));
        void reload(); // member counts
        return;
      }
      void reload();
      const self = e.actorId === user!.id;
      const name = e.name;
      if (e.reason === "added" && !self) setNotice({ text: t("live.addedTo", { name }), tone: "info" });
      if (e.reason === "role" && !self) setNotice({ text: t("live.roleChanged", { name }), tone: "info" });
      if (e.reason === "removed" || e.reason === "deleted") {
        // Leave a page we can no longer see.
        const path = pathRef.current;
        const onIt = e.projectId
          ? path === `/projects/${e.projectId}`
          : path === `/orgs/${e.orgId}` || !!projectsRef.current?.some((p) => p.organizationId === e.orgId && path === `/projects/${p.id}`);
        if (!self) setNotice({ text: t(e.reason === "deleted" ? "live.deleted" : "live.removedFrom", { name }), tone: "warn" });
        if (onIt) navigate("/home", { replace: true });
      }
    });
    return () => {
      off();
      socket.close();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [reload]);

  const ctx: LayoutContext = {
    projects: projects ?? [],
    orgs,
    reload,
    openCreateProject: (orgId) => setCreatingProject({ orgId }),
    openCreateOrg: () => setCreatingOrg(true),
    orgMembersSignal,
  };

  return (
    <div className={`app${drawerOpen ? " app--drawer" : ""}`}>
      <header className="topbar">
        <button
          type="button"
          className="icon-btn menu-toggle"
          aria-label={t("nav.menu")}
          aria-expanded={drawerOpen}
          onClick={() => setDrawerOpen((v) => !v)}
        >
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
            <path d="M4 7h16M4 12h16M4 17h16" />
          </svg>
        </button>
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

      {!online && <div className="offline-bar">{t("pwa.offline")}</div>}

      <div className="app-body">
        {drawerOpen && <div className="drawer-backdrop" onClick={() => setDrawerOpen(false)} />}
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

        {notice && (
          <div className={`toast toast--${notice.tone}`} role="status" onClick={() => setNotice(null)}>
            {notice.text}
          </div>
        )}
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
