import { cx } from "@/lib/cx";
import s from "./Segmented.module.scss";

interface Props<T extends string> {
  options: { id: T; label: string }[];
  value: T;
  onChange: (id: T) => void;
  label: string;
}

/** A compact choice of one option (quick filters, periods). */
export function Segmented<T extends string>({ options, value, onChange, label }: Props<T>) {
  return (
    <div className={s.group} role="radiogroup" aria-label={label}>
      {options.map((o) => (
        <button key={o.id} type="button" role="radio" aria-checked={o.id === value} className={cx(s.option, o.id === value && s.on)} onClick={() => onChange(o.id)}>
          {o.label}
        </button>
      ))}
    </div>
  );
}
