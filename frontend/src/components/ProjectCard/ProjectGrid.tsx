import type { ReactNode } from "react";
import s from "./ProjectCard.module.scss";

/** Project cards in a responsive grid. */
export function ProjectGrid({ children }: { children: ReactNode }) {
  return <div className={s.grid}>{children}</div>;
}
