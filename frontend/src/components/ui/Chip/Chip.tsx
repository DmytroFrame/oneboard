import type { CSSProperties, ReactNode } from "react";
import { cx } from "@/lib/cx";
import s from "./Chip.module.scss";

interface Props {
  /** A colour of its own (tags); otherwise neutral. */
  color?: string;
  tone?: "neutral" | "accent" | "success" | "warning" | "danger";
  children: ReactNode;
  title?: string;
}

/** A small pill: tags, statuses, roles. */
export function Chip({ color, tone = "neutral", children, title }: Props) {
  const style = color ? ({ "--chip": color } as CSSProperties) : undefined;
  return (
    <span className={cx(s.chip, color ? s.colored : s[tone])} style={style} title={title}>
      {children}
    </span>
  );
}
