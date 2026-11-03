import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { PlanningModal, type PlanningKind } from "@/components/PlanningModal/PlanningModal";
import { ProjectPage } from "@/components/ProjectPage/ProjectPage";
import { Banner } from "@/components/ui/Banner/Banner";
import { Button } from "@/components/ui/Button/Button";
import { Chip } from "@/components/ui/Chip/Chip";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog/ConfirmDialog";
import { Muted } from "@/components/ui/Muted/Muted";
import { Panel } from "@/components/ui/Panel/Panel";
import { useAuth } from "@/context/auth";
import { tasksApi } from "@/lib/api/tasks";
import { can } from "@/lib/companyRoles";
import { formatDate } from "@/lib/format";
import type { Project } from "@/types";
import type { Milestone, Planning, Sprint, Version } from "@/types/tasks";
import s from "./Planning.module.scss";

export default function PlanningRoute() {
  return <ProjectPage>{(project) => <PlanningView key={project.id} project={project} />}</ProjectPage>;
}

type Item = Milestone | Sprint | Version;

/** Milestones (status from their dates), sprints and versions, with their tasks' progress. */
function PlanningView({ project }: { project: Project }) {
  const { t, i18n } = useTranslation();
  const { user } = useAuth();
  const [data, setData] = useState<Planning | null>(null);
  const [showPast, setShowPast] = useState(false);
  const [editing, setEditing] = useState<{ kind: PlanningKind; item: Item | null } | null>(null);
  const [removing, setRemoving] = useState<{ kind: PlanningKind; item: Item } | null>(null);
  const [error, setError] = useState("");
  const manage = project.role === "owner" || project.role === "admin" || can(user, "editProject");

  const load = useCallback(() => {
    tasksApi.planning(project.id).then(setData).catch((err) => setError(err.message));
  }, [project.id]);
  useEffect(load, [load]);

  const date = (d: string | null) => (d ? formatDate(d, i18n.language) : "…");
  const today = new Date().toISOString().slice(0, 10);
  const milestoneState = (m: Milestone) =>
    m.endDate && m.endDate < today ? "finished" : m.startDate && m.startDate > today ? "upcoming" : "active";

  const section = <T extends Item>(kind: PlanningKind, title: string, items: T[], render: (item: T) => { badge: string; tone: "accent" | "success" | "neutral" | "warning"; dates: string; text: string }) => (
    <Panel compact>
      <div className={s.head}>
        <h2>{title}</h2>
        {manage && (
          <Button size="sm" onClick={() => setEditing({ kind, item: null })}>
            + {t("common.add")}
          </Button>
        )}
      </div>
      {items.length === 0 && <Muted as="p">{t("tasks.emptyPlanning")}</Muted>}
      {items.map((item) => {
        const r = render(item);
        const pct = item.cardCount ? Math.round((item.doneCount / item.cardCount) * 100) : 0;
        return (
          <div key={item.id} className={s.row}>
            <div className={s.main}>
              <div className={s.name}>
                {item.name} <Chip tone={r.tone}>{r.badge}</Chip>
              </div>
              <small>{r.dates}</small>
              {r.text && <p className={s.text}>{r.text}</p>}
            </div>
            <div className={s.progressWrap}>
              <div className={s.progress}>
                <span style={{ width: `${pct}%` }} />
              </div>
              <small>{t("tasks.tasksDone", { done: item.doneCount, total: item.cardCount })}</small>
            </div>
            {manage && (
              <div className={s.actions}>
                <Button size="sm" variant="link" onClick={() => setEditing({ kind, item })}>
                  ✎
                </Button>
                <Button size="sm" variant="danger" onClick={() => setRemoving({ kind, item })}>
                  ✕
                </Button>
              </div>
            )}
          </div>
        );
      })}
    </Panel>
  );

  if (!data) return null;
  const sprints = data.sprints.filter((x) => showPast || x.status !== "past");
  const versions = data.versions.filter((x) => showPast || x.status !== "archived");

  return (
    <>
      <label className={s.check}>
        <input type="checkbox" checked={showPast} onChange={(e) => setShowPast(e.target.checked)} />
        {t("tasks.showPast")}
      </label>
      {error && <Banner onDismiss={() => setError("")}>{error}</Banner>}
      {section("milestones", t("tasks.milestones"), data.milestones, (m) => ({
        badge: t(`tasks.milestoneStates.${milestoneState(m)}`),
        tone: milestoneState(m) === "active" ? "accent" : milestoneState(m) === "finished" ? "success" : "neutral",
        dates: `${date(m.startDate)} – ${date(m.endDate)}`,
        text: m.description,
      }))}
      {section("sprints", t("tasks.sprints"), sprints, (sp) => ({
        badge: t(`tasks.sprintStatuses.${sp.status}`),
        tone: sp.status === "current" ? "accent" : sp.status === "past" ? "neutral" : "warning",
        dates: `${date(sp.startDate)} – ${date(sp.endDate)}`,
        text: sp.goal,
      }))}
      {section("versions", t("tasks.versions"), versions, (v) => ({
        badge: t(`tasks.versionStatuses.${v.status}`),
        tone: v.status === "released" ? "success" : v.status === "archived" ? "neutral" : v.status === "upcoming" ? "warning" : "accent",
        dates: v.releaseDate ? `${t("tasks.releaseDate")}: ${date(v.releaseDate)}` : "",
        text: v.description,
      }))}

      {editing && (
        <PlanningModal kind={editing.kind} projectId={project.id} item={editing.item} onSaved={load} onClose={() => setEditing(null)} />
      )}
      {removing && (
        <ConfirmDialog
          title={t("tasks.deletePlanning", { name: removing.item.name })}
          confirmLabel={t("common.delete")}
          danger
          onClose={() => setRemoving(null)}
          onConfirm={async () => {
            await tasksApi.deletePlanning(removing.kind, removing.item.id);
            load();
          }}
        />
      )}
    </>
  );
}
