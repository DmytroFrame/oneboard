import { useState } from "react";
import { useTranslation } from "react-i18next";
import { cx } from "@/lib/cx";
import { initials } from "@/lib/format";
import type { User } from "@/types";
import s from "./Avatar.module.scss";

interface Props {
  user?: User | null;
  /** Instead of a user: a glass placeholder with this text ("+3", "@"). */
  text?: string;
  size?: number;
  active?: boolean;
  /** Presence: true = here right now (bright, with a ring), false = away (dimmed), undefined = not shown. */
  online?: boolean;
  title?: string;
  onClick?: () => void;
  className?: string;
}

export function Avatar({ user, text, size = 28, active, online, title, onClick, className }: Props) {
  const { t } = useTranslation();
  const [broken, setBroken] = useState<string | null>(null);
  const placeholder = text !== undefined;
  const style = {
    width: size,
    height: size,
    fontSize: placeholder ? undefined : Math.round(size * 0.4),
    background: placeholder ? undefined : user ? user.color : "#DFE1E6",
  };
  const cls = cx(
    s.avatar,
    placeholder && s.placeholder,
    active && s.active,
    onClick && s.clickable,
    online === true && s.online,
    online === false && s.away,
    className,
  );
  const label = title ?? (placeholder ? undefined : user ? `${user.name} (${user.email})` : t("common.unassigned"));
  const content = placeholder ? (
    text
  ) : user?.avatarUrl && broken !== user.avatarUrl ? (
    <img src={user.avatarUrl} alt="" loading="lazy" draggable={false} onError={() => setBroken(user.avatarUrl)} />
  ) : user ? (
    initials(user.name)
  ) : (
    "?"
  );

  if (onClick) {
    return (
      <button type="button" className={cls} style={style} title={label} onClick={onClick}>
        {content}
      </button>
    );
  }
  return (
    <span className={cls} style={style} title={label}>
      {content}
    </span>
  );
}
