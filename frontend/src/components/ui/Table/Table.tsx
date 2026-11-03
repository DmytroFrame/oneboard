import type { ReactNode } from "react";
import { cx } from "@/lib/cx";
import s from "./Table.module.scss";

/** A scrollable data table on glass; children are <thead>/<tbody>. Cells are plain <th>/<td>. */
export function Table({ className, children }: { className?: string; children: ReactNode }) {
  return (
    <div className={cx(s.wrap, className)}>
      <table className={s.table}>{children}</table>
    </div>
  );
}

interface SortProps<T extends string> {
  id: T;
  sort: T;
  desc: boolean;
  onSort: (id: T) => void;
  children: ReactNode;
}

/** A header cell that sorts by its column. */
export function SortHeader<T extends string>({ id, sort, desc, onSort, children }: SortProps<T>) {
  const active = id === sort;
  return (
    <th aria-sort={active ? (desc ? "descending" : "ascending") : "none"}>
      <button type="button" className={cx(s.sort, active && s.sorted)} onClick={() => onSort(id)}>
        {children}
        <span aria-hidden="true">{active ? (desc ? "↓" : "↑") : "↕"}</span>
      </button>
    </th>
  );
}
