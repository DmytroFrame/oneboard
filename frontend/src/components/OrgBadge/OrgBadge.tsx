import { cx } from "@/lib/cx";
import s from "./OrgBadge.module.scss";

/** An organization's first letter on a violet square. */
export function OrgBadge({ name, size = "sm" }: { name: string; size?: "sm" | "lg" | "xl" }) {
  return <span className={cx(s.badge, size !== "sm" && s[size])}>{name.slice(0, 1).toUpperCase()}</span>;
}
