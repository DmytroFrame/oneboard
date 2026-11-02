import type { ReactNode } from "react";
import { cx } from "@/lib/cx";
import s from "./FieldLabel.module.scss";

/** A section label that isn't tied to a single control ("Description", "Files"). */
export function FieldLabel({ className, children }: { className?: string; children: ReactNode }) {
  return <span className={cx(s.label, className)}>{children}</span>;
}
