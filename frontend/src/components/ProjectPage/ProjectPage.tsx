import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Navigate, useParams } from "react-router-dom";
import { PageHeader } from "@/components/PageHeader/PageHeader";
import { ProjectTabs } from "@/components/ProjectTabs/ProjectTabs";
import { useLayout } from "@/layouts/AppLayout/useLayout";
import type { Project } from "@/types";
import s from "./ProjectPage.module.scss";

/** A project's tab other than the board: header, tabs, and the tab's content (given the project). */
export function ProjectPage({ children }: { children: (project: Project) => ReactNode }) {
  const { t } = useTranslation();
  const { projectId } = useParams();
  const { projects } = useLayout();
  const project = projects.find((p) => p.id === Number(projectId));
  if (!project) return <Navigate to="/projects" replace />;
  return (
    <div className={s.page}>
      <PageHeader breadcrumbs={t("board.breadcrumbs", { name: project.name })} title={t("board.title", { key: project.key })} />
      <ProjectTabs project={project} />
      <div className={s.content}>{children(project)}</div>
    </div>
  );
}
