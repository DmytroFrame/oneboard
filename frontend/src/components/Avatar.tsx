import { useState } from "react";
import { useTranslation } from "react-i18next";
import type { User } from "../types";

export function initials(name: string) {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((p) => p[0]!.toUpperCase())
    .join("");
}

interface Props {
  user: User | null | undefined;
  size?: number;
  active?: boolean;
  /** Presence: true = here right now (bright, with a dot), false = away (dimmed), undefined = not shown. */
  online?: boolean;
  onClick?: () => void;
}

export function Avatar({ user, size = 28, active, online, onClick }: Props) {
  const { t } = useTranslation();
  const [broken, setBroken] = useState<string | null>(null);
  const style = {
    width: size,
    height: size,
    fontSize: Math.round(size * 0.4),
    background: user ? user.color : "#DFE1E6",
  };
  const presence = online === undefined ? "" : online ? " avatar--online" : " avatar--away";
  const className = `avatar${active ? " avatar--active" : ""}${onClick ? " avatar--clickable" : ""}${presence}`;
  const title = user ? `${user.name} (${user.email})` : t("common.unassigned");
  const content =
    user?.avatarUrl && broken !== user.avatarUrl ? (
      <img src={user.avatarUrl} alt="" loading="lazy" draggable={false} onError={() => setBroken(user.avatarUrl)} />
    ) : user ? (
      initials(user.name)
    ) : (
      "?"
    );

  if (onClick) {
    return (
      <button type="button" className={className} style={style} title={title} onClick={onClick}>
        {content}
      </button>
    );
  }
  return (
    <span className={className} style={style} title={title}>
      {content}
    </span>
  );
}
