import type { ButtonHTMLAttributes } from "react";
import { cx } from "@/lib/cx";
import s from "./MenuItem.module.scss";

type Props = ButtonHTMLAttributes<HTMLButtonElement> & { danger?: boolean };

export function MenuItem({ danger, className, type = "button", ...rest }: Props) {
  return <button type={type} className={cx(s.item, danger && s.danger, className)} {...rest} />;
}
