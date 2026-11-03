import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { Page } from "@/components/Page/Page";
import { ProjectBadge } from "@/components/ProjectBadge/ProjectBadge";
import { Banner } from "@/components/ui/Banner/Banner";
import { Button } from "@/components/ui/Button/Button";
import { Chip } from "@/components/ui/Chip/Chip";
import { IconButton } from "@/components/ui/IconButton/IconButton";
import { Muted } from "@/components/ui/Muted/Muted";
import { useAuth } from "@/context/auth";
import { useLayout } from "@/layouts/AppLayout/useLayout";
import { api } from "@/lib/api";
import { can } from "@/lib/companyRoles";
import { cx } from "@/lib/cx";
import type { Project } from "@/types";
import s from "./Projects.module.scss";

type Sort = "name" | "client" | "newest" | "billable";

/** All projects you can see: pinned first, with search, sorting and inactive ones on request. */
export default function Projects() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { projects: mine, reload, openCreateProject } = useLayout();
  const seeAll = can(user, "viewAllProjects");
  const [all, setAll] = useState<Project[] | null>(null);
  const [showAll, setShowAll] = useState(false);
  const [q, setQ] = useState("");
  const [sort, setSort] = useState<Sort>("name");
  const [inactive, setInactive] = useState(false);
  const [error, setError] = useState("");

  const loadAll = useCallback(() => {
    if (seeAll && showAll) api.allProjects().then(setAll).catch((err) => setError(err.message));
  }, [seeAll, showAll]);
  useEffect(loadAll, [loadAll, mine]);

  const source = seeAll && showAll ? (all ?? []) : mine;
  const list = useMemo(() => {
    const query = q.trim().toLowerCase();
    const filtered = source.filter(
      (p) => (inactive || p.active) && (!query || [p.name, p.key, p.client, p.organizationName].some((v) => v.toLowerCase().includes(query))),
    );
    const by: Record<Sort, (a: Project, b: Project) => number> = {
      name: (a, b) => a.name.localeCompare(b.name),
      client: (a, b) => a.client.localeCompare(b.client) || a.name.localeCompare(b.name),
      newest: (a, b) => b.createdAt.localeCompare(a.createdAt),
      billable: (a, b) => Number(b.billable) - Number(a.billable) || a.name.localeCompare(b.name),
    };
    return filtered.sort(by[sort]);
  }, [source, q, sort, inactive]);

  async function togglePin(p: Project) {
    try {
      await api.pinProject(p.id, !p.pinned);
      await reload();
      loadAll();
    } catch (err) {
      setError((err as Error).message);
    }
  }

  const pinned = list.filter((p) => p.pinned);
  const others = list.filter((p) => !p.pinned);

  const row = (p: Project) => (
    <div key={p.id} className={cx(s.row, !p.active && s.inactive)}>
      <ProjectBadge projectKey={p.key} />
      <div className={s.main}>
        {p.role ? (
          <Link to={`/projects/${p.id}`} className={s.name}>
            {p.name}
          </Link>
        ) : (
          <span className={s.name}>{p.name}</span>
        )}
        <small>
          {p.key} · {p.organizationName}
          {p.client && ` · ${p.client}`}
        </small>
      </div>
      <div className={s.chips}>
        {p.isMain && <Chip tone="accent">{t("projects.main")}</Chip>}
        {p.billable && <Chip tone="success">{t("projects.billable")}</Chip>}
        {!p.active && <Chip tone="danger">{t("projects.inactive")}</Chip>}
        {!p.role && <Chip>{t("projects.notMember")}</Chip>}
        <Muted>{t("home.members", { count: p.memberCount })}</Muted>
      </div>
      <IconButton aria-label={p.pinned ? t("projects.unpin") : t("projects.pin")} title={p.pinned ? t("projects.unpin") : t("projects.pin")} onClick={() => togglePin(p)}>
        {p.pinned ? "★" : "☆"}
      </IconButton>
    </div>
  );

  return (
    <Page>
      <div className={s.head}>
        <h1>{t("projects.title")}</h1>
        <Button variant="primary" onClick={() => openCreateProject()}>
          {t("org.newProject")}
        </Button>
      </div>
      <div className={s.filters}>
        <input className={s.search} placeholder={t("projects.search")} value={q} onChange={(e) => setQ(e.target.value)} />
        <select value={sort} onChange={(e) => setSort(e.target.value as Sort)} aria-label={t("projects.sortName")}>
          <option value="name">{t("projects.sortName")}</option>
          <option value="client">{t("projects.sortClient")}</option>
          <option value="newest">{t("projects.sortNewest")}</option>
          <option value="billable">{t("projects.sortBillable")}</option>
        </select>
        <label className={s.check}>
          <input type="checkbox" checked={inactive} onChange={(e) => setInactive(e.target.checked)} />
          {t("projects.showInactive")}
        </label>
        {seeAll && (
          <label className={s.check}>
            <input type="checkbox" checked={showAll} onChange={(e) => setShowAll(e.target.checked)} />
            {t("projects.allProjects")}
          </label>
        )}
      </div>
      {error && <Banner onDismiss={() => setError("")}>{error}</Banner>}
      {pinned.length > 0 && (
        <section className={s.group}>
          <h2>{t("projects.pinned")}</h2>
          {pinned.map(row)}
        </section>
      )}
      <section className={s.group}>
        {pinned.length > 0 && <h2>{t("projects.others")}</h2>}
        {others.map(row)}
        {list.length === 0 && <Muted as="p">{t("projects.empty")}</Muted>}
      </section>
    </Page>
  );
}
