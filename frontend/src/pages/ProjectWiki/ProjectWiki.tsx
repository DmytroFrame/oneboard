import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { ProjectPage } from "@/components/ProjectPage/ProjectPage";
import { Banner } from "@/components/ui/Banner/Banner";
import { buttonClass } from "@/components/ui/Button/Button";
import { Panel } from "@/components/ui/Panel/Panel";
import { WikiTree } from "@/components/WikiTree/WikiTree";
import { useAuth } from "@/context/auth";
import { wikiApi } from "@/lib/api/wiki";
import { can } from "@/lib/companyRoles";
import type { Project } from "@/types";
import type { WikiItem } from "@/types/wiki";
import s from "./ProjectWiki.module.scss";

export default function ProjectWikiRoute() {
  return <ProjectPage>{(project) => <ProjectWikiView key={project.id} project={project} />}</ProjectPage>;
}

/** The project's wiki: articles open to its members. */
function ProjectWikiView({ project }: { project: Project }) {
  const { t } = useTranslation();
  const { user } = useAuth();
  const [items, setItems] = useState<WikiItem[] | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    wikiApi.project(project.id).then(setItems).catch((err) => setError(err.message));
  }, [project.id]);

  const visible = new Set(items?.map((a) => a.id));
  const roots = (items ?? []).filter((a) => !a.archived && (a.parentId === null || !visible.has(a.parentId)));
  return (
    <>
      <div className={s.head}>
        {can(user, "createWikiArticle") && project.role && (
          <Link to={`/wiki/new?project=${project.id}`} className={buttonClass({ variant: "primary", size: "sm" })}>
            + {t("wiki.newArticle")}
          </Link>
        )}
      </div>
      {error && <Banner>{error}</Banner>}
      {items && (
        <Panel compact>
          <WikiTree roots={roots} all={items} empty={t("wiki.projectEmpty")} />
        </Panel>
      )}
    </>
  );
}
