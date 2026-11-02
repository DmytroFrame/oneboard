import type { ReactNode } from "react";
import { cx } from "@/lib/cx";
import s from "./Banner.module.scss";

interface Props {
  tone?: "error" | "success";
  /** Shows a ✕ that calls it. */
  onDismiss?: () => void;
  children: ReactNode;
}

/** A page-level message above the content. */
export function Banner({ tone = "error", onDismiss, children }: Props) {
  return (
    <div className={cx(s.banner, s[tone])} role={tone === "error" ? "alert" : "status"}>
      {children}
      {onDismiss && (
        <button type="button" className={s.dismiss} onClick={onDismiss}>
          ✕
        </button>
      )}
    </div>
  );
}
