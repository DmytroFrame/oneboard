import type { ReactNode } from "react";
import s from "./FormError.module.scss";

export function FormError({ children }: { children: ReactNode }) {
  return <div className={s.error}>{children}</div>;
}
