import type { ButtonHTMLAttributes } from "react";
import { cx } from "@/lib/cx";
import s from "./Button.module.scss";

export type ButtonVariant = "primary" | "ghost" | "selected" | "light" | "danger" | "dangerSolid" | "link";
export type ButtonSize = "sm" | "md" | "lg";

interface Look {
  variant?: ButtonVariant;
  size?: ButtonSize;
  /** Full width. */
  block?: boolean;
  className?: string;
}

/** A button's class names, for links (<Link>, <a>) that should look like one. */
export function buttonClass({ variant = "ghost", size = "md", block, className }: Look = {}) {
  return cx(s.button, s[variant], size !== "md" && s[size], block && s.block, className);
}

type Props = Look & ButtonHTMLAttributes<HTMLButtonElement>;

export function Button({ variant, size, block, className, ...rest }: Props) {
  return <button className={buttonClass({ variant, size, block, className })} {...rest} />;
}
