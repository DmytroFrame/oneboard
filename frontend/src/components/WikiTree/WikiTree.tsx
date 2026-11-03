import { useState } from "react";
import { useTranslation } from "react-i18next";
import { NavLink } from "react-router-dom";
import { cx } from "@/lib/cx";
import type { WikiItem } from "@/types/wiki";
import s from "./WikiTree.module.scss";

interface Props {
  /** The roots to show; children are looked up in all. */
  roots: WikiItem[];
  all: WikiItem[];
  empty?: string;
}

/** Articles with their sub-articles, collapsible. */
export function WikiTree({ roots, all, empty }: Props) {
  if (!roots.length) return empty ? <p className={s.empty}>{empty}</p> : null;
  return (
    <ul className={s.tree}>
      {roots.map((a) => (
        <Node key={a.id} item={a} all={all} />
      ))}
    </ul>
  );
}

function Node({ item, all }: { item: WikiItem; all: WikiItem[] }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(true);
  const children = all.filter((c) => c.parentId === item.id && !c.archived);
  const isPrivate = !item.inheritAccess && item.companyAccess === "none" && item.sharedWith === 0 && item.projectId === null;
  return (
    <li>
      <div className={s.row}>
        {children.length > 0 ? (
          <button type="button" className={s.toggle} onClick={() => setOpen(!open)} aria-label={t(open ? "wiki.collapse" : "wiki.expand", { title: item.title })}>
            {open ? "▾" : "▸"}
          </button>
        ) : (
          <span className={s.toggle} />
        )}
        <NavLink to={`/wiki/${item.id}`} className={({ isActive }) => cx(s.link, isActive && s.active)}>
          {item.title}
        </NavLink>
        {isPrivate && (
          <span className={s.lock} title={t("wiki.onlyYou")}>
            🔒
          </span>
        )}
      </div>
      {open && children.length > 0 && (
        <ul className={s.tree}>
          {children.map((c) => (
            <Node key={c.id} item={c} all={all} />
          ))}
        </ul>
      )}
    </li>
  );
}

/** Splits visible articles into the sidebar's sections: roots are articles whose parent the viewer can't see. */
export function wikiSections(items: WikiItem[], userId: number) {
  const visible = new Set(items.filter((a) => !a.archived).map((a) => a.id));
  const roots = items.filter((a) => !a.archived && (a.parentId === null || !visible.has(a.parentId)));
  return {
    mine: roots.filter((a) => a.ownerId === userId),
    shared: roots.filter((a) => a.ownerId !== userId),
    archived: items.filter((a) => a.archived),
  };
}
