import type { ReactNode } from "react";
import { cx } from "@/lib/cx";
import s from "./Menu.module.scss";

interface Props {
  /** Which edge of the anchor (the positioned parent) the menu lines up with. */
  align?: "left" | "right";
  className?: string;
  children: ReactNode;
}

/** A dropdown under its anchor; the parent must be position: relative. */
export function Menu({ align = "left", className, children }: Props) {
  return <div className={cx(s.menu, s[align], className)}>{children}</div>;
}
