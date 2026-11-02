import type { ReactNode } from "react";
import { cx } from "@/lib/cx";
import s from "./Panel.module.scss";

interface Props {
  /** Less padding, for dashboard tiles. */
  compact?: boolean;
  className?: string;
  children: ReactNode;
}

/** A glass section in the page flow (settings cards, dashboard panels). */
export function Panel({ compact, className, children }: Props) {
  return <section className={cx(s.panel, compact && s.compact, className)}>{children}</section>;
}
