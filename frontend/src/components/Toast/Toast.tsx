import type { ReactNode } from "react";
import { cx } from "@/lib/cx";
import s from "./Toast.module.scss";

interface Props {
  tone: "info" | "warn";
  onClick: () => void;
  children: ReactNode;
}

/** A notice at the top of the screen (access granted or revoked, someone joined the call); click dismisses. */
export function Toast({ tone, onClick, children }: Props) {
  return (
    <div className={cx(s.toast, s[tone])} role="status" onClick={onClick}>
      {children}
    </div>
  );
}
