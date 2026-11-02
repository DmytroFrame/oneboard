import type { ReactNode } from "react";
import s from "./AuthLayout.module.scss";

/** Grey centered text under the form ("No account? Sign up"). */
export function AuthNote({ children }: { children: ReactNode }) {
  return <p className={s.note}>{children}</p>;
}
