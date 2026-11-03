import { useCallback, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Avatar } from "@/components/Avatar/Avatar";
import { Button } from "@/components/ui/Button/Button";
import { Chip } from "@/components/ui/Chip/Chip";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog/ConfirmDialog";
import { Field } from "@/components/ui/Field/Field";
import { Menu } from "@/components/ui/Menu/Menu";
import { MenuItem } from "@/components/ui/MenuItem/MenuItem";
import { useAuth } from "@/context/auth";
import { useClickOutside } from "@/hooks/useClickOutside";
import { peopleApi } from "@/lib/api/people";
import { can } from "@/lib/companyRoles";
import { formatDate } from "@/lib/format";
import type { CompanyRole } from "@/types";
import type { Person } from "@/types/people";
import s from "./PersonHeader.module.scss";

type Dialog = "role" | "reset" | "dismiss" | "deactivate" | null;

/** Avatar, name, role and status, plus HR's actions on the person. */
export function PersonHeader({ person, onChanged }: { person: Person; onChanged: () => void }) {
  const { t, i18n } = useTranslation();
  const { user } = useAuth();
  const [menu, setMenu] = useState(false);
  const [dialog, setDialog] = useState<Dialog>(null);
  const [role, setRole] = useState<CompanyRole>(person.role);
  const [lead, setLead] = useState(person.additionRoles.includes("LEAD"));
  const [reason, setReason] = useState("");
  const [notice, setNotice] = useState("");
  const ref = useRef<HTMLDivElement>(null);
  useClickOutside(ref, menu, useCallback(() => setMenu(false), []));

  const self = person.access.self;
  const owner = person.role === "OWNER";
  const canRole = !self && !owner && (user?.assignableRoles.length ?? 0) > 0;
  const canReset = !owner && can(user, "resetPassword");
  const canDismiss = !owner && person.active && can(user, "fireUser") && (!self || can(user, "fireYourself"));
  const canToggle = !self && !owner && can(user, "editUser");
  const actions = canRole || canReset || canDismiss || canToggle;
  const pick = (d: Dialog) => (setMenu(false), setDialog(d));
  const roleOptions = Array.from(new Set([person.role, ...(user?.assignableRoles ?? [])]));

  return (
    <header className={s.header}>
      <Avatar user={person} size={72} />
      <div className={s.text}>
        <h1>{person.name}</h1>
        {person.nameCyrillic && <div className={s.sub}>{person.nameCyrillic}</div>}
        <div className={s.chips}>
          <Chip tone="accent">{t(`companyRoles.${person.role}`)}</Chip>
          {person.additionRoles.map((r) => (
            <Chip key={r} tone="accent">
              {t(`companyRoles.${r}`)}
            </Chip>
          ))}
          {!person.active && <Chip tone="danger">{person.dismissedAt ? t("people.dismissed") : t("people.inactive")}</Chip>}
          {person.trialUntil && <Chip tone="warning">{t("people.trial", { date: formatDate(person.trialUntil, i18n.language) })}</Chip>}
        </div>
        {person.dismissalReason && <div className={s.sub}>{t("people.dismissalReason")}: {person.dismissalReason}</div>}
        {notice && <div className={s.notice}>{notice}</div>}
      </div>
      {actions && (
        <div className={s.menuWrap} ref={ref}>
          <Button onClick={() => setMenu((v) => !v)}>{t("people.actions")} ▾</Button>
          {menu && (
            <Menu align="right" className={s.menu}>
              {canRole && <MenuItem onClick={() => pick("role")}>{t("people.changeRole")}</MenuItem>}
              {canReset && <MenuItem onClick={() => pick("reset")}>{t("people.resetPassword")}</MenuItem>}
              {canToggle && (
                <MenuItem onClick={() => (person.active ? pick("deactivate") : (setMenu(false), peopleApi.setActive(person.id, true).then(onChanged)))}>
                  {person.active ? t("people.deactivate") : t("people.activate")}
                </MenuItem>
              )}
              {canDismiss && (
                <MenuItem danger onClick={() => pick("dismiss")}>
                  {t("people.dismiss")}
                </MenuItem>
              )}
            </Menu>
          )}
        </div>
      )}

      {dialog === "role" && (
        <ConfirmDialog
          title={t("people.changeRole")}
          confirmLabel={t("common.save")}
          onClose={() => setDialog(null)}
          onConfirm={async () => {
            const additions = can(user, "changeAdditionRole") ? (lead ? ["LEAD"] : []) : null;
            await peopleApi.changeRole(person.id, role, additions);
            onChanged();
          }}
        >
          <Field label={t("people.colRole")}>
            <select value={role} onChange={(e) => setRole(e.target.value as CompanyRole)}>
              {roleOptions.map((r) => (
                <option key={r} value={r}>
                  {t(`companyRoles.${r}`)}
                </option>
              ))}
            </select>
          </Field>
          {can(user, "changeAdditionRole") && (
            <label className={s.check}>
              <input type="checkbox" checked={lead} onChange={(e) => setLead(e.target.checked)} />
              {t("people.leadRole")}
            </label>
          )}
        </ConfirmDialog>
      )}
      {dialog === "reset" && (
        <ConfirmDialog
          title={t("people.resetPassword")}
          text={t("people.resetPasswordText", { name: person.name })}
          confirmLabel={t("people.resetPassword")}
          onClose={() => setDialog(null)}
          onConfirm={async () => {
            await peopleApi.resetPassword(person.id);
            setNotice(t("people.passwordSent"));
          }}
        />
      )}
      {dialog === "deactivate" && (
        <ConfirmDialog
          title={t("people.deactivate")}
          text={t("people.deactivateText", { name: person.name })}
          confirmLabel={t("people.deactivate")}
          danger
          onClose={() => setDialog(null)}
          onConfirm={async () => {
            await peopleApi.setActive(person.id, false);
            onChanged();
          }}
        />
      )}
      {dialog === "dismiss" && (
        <ConfirmDialog
          title={t("people.dismissTitle", { name: person.name })}
          text={t("people.dismissText")}
          confirmLabel={t("people.dismiss")}
          danger
          onClose={() => setDialog(null)}
          onConfirm={async () => {
            await peopleApi.dismiss(person.id, reason);
            onChanged();
          }}
        >
          <Field label={t("people.dismissalReason")}>
            <textarea rows={3} value={reason} onChange={(e) => setReason(e.target.value)} maxLength={2000} />
          </Field>
        </ConfirmDialog>
      )}
    </header>
  );
}
