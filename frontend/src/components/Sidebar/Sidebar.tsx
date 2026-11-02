import { useTranslation } from "react-i18next";
import { NavLink } from "react-router-dom";
import { OrgBadge } from "@/components/OrgBadge/OrgBadge";
import { ProjectBadge } from "@/components/ProjectBadge/ProjectBadge";
import { RoleChip } from "@/components/RoleChip/RoleChip";
import { Button } from "@/components/ui/Button/Button";
import { HomeIcon, SettingsIcon } from "@/components/ui/icons";
import { Spacer } from "@/components/ui/Spacer/Spacer";
import { VoiceRoomBadge } from "@/components/VoiceRoomBadge/VoiceRoomBadge";
import { cx } from "@/lib/cx";
import { can } from "@/lib/permissions";
import type { Organization, Project } from "@/types";
import s from "./Sidebar.module.scss";

interface Props {
  orgs: Organization[];
  projects: Project[];
  /** Phones: the sidebar is an off-canvas drawer. */
  open: boolean;
  onCreateProject: (orgId: number) => void;
  onCreateOrg: () => void;
}

const linkClass = (base: string, active: string) => ({ isActive }: { isActive: boolean }) => cx(base, isActive && active);

/** Navigation: home, organizations with their projects, settings. */
export function Sidebar({ orgs, projects, open, onCreateProject, onCreateOrg }: Props) {
  const { t } = useTranslation();
  return (
    <aside className={cx(s.sidebar, open && s.open)}>
      <NavLink to="/home" className={linkClass(s.navLink, s.navLinkActive)}>
        <HomeIcon />
        {t("nav.home")}
      </NavLink>

      <div className={s.title}>{t("nav.organizations")}</div>
      <nav className={s.orgList}>
        {orgs.map((o) => (
          <div key={o.id}>
            <NavLink to={`/orgs/${o.id}`} className={linkClass(s.orgLink, s.orgLinkActive)}>
              <OrgBadge name={o.name} />
              <span className={s.orgName}>{o.name}</span>
              <RoleChip role={o.role} label={t(`orgRoles.${o.role}`)} />
            </NavLink>
            <div className={s.projectList}>
              {projects
                .filter((p) => p.organizationId === o.id)
                .map((p) => (
                  <NavLink key={p.id} to={`/projects/${p.id}`} className={linkClass(s.projectLink, s.projectLinkActive)}>
                    <ProjectBadge projectKey={p.key} />
                    <span className={s.projectText}>
                      <span className={s.projectName}>{p.name}</span>
                      <small>{t("layout.projectMeta", { key: p.key, count: p.memberCount })}</small>
                    </span>
                    <VoiceRoomBadge projectId={p.id} />
                  </NavLink>
                ))}
              {can.createProjects(o.role) && (
                <button type="button" className={s.addProject} onClick={() => onCreateProject(o.id)}>
                  {t("nav.newProject")}
                </button>
              )}
            </div>
          </div>
        ))}
      </nav>
      <Button size="sm" className={s.addOrg} onClick={onCreateOrg}>
        {t("nav.newOrganization")}
      </Button>

      <Spacer />
      <NavLink to="/settings" className={linkClass(s.navLink, s.navLinkActive)}>
        <SettingsIcon />
        {t("layout.settings")}
      </NavLink>
    </aside>
  );
}
