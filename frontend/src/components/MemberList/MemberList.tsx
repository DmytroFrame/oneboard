import type { ReactNode } from "react";
import { cx } from "@/lib/cx";
import s from "./MemberList.module.scss";

interface Props {
  /** Limit the height and scroll (long candidate lists). */
  scroll?: boolean;
  /** Shown instead of rows when there are none. */
  empty?: ReactNode;
  children?: ReactNode;
}

/** A bordered list of people (project members, organization members, invitations). */
export function MemberList({ scroll, empty, children }: Props) {
  return (
    <div className={cx(s.list, scroll && s.scroll)}>
      {empty && <div className={s.empty}>{empty}</div>}
      {children}
    </div>
  );
}
