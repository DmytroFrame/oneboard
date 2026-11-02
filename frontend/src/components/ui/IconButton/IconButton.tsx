import type { ButtonHTMLAttributes } from "react";
import { cx } from "@/lib/cx";
import s from "./IconButton.module.scss";

interface Look {
  size?: "sm" | "md";
  className?: string;
}

/** A round icon button's class names, for links that should look like one. */
export function iconButtonClass({ size = "md", className }: Look = {}) {
  return cx(s.iconButton, size === "sm" && s.sm, className);
}

type Props = Look & ButtonHTMLAttributes<HTMLButtonElement>;

/** A round, borderless button holding an icon; give it an aria-label. */
export function IconButton({ size, className, type = "button", ...rest }: Props) {
  return <button type={type} className={iconButtonClass({ size, className })} {...rest} />;
}
