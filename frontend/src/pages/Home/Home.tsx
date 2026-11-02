import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { EmptyState } from "@/components/EmptyState/EmptyState";
import { OrgBadge } from "@/components/OrgBadge/OrgBadge";
import { Page } from "@/components/Page/Page";
import { PriorityIcon } from "@/components/PriorityIcon/PriorityIcon";
import { ProjectCard } from "@/components/ProjectCard/ProjectCard";
import { ProjectGrid } from "@/components/ProjectCard/ProjectGrid";
import { RoleChip } from "@/components/RoleChip/RoleChip";
import { Banner } from "@/components/ui/Banner/Banner";
import { Button } from "@/components/ui/Button/Button";
import { Muted } from "@/components/ui/Muted/Muted";
import { Panel } from "@/components/ui/Panel/Panel";
import { useAuth } from "@/context/auth";
import { useLayout } from "@/layouts/AppLayout/useLayout";
import { api } from "@/lib/api";
import { cx } from "@/lib/cx";
import type { Dashboard } from "@/types";
import s from "./Home.module.scss";

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
    { label: t("home.statOrganizations"), value: stats?.organizations, tone: s.violet },
    { label: t("home.statProjects"), value: stats?.projects, tone: s.blue },
    { label: t("home.statAssigned"), value: stats?.assigned, tone: s.teal },
    { label: t("home.statUrgent"), value: stats?.urgent, tone: s.red },
    { label: t("home.statCreated"), value: stats?.created, tone: s.amber },
  ];

  if (orgs.length === 0) {
    return (
      <Page>
        <h1 className={s.greeting}>{t("home.greeting", { name: firstName })}</h1>
        <EmptyState
          icon="◎"
          title={t("home.noOrgsTitle")}
          text={t("home.noOrgsText")}
          action={
            <Button variant="primary" onClick={openCreateOrg}>
              {t("home.createOrg")}
            </Button>
          }
        />
      </Page>
    );
  }

  return (
    <Page>
      <div>
        <h1 className={s.greeting}>{t("home.greeting", { name: firstName })}</h1>
        <Muted as="p" className={s.flush}>
          {t("home.subtitle")}
        </Muted>
      </div>
      {error && <Banner>{error}</Banner>}

      <div className={s.stats}>
        {tiles.map((tile) => (
          <div key={tile.label} className={cx(s.stat, tile.tone)}>
            <span className={s.statValue}>{tile.value ?? "–"}</span>
            <span className={s.statLabel}>{tile.label}</span>
          </div>
        ))}
      </div>

      <div className={s.columns}>
        <Panel compact>
          <h2>{t("home.myTasks")}</h2>
          {data && data.assigned.length === 0 && <Muted as="p" className={s.flush}>{t("home.myTasksEmpty")}</Muted>}
          <div className={s.list}>
            {data?.assigned.map((c) => (
              <Link key={c.id} to={`/projects/${c.projectId}`} className={s.row}>
                <PriorityIcon priority={c.priority} />
                <span className={s.rowMain}>
                  <span className={s.rowTitle}>{c.title}</span>
                  <small>
                    {c.key} · {c.projectName} · {c.columnName}
                  </small>
                </span>
              </Link>
            ))}
          </div>
        </Panel>

        <Panel compact>
          <h2>{t("home.organizations")}</h2>
          <div className={s.list}>
            {orgs.map((o) => (
              <Link key={o.id} to={`/orgs/${o.id}`} className={s.row}>
                <OrgBadge name={o.name} size="lg" />
                <span className={s.rowMain}>
                  <span className={s.rowTitle}>{o.name}</span>
                  <small>
                    {t("home.members", { count: o.memberCount })} · {t("home.projectsCount", { count: o.projectCount })}
                  </small>
                </span>
                <RoleChip role={o.role} label={t(`orgRoles.${o.role}`)} />
              </Link>
            ))}
          </div>
        </Panel>
      </div>

      <section>
        <div className={s.sectionHead}>
          <h2>{t("home.projects")}</h2>
          <Button size="sm" onClick={() => openCreateProject()}>
            {t("org.newProject")}
          </Button>
        </div>
        {projects.length === 0 && <Muted as="p" className={s.flush}>{t("home.projectsEmpty")}</Muted>}
        <ProjectGrid>
          {projects.map((p) => (
            <ProjectCard key={p.id} project={p} showOrganization />
          ))}
        </ProjectGrid>
      </section>
    </Page>
  );
}
