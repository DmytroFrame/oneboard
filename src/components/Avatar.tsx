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
  onClick?: () => void;
}

export function Avatar({ user, size = 28, active, onClick }: Props) {
  const style = {
    width: size,
    height: size,
    fontSize: Math.round(size * 0.4),
    background: user ? user.color : "#DFE1E6",
  };
  const className = `avatar${active ? " avatar--active" : ""}${onClick ? " avatar--clickable" : ""}`;
  const title = user ? `${user.name} (${user.email})` : "Не призначено";

  if (onClick) {
    return (
      <button type="button" className={className} style={style} title={title} onClick={onClick}>
        {user ? initials(user.name) : "?"}
      </button>
    );
  }
  return (
    <span className={className} style={style} title={title}>
      {user ? initials(user.name) : "?"}
    </span>
  );
}
