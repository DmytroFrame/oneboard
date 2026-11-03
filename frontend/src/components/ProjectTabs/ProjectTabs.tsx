import { useTranslation } from "react-i18next";
import { NavLink } from "react-router-dom";
import { cx } from "@/lib/cx";
import type { Project } from "@/types";
import s from "./ProjectTabs.module.scss";

/** A project's pages; sections can be switched off in the project's settings. */
export function ProjectTabs({ project }: { project: Project }) {
  const { t } = useTranslation();
  const base = `/projects/${project.id}`;
  const tabs = [
    { to: base, label: t("tasks.tabBoard"), on: true },
    { to: `${base}/backlog`, label: t("tasks.tabBacklog"), on: project.sections.backlog },
    { to: `${base}/planning`, label: t("tasks.tabPlanning"), on: project.sections.planning },
    { to: `${base}/statistics`, label: t("statistics.tab"), on: project.sections.statistics },
    { to: `${base}/comments`, label: t("comments.tab"), on: project.sections.comments },
    { to: `${base}/wiki`, label: t("wiki.tab"), on: project.sections.wiki },
  ];
  return (
    <nav className={s.tabs} aria-label={project.name}>
      {tabs
        .filter((tab) => tab.on)
        .map((tab) => (
          <NavLink key={tab.to} to={tab.to} end className={({ isActive }) => cx(s.tab, isActive && s.active)}>
            {tab.label}
          </NavLink>
        ))}
    </nav>
  );
}
