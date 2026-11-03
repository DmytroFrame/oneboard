import { cx } from "@/lib/cx";
import type { CompanyRole, Role } from "@/types";
import s from "./RoleChip.module.scss";

interface Props {
  role: CompanyRole | Role;
  /** Translated role name. */
  label: string;
  /** md: in member lists of a project. */
  size?: "sm" | "md";
}

export function RoleChip({ role, label, size = "sm" }: Props) {
  // Owners and admins stand out, in projects (owner/admin) and organizations (OWNER/ADMIN) alike.
  return <span className={cx(s.chip, s[role.toLowerCase()], size === "md" && s.md)}>{label}</span>;
}
