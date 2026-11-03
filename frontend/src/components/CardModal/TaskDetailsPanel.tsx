import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Field } from "@/components/ui/Field/Field";
import { tasksApi } from "@/lib/api/tasks";
import { cx } from "@/lib/cx";
import { SPECIAL_TYPES, type Card, type Tag } from "@/types";
import type { Planning, TaskDetails } from "@/types/tasks";
import s from "./CardModal.module.scss";

interface Props {
  card: Card;
  planning: Planning;
  tags: Tag[];
  onChanged: (card: Card) => void;
  onError: (message: string) => void;
}

const detailsOf = (c: Card): TaskDetails => ({
  storyPoints: c.storyPoints, budgetHours: c.budgetHours, billable: c.billable, specialType: c.specialType,
  milestoneId: c.milestoneId, sprintId: c.sprintId, versionId: c.versionId, tagIds: c.tags.map((x) => x.id),
});

/** OmiTeam task fields in the card's sidebar; every change is saved right away. */
export function TaskDetailsPanel({ card, planning, tags, onChanged, onError }: Props) {
  const { t } = useTranslation();
  const [d, setD] = useState<TaskDetails>(() => detailsOf(card));
  useEffect(() => setD(detailsOf(card)), [card]);

  async function save(patch: Partial<TaskDetails>) {
    const next = { ...d, ...patch };
    setD(next);
    try {
      onChanged(await tasksApi.setDetails(card.id, next));
    } catch (err) {
      onError((err as Error).message);
      setD(detailsOf(card));
    }
  }

  const num = (v: string) => (v === "" ? null : Number(v));
  const select = (value: number | null, options: { id: number; name: string }[], onChange: (id: number | null) => void) => (
    <select value={value ?? ""} onChange={(e) => onChange(e.target.value ? Number(e.target.value) : null)}>
      <option value="">{t("tasks.none")}</option>
      {options.map((o) => (
        <option key={o.id} value={o.id}>
          {o.name}
        </option>
      ))}
    </select>
  );

  return (
    <>
      <div className={s.twoFields}>
        <Field label={t("tasks.storyPoints")}>
          <input
            type="number"
            min={0}
            max={1000}
            value={d.storyPoints ?? ""}
            onChange={(e) => setD({ ...d, storyPoints: num(e.target.value) })}
            onBlur={() => d.storyPoints !== card.storyPoints && save({})}
          />
        </Field>
        <Field label={t("tasks.budget")}>
          <input
            type="number"
            min={0}
            step="0.5"
            value={d.budgetHours ?? ""}
            onChange={(e) => setD({ ...d, budgetHours: num(e.target.value) })}
            onBlur={() => d.budgetHours !== card.budgetHours && save({})}
          />
        </Field>
      </div>
      <label className={s.check}>
        <input type="checkbox" checked={d.billable} onChange={(e) => save({ billable: e.target.checked })} />
        {t("tasks.billable")}
      </label>
      <Field label={t("tasks.specialType")}>
        <select value={d.specialType} onChange={(e) => save({ specialType: e.target.value as TaskDetails["specialType"] })}>
          {SPECIAL_TYPES.map((x) => (
            <option key={x} value={x}>
              {t(`tasks.specialTypes.${x}`)}
            </option>
          ))}
        </select>
      </Field>
      {planning.milestones.length > 0 && <Field label={t("tasks.milestone")}>{select(d.milestoneId, planning.milestones, (milestoneId) => save({ milestoneId }))}</Field>}
      {planning.sprints.length > 0 && (
        <Field label={t("tasks.sprint")}>
          {select(d.sprintId, planning.sprints.filter((x) => x.status !== "past" || x.id === d.sprintId), (sprintId) => save({ sprintId }))}
        </Field>
      )}
      {planning.versions.length > 0 && (
        <Field label={t("tasks.version")}>
          {select(d.versionId, planning.versions.filter((x) => x.status !== "archived" || x.id === d.versionId), (versionId) => save({ versionId }))}
        </Field>
      )}
      {tags.length > 0 && (
        <Field as="div" label={t("tasks.tags")}>
          <div className={s.tagPicker}>
            {tags.map((tag) => {
              const on = d.tagIds.includes(tag.id);
              return (
                <button
                  key={tag.id}
                  type="button"
                  aria-pressed={on}
                  className={cx(s.tagOption, on && s.tagOn)}
                  style={{ ["--tag" as string]: tag.color }}
                  onClick={() => save({ tagIds: on ? d.tagIds.filter((x) => x !== tag.id) : [...d.tagIds, tag.id] })}
                >
                  {tag.name}
                </button>
              );
            })}
          </div>
        </Field>
      )}
    </>
  );
}
