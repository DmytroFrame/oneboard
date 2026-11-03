import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { OrgBadge } from "@/components/OrgBadge/OrgBadge";
import { ChevronDownIcon } from "@/components/ui/icons";
import { Menu } from "@/components/ui/Menu/Menu";
import { MenuItem } from "@/components/ui/MenuItem/MenuItem";
import { switchOrganization } from "@/lib/api";
import type { CurrentUser } from "@/types";
import s from "./OrgSwitcher.module.scss";

interface Props {
  user: CurrentUser;
  onCreateOrg: () => void;
}

/** The organization the app works in; each is an isolated workspace, so switching starts over. */
export function OrgSwitcher({ user, onCreateOrg }: Props) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const current = user.organization;

  useEffect(() => {
    if (!open) return;
    const close = (e: MouseEvent) => !ref.current?.contains(e.target as Node) && setOpen(false);
    document.addEventListener("mousedown", close);
    return () => document.removeEventListener("mousedown", close);
  }, [open]);

  return (
    <div className={s.switcher} ref={ref}>
      <button type="button" className={s.current} aria-haspopup="menu" aria-expanded={open} onClick={() => setOpen(!open)}>
        {current ? <OrgBadge name={current.name} /> : <span className={s.none}>?</span>}
        <span className={s.text}>
          <span className={s.name}>{current?.name ?? t("orgSwitcher.none")}</span>
          {current && <small>{t(`companyRoles.${current.role}`)}</small>}
        </span>
        <ChevronDownIcon />
      </button>
      {open && (
        <Menu className={s.menu}>
          <div className={s.heading}>{t("orgSwitcher.title")}</div>
          {user.organizations.map((o) => (
            <MenuItem
              key={o.id}
              className={o.id === current?.id ? s.selected : undefined}
              disabled={!o.active}
              title={o.active ? undefined : t("orgSwitcher.inactive")}
              onClick={() => (o.id === current?.id ? setOpen(false) : switchOrganization(o.id))}
            >
              <OrgBadge name={o.name} />
              <span className={s.itemName}>{o.name}</span>
              <small>{o.active ? t(`companyRoles.${o.role}`) : t("orgSwitcher.inactive")}</small>
            </MenuItem>
          ))}
          <MenuItem onClick={() => (setOpen(false), onCreateOrg())}>{t("nav.newOrganization")}</MenuItem>
        </Menu>
      )}
    </div>
  );
}
