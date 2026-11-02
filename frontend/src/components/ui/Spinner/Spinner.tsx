import { cx } from "@/lib/cx";
import s from "./Spinner.module.scss";

export function Spinner({ className }: { className?: string }) {
  return <span className={cx(s.spinner, className)} />;
}
