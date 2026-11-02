import type { ReactNode } from "react";
import { cx } from "@/lib/cx";
import s from "./Muted.module.scss";

interface Props {
  as?: "span" | "p" | "div";
  className?: string;
  children: ReactNode;
}

/** Secondary text: hints, counts, empty states. */
export function Muted({ as: Tag = "span", className, children }: Props) {
  return <Tag className={cx(s.muted, className)}>{children}</Tag>;
}
