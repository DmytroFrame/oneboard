import { cx } from "@/lib/cx";
import type { OrgRole, Role } from "@/types";
import s from "./RoleChip.module.scss";

interface Props {
  role: OrgRole | Role;
  /** Translated role name. */
  label: string;
  /** md: in member lists of a project. */
  size?: "sm" | "md";
}

export function RoleChip({ role, label, size = "sm" }: Props) {
  return <span className={cx(s.chip, s[role], size === "md" && s.md)}>{label}</span>;
}
