import { useTranslation } from "react-i18next";
import { NavLink } from "react-router-dom";
import { OrgSwitcher } from "@/components/OrgSwitcher/OrgSwitcher";
import { ProjectBadge } from "@/components/ProjectBadge/ProjectBadge";
import { BookIcon, CalendarIcon, ChartIcon, ClockIcon, FolderIcon, HomeIcon, ProfileIcon, SettingsIcon, TeamIcon, UserPlusIcon } from "@/components/ui/icons";
import { Spacer } from "@/components/ui/Spacer/Spacer";
import { VoiceRoomBadge } from "@/components/VoiceRoomBadge/VoiceRoomBadge";
import { can, canAccess } from "@/lib/companyRoles";
import { cx } from "@/lib/cx";
import type { CurrentUser, Organization, Project } from "@/types";
import s from "./Sidebar.module.scss";

interface Props {
  user: CurrentUser;
  orgs: Organization[];
  projects: Project[];
  /** Phones: the sidebar is an off-canvas drawer. */
  open: boolean;
  onCreateProject: (orgId: number) => void;
  onCreateOrg: () => void;
  /** Red dots: time missing on previous working days, absence requests waiting for you. */
  alerts: { time: boolean; vacations: boolean };
}

const linkClass = (base: string, active: string) => ({ isActive }: { isActive: boolean }) => cx(base, isActive && active);

/** Navigation: the organization switcher, sections, the organization's projects, settings. */
export function Sidebar({ user, orgs, projects, open, onCreateProject, onCreateOrg, alerts }: Props) {
  const { t } = useTranslation();
  return (
    <aside className={cx(s.sidebar, open && s.open)}>
      <OrgSwitcher user={user} onCreateOrg={onCreateOrg} />
      {canAccess(user, "home") && (
        <NavLink to="/home" className={linkClass(s.navLink, s.navLinkActive)}>
          <HomeIcon />
          {t("nav.home")}
        </NavLink>
      )}

      {canAccess(user, "time") && (
        <NavLink to="/time" className={linkClass(s.navLink, s.navLinkActive)}>
          <ClockIcon />
          {t("time.title")}
          {alerts.time && <span className={s.dot} title={t("time.missing")} />}
        </NavLink>
      )}
      {canAccess(user, "vacations") && (
        <NavLink to="/vacations" className={linkClass(s.navLink, s.navLinkActive)}>
          <CalendarIcon />
          {t("vacations.title")}
          {alerts.vacations && <span className={s.dot} />}
        </NavLink>
      )}
      {canAccess(user, "reports") && (
        <NavLink to="/reports" className={linkClass(s.navLink, s.navLinkActive)}>
          <ChartIcon />
          {t("reports.title")}
        </NavLink>
      )}
      {canAccess(user, "projects") && (
        <NavLink to="/projects" end className={linkClass(s.navLink, s.navLinkActive)}>
          <FolderIcon />
          {t("projects.title")}
        </NavLink>
      )}
      {canAccess(user, "team") && (
        <NavLink to="/team" className={linkClass(s.navLink, s.navLinkActive)}>
          <TeamIcon />
          {t("people.team")}
        </NavLink>
      )}
      {canAccess(user, "wiki") && (
        <NavLink to="/wiki" className={linkClass(s.navLink, s.navLinkActive)}>
          <BookIcon />
          {t("wiki.title")}
        </NavLink>
      )}
      {canAccess(user, "candidates") && (
        <NavLink to="/candidates" className={linkClass(s.navLink, s.navLinkActive)}>
          <UserPlusIcon />
          {t("candidates.title")}
        </NavLink>
      )}
      {canAccess(user, "hrStatistics") && (
        <NavLink to="/statistics" className={linkClass(s.navLink, s.navLinkActive)}>
          <ChartIcon />
          {t("hrStats.title")}
        </NavLink>
      )}
      <NavLink to="/profile" className={linkClass(s.navLink, s.navLinkActive)}>
        <ProfileIcon />
        {t("people.myProfile")}
      </NavLink>

      {canAccess(user, "projects") && <ProjectNav user={user} orgs={orgs} projects={projects} onCreateProject={onCreateProject} />}

      <Spacer />
      <NavLink to="/settings" className={linkClass(s.navLink, s.navLinkActive)}>
        <SettingsIcon />
        {t("layout.settings")}
      </NavLink>
    </aside>
  );
}

/** The current organization's projects (other organizations are reached through the switcher). */
function ProjectNav({ user, orgs, projects, onCreateProject }: Pick<Props, "user" | "orgs" | "projects" | "onCreateProject">) {
  const { t } = useTranslation();
  const org = orgs.find((o) => o.id === user.organization?.id);
  if (!org) return null;
  return (
    <>
      <NavLink to={`/orgs/${org.id}`} className={linkClass(s.title, s.titleActive)}>
        {t("nav.projectsOf")}
      </NavLink>
      <nav className={s.projectList}>
        {projects.map((p) => (
          <NavLink key={p.id} to={`/projects/${p.id}`} className={linkClass(s.projectLink, s.projectLinkActive)}>
            <ProjectBadge projectKey={p.key} />
            <span className={s.projectText}>
              <span className={s.projectName}>{p.name}</span>
              <small>{t("layout.projectMeta", { key: p.key, count: p.memberCount })}</small>
            </span>
            <VoiceRoomBadge projectId={p.id} />
          </NavLink>
        ))}
        {can(user, "addProject") && (
          <button type="button" className={s.addProject} onClick={() => onCreateProject(org.id)}>
            {t("nav.newProject")}
          </button>
        )}
      </nav>
    </>
  );
}
