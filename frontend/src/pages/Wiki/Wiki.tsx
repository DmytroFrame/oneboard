import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useSearchParams } from "react-router-dom";
import { Banner } from "@/components/ui/Banner/Banner";
import { Muted } from "@/components/ui/Muted/Muted";
import { Panel } from "@/components/ui/Panel/Panel";
import { WikiShell } from "@/components/WikiShell/WikiShell";
import { wikiApi } from "@/lib/api/wiki";
import { formatDate } from "@/lib/format";
import type { WikiItem } from "@/types/wiki";
import s from "./Wiki.module.scss";

/** The wiki's start page: search results, or what you opened lately and the latest changes. */
export default function Wiki() {
  const { t, i18n } = useTranslation();
  const [params] = useSearchParams();
  const q = params.get("q") ?? "";
  const [home, setHome] = useState<{ recent: WikiItem[]; changed: WikiItem[] } | null>(null);
  const [results, setResults] = useState<WikiItem[] | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    setError("");
    if (q) {
      setResults(null);
      if ([...q.trim()].length < 2) {
        setError(t("wiki.searchShort"));
        return;
      }
      wikiApi.search(q).then(setResults).catch((err) => setError(err.message || t("wiki.searchFailed")));
    } else {
      wikiApi.home().then(setHome).catch((err) => setError(err.message));
    }
  }, [q, t]);

  const list = (items: WikiItem[], meta: boolean) => (
    <ul className={s.list}>
      {items.map((a) => (
        <li key={a.id}>
          <Link to={`/wiki/${a.id}`}>{a.title}</Link>
          {meta && (
            <small>
              {a.updatedByName} · {formatDate(a.updatedAt, i18n.language)}
            </small>
          )}
        </li>
      ))}
    </ul>
  );

  return (
    <WikiShell>
      <h1 className={s.title}>{q ? t("wiki.results") : t("wiki.title")}</h1>
      {error && <Banner>{error}</Banner>}
      {q && results && (
        <Panel compact>{results.length ? list(results, true) : <Muted as="p" className={s.flush}>{t("wiki.nothingFound")}</Muted>}</Panel>
      )}
      {!q && home && (
        <div className={s.columns}>
          <Panel compact>
            <h2>{t("wiki.recent")}</h2>
            {home.recent.length ? list(home.recent, false) : <Muted as="p" className={s.flush}>{t("wiki.recentEmpty")}</Muted>}
          </Panel>
          <Panel compact>
            <h2>{t("wiki.changed")}</h2>
            {home.changed.length ? list(home.changed, true) : <Muted as="p" className={s.flush}>{t("wiki.empty")}</Muted>}
          </Panel>
        </div>
      )}
    </WikiShell>
  );
}
