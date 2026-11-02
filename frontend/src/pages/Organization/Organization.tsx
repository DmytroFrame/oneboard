import { useCallback, useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Navigate, useNavigate, useParams } from "react-router-dom";
import { Avatar } from "@/components/Avatar/Avatar";
import { DangerZone } from "@/components/DangerZone/DangerZone";
import { MemberList } from "@/components/MemberList/MemberList";
import { MemberRow } from "@/components/MemberList/MemberRow";
import { OrgBadge } from "@/components/OrgBadge/OrgBadge";
import { Page } from "@/components/Page/Page";
import { PageHeader } from "@/components/PageHeader/PageHeader";
import { ProjectCard } from "@/components/ProjectCard/ProjectCard";
import { ProjectGrid } from "@/components/ProjectCard/ProjectGrid";
import { RoleChip } from "@/components/RoleChip/RoleChip";
import { Banner } from "@/components/ui/Banner/Banner";
import { Button } from "@/components/ui/Button/Button";
import { InlineForm } from "@/components/ui/InlineForm/InlineForm";
import { Muted } from "@/components/ui/Muted/Muted";
import { Panel } from "@/components/ui/Panel/Panel";
import { useAuth } from "@/context/auth";
import { useLayout } from "@/layouts/AppLayout/useLayout";
import { api } from "@/lib/api";
import { can, ORG_ROLES } from "@/lib/permissions";
import type { Invitation, Organization, OrgMember, OrgRole } from "@/types";
import s from "./Organization.module.scss";

/** Route wrapper: remounts the page when switching organizations so all state resets. */
export default function OrganizationRoute() {
  const { orgId } = useParams();
  const { orgs } = useLayout();
  const org = orgs.find((o) => o.id === Number(orgId));
  if (!org) return <Navigate to="/home" replace />;
  return <OrganizationPage key={org.id} org={org} />;
}

function OrganizationPage({ org }: { org: Organization }) {
  const { t, i18n } = useTranslation();
  const { user } = useAuth();
  const { projects, reload, openCreateProject, orgMembersSignal } = useLayout();
  const navigate = useNavigate();
  const me = user!;
  const role = org.role;

  const [members, setMembers] = useState<OrgMember[]>([]);
  const [invitations, setInvitations] = useState<Invitation[]>([]);
  const [name, setName] = useState(org.name);
  const [email, setEmail] = useState("");
  const [inviteRole, setInviteRole] = useState<OrgRole>("member");
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    try {
      setMembers(await api.orgMembers(org.id));
      if (can.invite(role)) setInvitations(await api.invitations(org.id));
    } catch (err) {
      setError((err as Error).message);
    }
  }, [org.id, role]);

  useEffect(() => {
    load();
  }, [load]);

  // Someone else invited, removed or re-roled a member: refresh the lists.
  useEffect(() => {
    if (orgMembersSignal.orgId === org.id) load();
  }, [orgMembersSignal, org.id, load]);

  async function run(action: () => Promise<void>) {
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await action();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  const invite = (e: FormEvent) => {
    e.preventDefault();
    run(async () => {
      const res = await api.invite(org.id, email, inviteRole);
      setNotice(t(res.status === "added" ? "org.added" : "org.invited", { email }));
      setEmail("");
      await load();
      await reload(); // member counts in the sidebar
    });
  };

  const rename = (e: FormEvent) => {
    e.preventDefault();
    run(async () => {
      await api.renameOrganization(org.id, name);
      await reload();
    });
  };

  const changeRole = (m: OrgMember, next: OrgRole) =>
    run(async () => {
      setMembers(await api.changeOrgRole(org.id, m.id, next));
      if (m.id === me.id) await reload(); // my own permissions changed
    });

  const remove = (m: OrgMember) =>
    run(async () => {
      if (!confirm(t("org.confirmRemove", { name: m.name }))) return;
      await api.removeOrgMember(org.id, m.id);
      await load();
      await reload();
    });

  const leave = () =>
    run(async () => {
      if (!confirm(t("org.confirmLeave", { name: org.name }))) return;
      await api.removeOrgMember(org.id, me.id);
      await reload();
      navigate("/home", { replace: true });
    });

  const destroy = () =>
    run(async () => {
      if (!confirm(t("org.confirmDelete", { name: org.name }))) return;
      await api.deleteOrganization(org.id);
      await reload();
      navigate("/home", { replace: true });
    });

  const orgProjects = projects.filter((p) => p.organizationId === org.id);
  const dateFmt = new Intl.DateTimeFormat(i18n.language, { dateStyle: "medium" });

  return (
    <Page width="medium">
      <PageHeader
        icon={<OrgBadge name={org.name} size="xl" />}
        breadcrumbs={t("nav.organizations")}
        title={org.name}
        meta={
          <Muted>
            {t("home.members", { count: org.memberCount })} · {t("home.projectsCount", { count: org.projectCount })} ·{" "}
            <RoleChip role={role} label={t(`orgRoles.${role}`)} />
          </Muted>
        }
        actions={
          can.createProjects(role) && (
            <Button variant="primary" onClick={() => openCreateProject(org.id)}>
              {t("org.newProject")}
            </Button>
          )
        }
      />

      {error && <Banner>{error}</Banner>}
      {notice && <Banner tone="success">{notice}</Banner>}

      <Panel>
        <h2>{t("org.projects")}</h2>
        {orgProjects.length === 0 && <Muted as="p">{t("org.noProjects")}</Muted>}
        <ProjectGrid>
          {orgProjects.map((p) => (
            <ProjectCard key={p.id} project={p} />
          ))}
        </ProjectGrid>
      </Panel>

      <Panel>
        <h2>{t("org.members", { count: members.length })}</h2>
        <Muted as="p" className={s.rolesHint}>
          {t("org.rolesHint")}
        </Muted>
        <MemberList>
          {members.map((m) => (
            <MemberRow key={m.id} avatar={<Avatar user={m} size={34} />} name={m.name} note={m.id === me.id && t("common.you")} detail={m.email}>
              {can.manageRoles(role) ? (
                <select className={s.roleSelect} value={m.role} disabled={busy} onChange={(e) => changeRole(m, e.target.value as OrgRole)}>
                  {ORG_ROLES.map((r) => (
                    <option key={r} value={r}>
                      {t(`orgRoles.${r}`)}
                    </option>
                  ))}
                </select>
              ) : (
                <RoleChip role={m.role} label={t(`orgRoles.${m.role}`)} />
              )}
              {m.id !== me.id && can.remove(role, m.role) && (
                <Button variant="danger" size="sm" onClick={() => remove(m)} disabled={busy}>
                  {t("common.remove")}
                </Button>
              )}
            </MemberRow>
          ))}
        </MemberList>
      </Panel>

      {can.invite(role) && (
        <Panel>
          <h2>{t("org.invite")}</h2>
          <InlineForm className={s.inviteForm} onSubmit={invite}>
            <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder={t("org.emailPlaceholder")} required />
            <select value={inviteRole} onChange={(e) => setInviteRole(e.target.value as OrgRole)}>
              {ORG_ROLES.filter((r) => can.grant(role, r)).map((r) => (
                <option key={r} value={r}>
                  {t(`orgRoles.${r}`)}
                </option>
              ))}
            </select>
            <Button variant="primary" disabled={busy}>
              {t("org.sendInvite")}
            </Button>
          </InlineForm>
          <Muted as="p" className={s.hint}>
            {t("org.inviteHint")}
          </Muted>

          {invitations.length > 0 && (
            <>
              <h3 className={s.subhead}>{t("org.pending")}</h3>
              <MemberList>
                {invitations.map((inv) => (
                  <MemberRow key={inv.id} avatar={<Avatar text="@" size={34} />} name={inv.email} detail={dateFmt.format(new Date(inv.createdAt))}>
                    <RoleChip role={inv.role} label={t(`orgRoles.${inv.role}`)} />
                    <Button size="sm" disabled={busy} onClick={() => run(async () => (await api.revokeInvitation(org.id, inv.id), await load()))}>
                      {t("org.revoke")}
                    </Button>
                  </MemberRow>
                ))}
              </MemberList>
            </>
          )}
        </Panel>
      )}

      {can.rename(role) && (
        <Panel>
          <h2>{t("org.rename")}</h2>
          <InlineForm onSubmit={rename}>
            <input value={name} onChange={(e) => setName(e.target.value)} maxLength={100} required />
            <Button disabled={busy || name.trim() === org.name}>{t("common.save")}</Button>
          </InlineForm>
        </Panel>
      )}

      {can.delete(role) ? (
        <DangerZone
          title={t("org.deleteTitle")}
          text={t("org.deleteText")}
          action={
            <Button variant="dangerSolid" onClick={destroy} disabled={busy}>
              {t("org.deleteTitle")}
            </Button>
          }
        />
      ) : (
        <DangerZone
          title={t("org.leaveTitle")}
          text={t("org.leaveText")}
          action={
            <Button variant="dangerSolid" onClick={leave} disabled={busy}>
              {t("org.leave")}
            </Button>
          }
        />
      )}
    </Page>
  );
}
