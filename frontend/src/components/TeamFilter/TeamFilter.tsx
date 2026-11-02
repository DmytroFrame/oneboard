import { useCallback, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Avatar } from "@/components/Avatar/Avatar";
import { Badge } from "@/components/ui/Badge/Badge";
import { Menu } from "@/components/ui/Menu/Menu";
import { useClickOutside } from "@/hooks/useClickOutside";
import type { Member } from "@/types";
import s from "./TeamFilter.module.scss";

const VISIBLE_AVATARS = 6;

interface Props {
  /** Current user first, then everyone else alphabetically. */
  members: Member[];
  meId: number;
  onlineIds: Set<number>;
  selected: Set<number>;
  showUnassigned: boolean;
  onToggleUser: (id: number) => void;
  onToggleUnassigned: () => void;
  /** Shown to project owners: opens the project's members settings. */
  onAddPeople?: () => void;
}

/** The board's people filter: avatars to toggle (with presence) and a dropdown with everyone. */
export function TeamFilter({ members, meId, onlineIds, selected, showUnassigned, onToggleUser, onToggleUnassigned, onAddPeople }: Props) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const close = useCallback(() => setOpen(false), []);
  useClickOutside(ref, open, close);
  const overflow = members.length - VISIBLE_AVATARS;

  return (
    <div className={s.group}>
      {members.slice(0, VISIBLE_AVATARS).map((u) => (
        <Avatar key={u.id} user={u} size={32} active={selected.has(u.id)} online={onlineIds.has(u.id)} onClick={() => onToggleUser(u.id)} />
      ))}
      <div className={s.dropdown} ref={ref}>
        <Avatar text={overflow > 0 ? `+${overflow}` : "☰"} size={32} active={open} title={t("board.allMembers")} onClick={() => setOpen((v) => !v)} />
        {open && (
          <Menu className={s.menu}>
            <div className={s.menuTitle}>{t("board.members", { count: members.length })}</div>
            {members.map((u) => (
              <label key={u.id} className={s.row}>
                <input type="checkbox" checked={selected.has(u.id)} onChange={() => onToggleUser(u.id)} />
                <Avatar user={u} size={28} online={onlineIds.has(u.id)} />
                <span className={s.rowName}>
                  <span>
                    {u.name}
                    {u.id === meId && <em> {t("common.you")}</em>}
                    {u.role === "owner" && <em> · {t("board.owner")}</em>}
                  </span>
                  <small>{u.email}</small>
                </span>
                <Badge title={t("board.assignedTasks")}>{u.assignedCount}</Badge>
              </label>
            ))}
            <label className={s.row}>
              <input type="checkbox" checked={showUnassigned} onChange={onToggleUnassigned} />
              <Avatar user={null} size={28} />
              <span className={s.rowName}>{t("common.unassigned")}</span>
            </label>
            {onAddPeople && (
              <button
                type="button"
                className={s.addPeople}
                onClick={() => {
                  setOpen(false);
                  onAddPeople();
                }}
              >
                {t("board.addPeople")}
              </button>
            )}
          </Menu>
        )}
      </div>
    </div>
  );
}
