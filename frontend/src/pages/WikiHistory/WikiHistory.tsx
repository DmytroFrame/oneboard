import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useParams } from "react-router-dom";
import { Banner } from "@/components/ui/Banner/Banner";
import { Muted } from "@/components/ui/Muted/Muted";
import { Panel } from "@/components/ui/Panel/Panel";
import { WikiShell } from "@/components/WikiShell/WikiShell";
import { wikiApi } from "@/lib/api/wiki";
import { cx } from "@/lib/cx";
import { diffLines } from "@/lib/diff";
import type { WikiVersion } from "@/types/wiki";
import s from "./WikiHistory.module.scss";

/** An article's versions; picking one shows what changed since the version before it. */
export default function WikiHistory() {
  const { t, i18n } = useTranslation();
  const { id } = useParams();
  const [versions, setVersions] = useState<WikiVersion[] | null>(null);
  const [selected, setSelected] = useState<number | null>(null);
  const [pair, setPair] = useState<{ older: WikiVersion | null; newer: WikiVersion } | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    wikiApi
      .versions(Number(id))
      .then((v) => (setVersions(v), setSelected(v[0]?.id ?? null)))
      .catch((err) => setError(err.message));
  }, [id]);
  useEffect(() => {
    if (!versions || selected === null) return;
    const i = versions.findIndex((v) => v.id === selected);
    const olderId = versions[i + 1]?.id;
    Promise.all([wikiApi.version(selected), olderId ? wikiApi.version(olderId) : Promise.resolve(null)])
      .then(([newer, older]) => setPair({ newer, older }))
      .catch((err) => setError(err.message));
  }, [versions, selected]);

  const lines = useMemo(() => {
    if (!pair) return [];
    const text = (v: WikiVersion | null) => (v ? `# ${v.title}\n\n${v.content ?? ""}` : "");
    return diffLines(text(pair.older), text(pair.newer));
  }, [pair]);
  const when = (d: string) => new Intl.DateTimeFormat(i18n.language, { dateStyle: "medium", timeStyle: "short" }).format(new Date(d));

  return (
    <WikiShell>
      <Link to={`/wiki/${id}`} className={s.back}>
        ← {t("wiki.backToArticle")}
      </Link>
      <h1 className={s.title}>{t("wiki.history")}</h1>
      {error && <Banner>{error}</Banner>}
      {versions && versions.length === 0 && <Muted as="p">{t("wiki.noHistory")}</Muted>}
      {versions && versions.length > 0 && (
        <div className={s.layout}>
          <Panel compact className={s.versions}>
            <h2>{t("wiki.versions")}</h2>
            {versions.map((v, i) => (
              <button key={v.id} type="button" className={cx(s.version, v.id === selected && s.active)} onClick={() => setSelected(v.id)}>
                <strong>{i === 0 ? t("wiki.latest") : i === versions.length - 1 ? t("wiki.created") : v.title}</strong>
                <small>
                  {v.authorName || t("wiki.deletedUser")} · {when(v.createdAt)}
                </small>
              </button>
            ))}
          </Panel>
          <Panel compact className={s.diffPanel}>
            {pair && (
              <>
                <h2>{pair.older ? t("wiki.changesAfter", { date: when(pair.older.createdAt) }) : t("wiki.created")}</h2>
                <pre className={s.diff}>
                  {lines.map((l, i) => (
                    <div key={i} className={cx(s.line, l.kind === "added" && s.added, l.kind === "removed" && s.removed)}>
                      <span className={s.sign}>{l.kind === "added" ? "+" : l.kind === "removed" ? "−" : " "}</span>
                      {l.text || " "}
                    </div>
                  ))}
                </pre>
              </>
            )}
          </Panel>
        </div>
      )}
    </WikiShell>
  );
}
