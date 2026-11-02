import type { FormHTMLAttributes } from "react";
import { cx } from "@/lib/cx";
import s from "./Form.module.scss";

/** A vertical form: fields stacked with even spacing. */
export function Form({ className, ...rest }: FormHTMLAttributes<HTMLFormElement>) {
  return <form className={cx(s.form, className)} {...rest} />;
}
