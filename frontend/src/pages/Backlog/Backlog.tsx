import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { Avatar } from "@/components/Avatar/Avatar";
import { PriorityIcon } from "@/components/PriorityIcon/PriorityIcon";
import { ProjectPage } from "@/components/ProjectPage/ProjectPage";
import { Banner } from "@/components/ui/Banner/Banner";
import { Chip } from "@/components/ui/Chip/Chip";
import { Muted } from "@/components/ui/Muted/Muted";
import { Pagination } from "@/components/ui/Pagination/Pagination";
import { Table } from "@/components/ui/Table/Table";
import { useDebounced } from "@/hooks/useDebounced";
import { api } from "@/lib/api";
import { tasksApi } from "@/lib/api/tasks";
import { formatMinutes } from "@/lib/format";
import type { Card, Column, Member, Project } from "@/types";
import type { Milestone } from "@/types/tasks";
import s from "./Backlog.module.scss";

const PAGE = 20;

export default function BacklogRoute() {
  return <ProjectPage>{(project) => <Backlog key={project.id} project={project} />}</ProjectPage>;
}

/** Unplanned tasks: everything in the project's backlog columns, with search and a milestone filter. */
function Backlog({ project }: { project: Project }) {
  const { t } = useTranslation();
  const [column, setColumn] = useState<Column | null | undefined>(undefined);
  const [cards, setCards] = useState<Card[]>([]);
  const [total, setTotal] = useState(0);
  const [cursors, setCursors] = useState<(string | null)[]>([null]);
  const [page, setPage] = useState(0);
  const [q, setQ] = useState("");
  const [milestone, setMilestone] = useState<number | null>(null);
  const [milestones, setMilestones] = useState<Milestone[]>([]);
  const [members, setMembers] = useState<Member[]>([]);
  const [error, setError] = useState("");
  const query = useDebounced(q, 300);

  useEffect(() => {
    api.columns(project.id).then((cols) => setColumn(cols.find((c) => c.status === "backlog") ?? null)).catch((err) => setError(err.message));
    tasksApi.planning(project.id).then((p) => setMilestones(p.milestones)).catch(() => {});
    api.members(project.id).then(setMembers).catch(() => {});
  }, [project.id]);

  const load = useCallback(
    (pageIndex: number, cursor: string | null) => {
      if (!column) return;
      api
        .cards(project.id, column.id, { q: query, assignees: [], unassigned: false, milestone }, cursor, PAGE)
        .then((res) => {
          setCards(res.cards);
          setTotal(res.total);
          setPage(pageIndex);
          setCursors((c) => {
            const next = c.slice(0, pageIndex + 1);
            next[pageIndex + 1] = res.nextCursor;
            return next;
          });
        })
        .catch((err) => setError(err.message));
    },
    [project.id, column, query, milestone],
  );
  useEffect(() => load(0, null), [load]);

  if (column === null) return <Muted as="p">{t("tasks.backlogNoColumn")}</Muted>;
  const byId = new Map(members.map((m) => [m.id, m]));

  return (
    <>
      <div className={s.filters}>
        <input className={s.search} placeholder={t("board.search")} value={q} onChange={(e) => setQ(e.target.value)} />
        {milestones.length > 0 && (
          <select value={milestone ?? ""} onChange={(e) => setMilestone(e.target.value ? Number(e.target.value) : null)}>
            <option value="">{t("tasks.anyMilestone")}</option>
            {milestones.map((m) => (
              <option key={m.id} value={m.id}>
                {m.name}
              </option>
            ))}
          </select>
        )}
      </div>
      {error && <Banner onDismiss={() => setError("")}>{error}</Banner>}
      {cards.length === 0 ? (
        <Muted as="p">{t("tasks.backlogEmpty")}</Muted>
      ) : (
        <Table>
          <tbody>
            {cards.map((c) => (
              <tr key={c.id}>
                <td className={s.key}>
                  <PriorityIcon priority={c.priority} /> {c.key}
                </td>
                <td className={s.title}>
                  <Link to={`/projects/${project.id}?card=${c.id}`}>{c.title}</Link>
                  {c.tags.map((tag) => (
                    <Chip key={tag.id} color={tag.color}>
                      {tag.name}
                    </Chip>
                  ))}
                </td>
                <td>{c.storyPoints != null && <Chip tone="accent">{c.storyPoints}</Chip>}</td>
                <td>{c.trackedMinutes > 0 && formatMinutes(c.trackedMinutes)}</td>
                <td>
                  <Avatar user={c.assigneeId ? byId.get(c.assigneeId) : null} size={24} />
                </td>
              </tr>
            ))}
          </tbody>
        </Table>
      )}
      <Pagination offset={page * PAGE} limit={PAGE} total={total} onChange={(offset) => load(offset / PAGE, cursors[offset / PAGE] ?? null)} />
    </>
  );
}
