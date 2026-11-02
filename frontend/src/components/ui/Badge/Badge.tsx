import type { ReactNode } from "react";
import s from "./Badge.module.scss";

/** A small grey count pill. */
export function Badge({ title, children }: { title?: string; children: ReactNode }) {
  return (
    <span className={s.badge} title={title}>
      {children}
    </span>
  );
}
