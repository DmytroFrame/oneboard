import { cx } from "@/lib/cx";
import s from "./ProjectBadge.module.scss";

/** The first two letters of a project key on an accent square. */
export function ProjectBadge({ projectKey, className }: { projectKey: string; className?: string }) {
  return <span className={cx(s.badge, className)}>{projectKey.slice(0, 2)}</span>;
}
