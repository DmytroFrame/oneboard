import type { ReactNode } from "react";
import { cx } from "@/lib/cx";
import s from "./Page.module.scss";

interface Props {
  /** Max content width: wide = dashboard, medium = organization, narrow = settings. */
  width?: "wide" | "medium" | "narrow";
  className?: string;
  children: ReactNode;
}

/** A scrolling page inside the app shell: centered column of sections. */
export function Page({ width = "wide", className, children }: Props) {
  return <div className={cx(s.page, s[width], className)}>{children}</div>;
}
