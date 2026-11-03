import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useParams } from "react-router-dom";
import { DescriptionEditor } from "@/components/DescriptionEditor/DescriptionEditor";
import { Banner } from "@/components/ui/Banner/Banner";
import { Button, buttonClass } from "@/components/ui/Button/Button";
import { Chip } from "@/components/ui/Chip/Chip";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog/ConfirmDialog";
import { WikiShareModal } from "@/components/WikiShareModal/WikiShareModal";
import { WikiShell } from "@/components/WikiShell/WikiShell";
import { useAuth } from "@/context/auth";
import { wikiApi } from "@/lib/api/wiki";
import { formatDate } from "@/lib/format";
import type { WikiPage } from "@/types/wiki";
import s from "./WikiArticle.module.scss";

/** An article: where it sits, who sees it, and its text (read-only; editing has its own page). */
export default function WikiArticle() {
  const { t, i18n } = useTranslation();
  const { user } = useAuth();
  const { id } = useParams();
  const [page, setPage] = useState<WikiPage | null>(null);
  const [sharing, setSharing] = useState(false);
  const [archiving, setArchiving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    setPage(null);
    setError("");
    wikiApi.get(Number(id)).then(setPage).catch((err) => setError(err.message));
  }, [id]);

  const access = (p: WikiPage) => {
    if (p.inheritAccess) return t("wiki.inherited", { title: p.accessFrom.title });
    if (p.companyAccess !== "none") return t("wiki.companyAccess");
    if (p.sharedWith > 0) return t("wiki.sharedWith", { count: p.sharedWith });
    if (p.projectId) return t("wiki.projectAccess");
    return t("wiki.onlyYou");
  };
  const edit = page?.level === "edit";

  return (
    <WikiShell version={page && `${page.id}:${page.updatedAt}:${page.archived}:${page.title}`}>
      {error && <Banner>{error}</Banner>}
      {page && (
        <>
          <nav className={s.path} aria-label={t("wiki.location")}>
            <Link to="/wiki">{t("wiki.title")}</Link>
            {page.path.map((c) => (
              <span key={c.id}>
                {" / "}
                <Link to={`/wiki/${c.id}`}>{c.title}</Link>
              </span>
            ))}
          </nav>
          <div className={s.head}>
            <h1>{page.title}</h1>
            <div className={s.actions}>
              {edit && (
                <Link to={`/wiki/${page.id}/edit`} className={buttonClass({ variant: "primary", size: "sm" })}>
                  {t("wiki.edit")}
                </Link>
              )}
              {edit && (
                <Button size="sm" onClick={() => setSharing(true)}>
                  {t("wiki.share")}
                </Button>
              )}
              {edit && (
                <Link to={`/wiki/new?parent=${page.id}`} className={buttonClass({ size: "sm" })}>
                  {t("wiki.newSubArticle")}
                </Link>
              )}
              <Link to={`/wiki/${page.id}/history`} className={buttonClass({ size: "sm" })}>
                {t("wiki.history")}
              </Link>
              {edit && (
                <Button size="sm" onClick={() => (page.archived ? wikiApi.setArchived(page.id, false).then(setPage).catch((e) => setError(e.message)) : setArchiving(true))}>
                  {page.archived ? t("wiki.activate") : t("wiki.archiveAction")}
                </Button>
              )}
            </div>
          </div>
          <div className={s.meta}>
            <Chip>{access(page)}</Chip>
            {page.archived && <Chip tone="warning">{t("wiki.archivedBadge")}</Chip>}
            {page.ownerId !== user!.id && page.level === "read" && <Chip>{t("wiki.readOnly")}</Chip>}
            <span>
              {t("wiki.updated", { name: page.updatedByName || t("wiki.deletedUser"), date: formatDate(page.updatedAt, i18n.language) })}
            </span>
          </div>
          <div className={s.content}>
            <DescriptionEditor key={`${page.id}:${page.updatedAt}`} user={{ name: user!.name, color: user!.color }} initialMarkdown={page.content} editable={false} />
          </div>
          {sharing && <WikiShareModal page={page} onClose={() => setSharing(false)} onSaved={setPage} />}
          {archiving && (
            <ConfirmDialog
              title={t("wiki.archiveConfirm")}
              confirmLabel={t("wiki.archiveAction")}
              onClose={() => setArchiving(false)}
              onConfirm={async () => {
                setPage(await wikiApi.setArchived(page.id, true));
                setArchiving(false);
              }}
            />
          )}
        </>
      )}
    </WikiShell>
  );
}
