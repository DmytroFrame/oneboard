import type { ReactNode } from "react";
import s from "./PageHeader.module.scss";

interface Props {
  breadcrumbs: ReactNode;
  title: ReactNode;
  /** A line under the title (counts, role). */
  meta?: ReactNode;
  /** A badge left of the text. */
  icon?: ReactNode;
  actions?: ReactNode;
}

/** Board and organization page heading: breadcrumbs, title and the page's main actions. */
export function PageHeader({ breadcrumbs, title, meta, icon, actions }: Props) {
  return (
    <div className={s.head}>
      <div className={s.main}>
        {icon}
        <div>
          <div className={s.breadcrumbs}>{breadcrumbs}</div>
          <h1 className={meta ? s.titleWithMeta : s.title}>{title}</h1>
          {meta}
        </div>
      </div>
      {actions && <div className={s.actions}>{actions}</div>}
    </div>
  );
}
