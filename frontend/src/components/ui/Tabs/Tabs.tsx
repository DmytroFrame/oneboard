import { cx } from "@/lib/cx";
import s from "./Tabs.module.scss";

export interface Tab<T extends string> {
  id: T;
  label: string;
  /** A red dot: something needs attention there. */
  alert?: boolean;
}

interface Props<T extends string> {
  tabs: Tab<T>[];
  value: T;
  onChange: (id: T) => void;
  className?: string;
}

/** A row of tabs; the page shows the selected tab's content below it. */
export function Tabs<T extends string>({ tabs, value, onChange, className }: Props<T>) {
  return (
    <div className={cx(s.tabs, className)} role="tablist">
      {tabs.map((tab) => (
        <button
          key={tab.id}
          type="button"
          role="tab"
          aria-selected={tab.id === value}
          className={cx(s.tab, tab.id === value && s.active)}
          onClick={() => onChange(tab.id)}
        >
          {tab.label}
          {tab.alert && <span className={s.dot} aria-hidden="true" />}
        </button>
      ))}
    </div>
  );
}
