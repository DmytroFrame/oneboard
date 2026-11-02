import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { ProjectBadge } from "@/components/ProjectBadge/ProjectBadge";
import { Muted } from "@/components/ui/Muted/Muted";
import { VoiceRoomBadge } from "@/components/VoiceRoomBadge/VoiceRoomBadge";
import type { Project } from "@/types";
import s from "./ProjectCard.module.scss";

interface Props {
  project: Project;
  /** Show the organization's name (Home lists projects of every organization). */
  showOrganization?: boolean;
}

export function ProjectCard({ project: p, showOrganization }: Props) {
  const { t } = useTranslation();
  return (
    <Link to={`/projects/${p.id}`} className={s.card}>
      <ProjectBadge projectKey={p.key} className={s.badge} />
      <span className={s.name}>
        {p.name} <VoiceRoomBadge projectId={p.id} />
      </span>
      {showOrganization && <Muted as="span">{p.organizationName}</Muted>}
      <span className={s.stats}>
        <span>{t("home.tasks", { count: p.cardCount })}</span>
        <span>{t("home.members", { count: p.memberCount })}</span>
      </span>
    </Link>
  );
}

