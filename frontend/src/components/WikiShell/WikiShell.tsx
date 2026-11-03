import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { Page } from "@/components/Page/Page";
import { buttonClass } from "@/components/ui/Button/Button";
import { WikiTree, wikiSections } from "@/components/WikiTree/WikiTree";
import { useAuth } from "@/context/auth";
import { wikiApi } from "@/lib/api/wiki";
import { can } from "@/lib/companyRoles";
import { cx } from "@/lib/cx";
import type { WikiItem } from "@/types/wiki";
import s from "./WikiShell.module.scss";

interface Props {
  /** Changing it reloads the tree (after creating, renaming, moving or archiving). */
  version?: unknown;
  children: ReactNode;
}

/** The wiki's frame: search, "new article", groups, and the article tree beside the page. */
export function WikiShell({ version, children }: Props) {
  const { t } = useTranslation();
  const { user } = useAuth();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const [items, setItems] = useState<WikiItem[]>([]);
  const [q, setQ] = useState(params.get("q") ?? "");
  const [showArchive, setShowArchive] = useState(false);

  useEffect(() => {
    wikiApi.list().then(setItems).catch(() => {});
  }, [version]);

  const sections = wikiSections(items, user!.id);
  const search = (e: FormEvent) => {
    e.preventDefault();
    navigate(q.trim() ? `/wiki?q=${encodeURIComponent(q.trim())}` : "/wiki");
  };

  return (
    <Page className={s.page}>
      <aside className={s.side}>
        <form onSubmit={search}>
          <input type="search" value={q} onChange={(e) => setQ(e.target.value)} placeholder={t("wiki.search")} aria-label={t("wiki.search")} />
        </form>
        <div className={s.buttons}>
          {can(user, "createWikiArticle") && (
            <Link to="/wiki/new" className={buttonClass({ variant: "primary", size: "sm" })}>
              + {t("wiki.newArticle")}
            </Link>
          )}
          <Link to="/wiki/groups" className={buttonClass({ size: "sm" })}>
            {t("wiki.groups")}
          </Link>
        </div>
        <Link to="/wiki" className={s.home}>
          {t("wiki.title")}
        </Link>
        <h3>{t("wiki.mine")}</h3>
        <WikiTree roots={sections.mine} all={items} empty={t("wiki.noArticles")} />
        <h3>{t("wiki.shared")}</h3>
        <WikiTree roots={sections.shared} all={items} empty={t("wiki.nothingShared")} />
        <label className={s.archiveToggle}>
          <input type="checkbox" checked={showArchive} onChange={(e) => setShowArchive(e.target.checked)} />
          {t("wiki.archive")} ({sections.archived.length})
        </label>
        {showArchive && (
          <ul className={cx(s.archived)}>
            {sections.archived.map((a) => (
              <li key={a.id}>
                <Link to={`/wiki/${a.id}`}>{a.title}</Link>
              </li>
            ))}
          </ul>
        )}
      </aside>
      <div className={s.main}>{children}</div>
    </Page>
  );
}
