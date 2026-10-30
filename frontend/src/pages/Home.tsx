import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { api } from "../api";
import { useAuth } from "../auth";
import { PRIORITY_ICON } from "../components/BoardColumn";
import { VoiceRoomBadge } from "../components/Voice";
import type { Dashboard } from "../types";
import { useLayout } from "./AppLayout";

/** The signed-in user's start page: stats, their tasks, projects and organizations. */
export default function Home() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { projects, orgs, openCreateOrg, openCreateProject } = useLayout();
  const [data, setData] = useState<Dashboard | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    api
      .dashboard()
      .then(setData)
      .catch((err) => setError((err as Error).message));
  }, [projects]); // refresh when the layout reloads projects (e.g. after creating one)

  const firstName = user!.name.split(/\s+/)[0];
  const stats = data?.stats;
  const tiles = [
    { label: t("home.statOrganizations"), value: stats?.organizations, tone: "violet" },
    { label: t("home.statProjects"), value: stats?.projects, tone: "blue" },
    { label: t("home.statAssigned"), value: stats?.assigned, tone: "teal" },
    { label: t("home.statUrgent"), value: stats?.urgent, tone: "red" },
    { label: t("home.statCreated"), value: stats?.created, tone: "amber" },
  ];

  if (orgs.length === 0) {
    return (
      <div className="home">
        <h1 className="home-greeting">{t("home.greeting", { name: firstName })}</h1>
        <div className="empty-state">
          <div className="empty-icon">◎</div>
          <h2>{t("home.noOrgsTitle")}</h2>
          <p>{t("home.noOrgsText")}</p>
          <button className="btn btn-primary" onClick={openCreateOrg}>
            {t("home.createOrg")}
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="home">
      <div>
        <h1 className="home-greeting">{t("home.greeting", { name: firstName })}</h1>
        <p className="muted">{t("home.subtitle")}</p>
      </div>
      {error && <div className="banner-error">{error}</div>}

      <div className="stat-grid">
        {tiles.map((tile) => (
          <div key={tile.label} className={`stat stat--${tile.tone}`}>
            <span className="stat-value">{tile.value ?? "–"}</span>
            <span className="stat-label">{tile.label}</span>
          </div>
        ))}
      </div>

      <div className="home-columns">
        <section className="panel">
          <h2>{t("home.myTasks")}</h2>
          {data && data.assigned.length === 0 && <p className="muted">{t("home.myTasksEmpty")}</p>}
          <div className="task-list">
            {data?.assigned.map((c) => (
              <Link key={c.id} to={`/projects/${c.projectId}`} className="task-row">
                <span className="priority" style={{ color: PRIORITY_ICON[c.priority].color }} title={t("priority.label", { name: t(`priority.${c.priority}`) })}>
                  {PRIORITY_ICON[c.priority].icon}
                </span>
                <span className="task-row-main">
                  <span className="task-row-title">{c.title}</span>
                  <small>
                    {c.key} · {c.projectName} · {c.columnName}
                  </small>
                </span>
              </Link>
            ))}
          </div>
        </section>

        <section className="panel">
          <h2>{t("home.organizations")}</h2>
          <div className="org-cards">
            {orgs.map((o) => (
              <Link key={o.id} to={`/orgs/${o.id}`} className="org-card">
                <span className="org-badge org-badge--lg">{o.name.slice(0, 1).toUpperCase()}</span>
                <span className="org-card-main">
                  <span className="org-card-name">{o.name}</span>
                  <small>
                    {t("home.members", { count: o.memberCount })} · {t("home.projectsCount", { count: o.projectCount })}
                  </small>
                </span>
                <span className={`role-chip role-chip--${o.role}`}>{t(`orgRoles.${o.role}`)}</span>
              </Link>
            ))}
          </div>
        </section>
      </div>

      <section>
        <div className="section-head">
          <h2>{t("home.projects")}</h2>
          <button className="btn btn-ghost btn-sm" onClick={() => openCreateProject()}>
            {t("org.newProject")}
          </button>
        </div>
        {projects.length === 0 && <p className="muted">{t("home.projectsEmpty")}</p>}
        <div className="project-grid">
          {projects.map((p) => (
            <Link key={p.id} to={`/projects/${p.id}`} className="project-card">
              <span className="project-badge">{p.key.slice(0, 2)}</span>
              <span className="project-card-name">
                {p.name} <VoiceRoomBadge projectId={p.id} />
              </span>
              <small className="muted">{p.organizationName}</small>
              <span className="project-card-stats">
                <span>{t("home.tasks", { count: p.cardCount })}</span>
                <span>{t("home.members", { count: p.memberCount })}</span>
              </span>
            </Link>
          ))}
        </div>
      </section>
    </div>
  );
}
