import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog/ConfirmDialog";
import { Field } from "@/components/ui/Field/Field";
import { Muted } from "@/components/ui/Muted/Muted";
import { tasksApi, type BulkTarget } from "@/lib/api/tasks";
import type { Column } from "@/types";
import type { Planning } from "@/types/tasks";

interface Props {
  column: Column;
  columns: Column[];
  planning: Planning;
  onDone: (moved: number) => void;
  onClose: () => void;
}

/** Moves the top tasks of a column to another column, the backlog, a milestone, sprint or version. */
export function BulkMoveModal({ column, columns, planning, onDone, onClose }: Props) {
  const { t } = useTranslation();
  const [target, setTarget] = useState<BulkTarget>("column");
  const [targetId, setTargetId] = useState<number | null>(null);
  const options: Record<BulkTarget, { id: number; name: string }[]> = {
    column: columns.filter((c) => c.id !== column.id),
    backlog: [],
    milestone: planning.milestones,
    sprint: planning.sprints.filter((x) => x.status !== "past"),
    version: planning.versions.filter((x) => x.status !== "archived"),
  };
  const targets: { id: BulkTarget; label: string }[] = [
    { id: "column", label: t("tasks.bulkColumn") },
    { id: "backlog", label: t("tasks.bulkBacklog") },
    { id: "milestone", label: t("tasks.bulkMilestone") },
    { id: "sprint", label: t("tasks.bulkSprint") },
    { id: "version", label: t("tasks.bulkVersion") },
  ];

  return (
    <ConfirmDialog
      title={t("tasks.bulkTitle", { name: column.name })}
      confirmLabel={t("tasks.move")}
      onClose={onClose}
      onConfirm={async () => {
        const res = await tasksApi.bulkMove(column.id, target, target === "backlog" ? null : (targetId ?? options[target][0]?.id ?? null));
        onDone(res.moved);
      }}
    >
      <Field label={t("tasks.bulkTo")}>
        <select value={target} onChange={(e) => (setTarget(e.target.value as BulkTarget), setTargetId(null))}>
          {targets.map((x) => (
            <option key={x.id} value={x.id}>
              {x.label}
            </option>
          ))}
        </select>
      </Field>
      {target !== "backlog" && (
        <Field label={targets.find((x) => x.id === target)!.label}>
          <select value={targetId ?? options[target][0]?.id ?? ""} onChange={(e) => setTargetId(Number(e.target.value))}>
            {options[target].map((o) => (
              <option key={o.id} value={o.id}>
                {o.name}
              </option>
            ))}
          </select>
        </Field>
      )}
      <Muted as="p">{t("tasks.bulkHint", { limit: 100 })}</Muted>
    </ConfirmDialog>
  );
}
