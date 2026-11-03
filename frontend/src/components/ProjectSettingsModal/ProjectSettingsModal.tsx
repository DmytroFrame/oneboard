import { useEffect, useMemo, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Avatar } from "@/components/Avatar/Avatar";
import { DangerZone } from "@/components/DangerZone/DangerZone";
import { MemberList } from "@/components/MemberList/MemberList";
import { MemberRow } from "@/components/MemberList/MemberRow";
import { RoleChip } from "@/components/RoleChip/RoleChip";
import { Button } from "@/components/ui/Button/Button";
import { FormError } from "@/components/ui/FormError/FormError";
import { InlineForm } from "@/components/ui/InlineForm/InlineForm";
import { Modal, ModalContent } from "@/components/ui/Modal/Modal";
import { Muted } from "@/components/ui/Muted/Muted";
import { ProjectDetailsForm } from "@/components/ProjectDetailsForm/ProjectDetailsForm";
import { TagManager } from "@/components/TagManager/TagManager";
import { useAuth } from "@/context/auth";
import { api } from "@/lib/api";
import { can } from "@/lib/companyRoles";
import type { Member, OrgMember, Project } from "@/types";
import s from "./ProjectSettingsModal.module.scss";

interface Props {
  project: Project;
  members: Member[];
  currentUserId: number;
  onClose: () => void;
  onMembersChanged: (members: Member[]) => void;
  onProjectChanged: () => Promise<void>;
  onLeftOrDeleted: () => Promise<void>;
}

/** A project's name, members (added from its organization) and leaving / deleting it. */
export function ProjectSettingsModal({ project, members, currentUserId, onClose, onMembersChanged, onProjectChanged, onLeftOrDeleted }: Props) {
  const { t } = useTranslation();
  const { user } = useAuth();
  const isOwner = project.role === "owner";
  // Company managers run projects they belong to (OmiTeam's editProject); admins of members too.
  const canManage = isOwner || can(user, "editProject");
  const canSetRoles = isOwner || can(user, "addProjectMembers");
  const [orgMembers, setOrgMembers] = useState<OrgMember[] | null>(null);
  const [query, setQuery] = useState("");
  const [name, setName] = useState(project.name);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    // Only people from the project's organization can join it.
    if (isOwner) api.orgMembers(project.organizationId).then(setOrgMembers).catch((err) => setError(err.message));
  }, [isOwner, project.organizationId]);

  const candidates = useMemo(() => {
    const memberIds = new Set(members.map((m) => m.id));
    const q = query.trim().toLowerCase();
    return (orgMembers ?? [])
      .filter((u) => !memberIds.has(u.id))
      .filter((u) => !q || u.name.toLowerCase().includes(q) || u.email.toLowerCase().includes(q));
  }, [orgMembers, members, query]);

  async function run(action: () => Promise<void>) {
    setBusy(true);
    setError("");
    try {
      await action();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  const add = (u: OrgMember) => run(async () => onMembersChanged(await api.addMember(project.id, u.id)));

  const remove = (m: Member) =>
    run(async () => {
      if (!confirm(t("project.confirmRemoveMember", { name: m.name }))) return;
      await api.removeMember(project.id, m.id);
      onMembersChanged(members.filter((x) => x.id !== m.id));
    });

  const rename = (e: FormEvent) => {
    e.preventDefault();
    run(async () => {
      await api.renameProject(project.id, name);
      await onProjectChanged();
    });
  };

  const leave = () =>
    run(async () => {
      if (!confirm(t("project.confirmLeave", { name: project.name }))) return;
      await api.removeMember(project.id, currentUserId);
      await onLeftOrDeleted();
    });

  const destroy = () =>
    run(async () => {
      if (!confirm(t("project.confirmDelete", { name: project.name }))) return;
      await api.deleteProject(project.id);
      await onLeftOrDeleted();
    });

  return (
    <Modal title={t("project.settingsTitle", { key: project.key })} onClose={onClose} wide>
      <ModalContent className={s.settings}>
        {error && <FormError>{error}</FormError>}

        {isOwner && (
          <section>
            <h3>{t("project.projectName")}</h3>
            <InlineForm onSubmit={rename}>
              <input value={name} onChange={(e) => setName(e.target.value)} maxLength={100} required />
              <Button disabled={busy || name.trim() === project.name}>{t("common.save")}</Button>
            </InlineForm>
          </section>
        )}

        {canManage && (
          <section>
            <h3>{t("projects.details")}</h3>
            <ProjectDetailsForm project={project} onSaved={onProjectChanged} />
          </section>
        )}

        {(canManage || project.role === "admin") && (
          <section>
            <h3>{t("tasks.manageTags")}</h3>
            <TagManager projectId={project.id} />
          </section>
        )}

        <section>
          <h3>{t("project.members", { count: members.length })}</h3>
          <MemberList>
            {members.map((m) => (
              <MemberRow key={m.id} avatar={<Avatar user={m} size={32} />} name={m.name} note={m.id === currentUserId && t("common.you")} detail={m.email}>
                {canSetRoles && m.role !== "owner" ? (
                  <select
                    className={s.role}
                    value={m.role}
                    disabled={busy}
                    onChange={(e) => run(async () => onMembersChanged(await api.setMemberRole(project.id, m.id, e.target.value as "admin" | "member")))}
                  >
                    <option value="member">{t("projects.roleMember")}</option>
                    <option value="admin">{t("projects.roleAdmin")}</option>
                  </select>
                ) : (
                  <RoleChip role={m.role || "member"} label={t(`roles.${m.role || "member"}` as "roles.member")} size="md" />
                )}
                {isOwner && m.role !== "owner" && (
                  <Button variant="danger" size="sm" onClick={() => remove(m)} disabled={busy}>
                    {t("common.remove")}
                  </Button>
                )}
              </MemberRow>
            ))}
          </MemberList>
        </section>

        {isOwner && (
          <section>
            <h3>{t("project.addPeople")}</h3>
            <input className={s.search} placeholder={t("project.searchPeople")} value={query} onChange={(e) => setQuery(e.target.value)} />
            <MemberList scroll empty={candidates.length === 0 && (orgMembers === null ? t("common.loading") : t("project.noCandidates"))}>
              {candidates.map((u) => (
                <MemberRow key={u.id} avatar={<Avatar user={u} size={32} />} name={u.name} detail={u.email}>
                  <Button variant="primary" size="sm" onClick={() => add(u)} disabled={busy}>
                    {t("common.add")}
                  </Button>
                </MemberRow>
              ))}
            </MemberList>
            <Muted as="p">{t("projectExtra.mustBeOrgMember", { org: project.organizationName })}</Muted>
          </section>
        )}

        {isOwner ? (
          <DangerZone
            title={t("project.deleteTitle")}
            text={t("project.deleteText")}
            action={
              <Button variant="dangerSolid" onClick={destroy} disabled={busy}>
                {t("project.deleteTitle")}
              </Button>
            }
          />
        ) : (
          <DangerZone
            title={t("project.leaveTitle")}
            text={t("project.leaveText")}
            action={
              <Button variant="dangerSolid" onClick={leave} disabled={busy}>
                {t("project.leave")}
              </Button>
            }
          />
        )}
      </ModalContent>
    </Modal>
  );
}
