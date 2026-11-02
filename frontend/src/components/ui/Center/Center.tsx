import type { ReactNode } from "react";
import { cx } from "@/lib/cx";
import s from "./Center.module.scss";

/** Centered, muted content: loading states. fullScreen fills the viewport. */
export function Center({ fullScreen, children }: { fullScreen?: boolean; children: ReactNode }) {
  return <div className={cx(s.center, fullScreen && s.fullScreen)}>{children}</div>;
}
