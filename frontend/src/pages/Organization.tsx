import { useCallback, useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Link, Navigate, useNavigate, useParams } from "react-router-dom";
import { api } from "../api";
import { useAuth } from "../auth";
import { Avatar } from "../components/Avatar";
import { can, ORG_ROLES } from "../orgs";
import type { Invitation, Organization, OrgMember, OrgRole } from "../types";
import { useLayout } from "./AppLayout";

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
    <div className="org-page">
      <div className="board-head">
        <div className="org-head">
          <span className="org-badge org-badge--xl">{org.name.slice(0, 1).toUpperCase()}</span>
          <div>
            <div className="breadcrumbs">{t("nav.organizations")}</div>
            <h1>{org.name}</h1>
            <span className="muted">
              {t("home.members", { count: org.memberCount })} · {t("home.projectsCount", { count: org.projectCount })} ·{" "}
              <span className={`role-chip role-chip--${role}`}>{t(`orgRoles.${role}`)}</span>
            </span>
          </div>
        </div>
        {can.createProjects(role) && (
          <button className="btn btn-primary" onClick={() => openCreateProject(org.id)}>
            {t("org.newProject")}
          </button>
        )}
      </div>

      {error && <div className="banner-error">{error}</div>}
      {notice && <div className="banner-ok">{notice}</div>}

      <section className="settings-card">
        <h2>{t("org.projects")}</h2>
        {orgProjects.length === 0 && <p className="muted">{t("org.noProjects")}</p>}
        <div className="project-grid">
          {orgProjects.map((p) => (
            <Link key={p.id} to={`/projects/${p.id}`} className="project-card">
              <span className="project-badge">{p.key.slice(0, 2)}</span>
              <span className="project-card-name">{p.name}</span>
              <span className="project-card-stats">
                <span>{t("home.tasks", { count: p.cardCount })}</span>
                <span>{t("home.members", { count: p.memberCount })}</span>
              </span>
            </Link>
          ))}
        </div>
      </section>

      <section className="settings-card">
        <h2>{t("org.members", { count: members.length })}</h2>
        <p className="muted settings-hint roles-hint">{t("org.rolesHint")}</p>
        <div className="member-list">
          {members.map((m) => (
            <div className="member-row" key={m.id}>
              <Avatar user={m} size={34} />
              <div className="member-info">
                <span>
                  {m.name}
                  {m.id === me.id && <em> {t("common.you")}</em>}
                </span>
                <small>{m.email}</small>
              </div>
              {can.manageRoles(role) ? (
                <select className="role-select" value={m.role} disabled={busy} onChange={(e) => changeRole(m, e.target.value as OrgRole)}>
                  {ORG_ROLES.map((r) => (
                    <option key={r} value={r}>
                      {t(`orgRoles.${r}`)}
                    </option>
                  ))}
                </select>
              ) : (
                <span className={`role-chip role-chip--${m.role}`}>{t(`orgRoles.${m.role}`)}</span>
              )}
              {m.id !== me.id && can.remove(role, m.role) && (
                <button className="btn btn-danger btn-sm" onClick={() => remove(m)} disabled={busy}>
                  {t("common.remove")}
                </button>
              )}
            </div>
          ))}
        </div>
      </section>

      {can.invite(role) && (
        <section className="settings-card">
          <h2>{t("org.invite")}</h2>
          <form className="inline-form invite-form" onSubmit={invite}>
            <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder={t("org.emailPlaceholder")} required />
            <select value={inviteRole} onChange={(e) => setInviteRole(e.target.value as OrgRole)}>
              {ORG_ROLES.filter((r) => can.grant(role, r)).map((r) => (
                <option key={r} value={r}>
                  {t(`orgRoles.${r}`)}
                </option>
              ))}
            </select>
            <button className="btn btn-primary" disabled={busy}>
              {t("org.sendInvite")}
            </button>
          </form>
          <p className="muted settings-hint">{t("org.inviteHint")}</p>

          {invitations.length > 0 && (
            <>
              <h3 className="settings-subhead">{t("org.pending")}</h3>
              <div className="member-list">
                {invitations.map((inv) => (
                  <div className="member-row" key={inv.id}>
                    <span className="avatar avatar--more" style={{ width: 34, height: 34 }}>
                      @
                    </span>
                    <div className="member-info">
                      <span>{inv.email}</span>
                      <small>{dateFmt.format(new Date(inv.createdAt))}</small>
                    </div>
                    <span className={`role-chip role-chip--${inv.role}`}>{t(`orgRoles.${inv.role}`)}</span>
                    <button
                      className="btn btn-ghost btn-sm"
                      disabled={busy}
                      onClick={() => run(async () => (await api.revokeInvitation(org.id, inv.id), await load()))}
                    >
                      {t("org.revoke")}
                    </button>
                  </div>
                ))}
              </div>
            </>
          )}
        </section>
      )}

      {can.rename(role) && (
        <section className="settings-card">
          <h2>{t("org.rename")}</h2>
          <form className="inline-form" onSubmit={rename}>
            <input value={name} onChange={(e) => setName(e.target.value)} maxLength={100} required />
            <button className="btn btn-ghost" disabled={busy || name.trim() === org.name}>
              {t("common.save")}
            </button>
          </form>
        </section>
      )}

      <section className="settings-card danger-zone">
        {can.delete(role) ? (
          <>
            <div>
              <strong>{t("org.deleteTitle")}</strong>
              <p className="muted">{t("org.deleteText")}</p>
            </div>
            <button className="btn btn-danger-solid" onClick={destroy} disabled={busy}>
              {t("org.deleteTitle")}
            </button>
          </>
        ) : (
          <>
            <div>
              <strong>{t("org.leaveTitle")}</strong>
              <p className="muted">{t("org.leaveText")}</p>
            </div>
            <button className="btn btn-danger-solid" onClick={leave} disabled={busy}>
              {t("org.leave")}
            </button>
          </>
        )}
      </section>
    </div>
  );
}
