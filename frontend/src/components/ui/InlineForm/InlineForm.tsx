import type { FormHTMLAttributes } from "react";
import { cx } from "@/lib/cx";
import s from "./InlineForm.module.scss";

/** A one-line form: inputs stretch, buttons keep their size. */
export function InlineForm({ className, ...rest }: FormHTMLAttributes<HTMLFormElement>) {
  return <form className={cx(s.form, className)} {...rest} />;
}
