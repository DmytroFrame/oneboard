import type { ReactNode } from "react";
import { cx } from "@/lib/cx";
import s from "./Field.module.scss";

interface Props {
  label: ReactNode;
  hint?: ReactNode;
  /** "div" when the content isn't a form control (a label would capture its clicks). */
  as?: "label" | "div";
  className?: string;
  children: ReactNode;
}

/** A form control with its label above and an optional hint below. */
export function Field({ label, hint, as: Tag = "label", className, children }: Props) {
  return (
    <Tag className={cx(s.field, className)}>
      {label}
      {children}
      {hint && <small className={s.hint}>{hint}</small>}
    </Tag>
  );
}
