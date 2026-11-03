import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/Button/Button";
import { Field } from "@/components/ui/Field/Field";
import { FormError } from "@/components/ui/FormError/FormError";
import { Modal, ModalActions, ModalContent } from "@/components/ui/Modal/Modal";
import { tasksApi } from "@/lib/api/tasks";
import type { Milestone, Sprint, Version } from "@/types/tasks";
import s from "./PlanningModal.module.scss";

export type PlanningKind = "milestones" | "sprints" | "versions";
interface Item {
  id?: number;
  name?: string;
  description?: string;
  goal?: string;
  startDate?: string | null;
  endDate?: string | null;
  releaseDate?: string | null;
  status?: string;
}

interface Props {
  kind: PlanningKind;
  projectId: number;
  item: Item | null; // null = new
  onSaved: () => void;
  onClose: () => void;
}

/** Create or edit a milestone, sprint or version. */
export function PlanningModal({ kind, projectId, item, onSaved, onClose }: Props) {
  const { t } = useTranslation();
  const [form, setForm] = useState<Item>(
    item ?? { name: "", description: "", goal: "", startDate: null, endDate: null, releaseDate: null, status: kind === "sprints" ? "new" : "in_development" },
  );
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const set = (patch: Item) => setForm((f) => ({ ...f, ...patch }));
  const title = item?.id ? item.name! : t(kind === "milestones" ? "tasks.newMilestone" : kind === "sprints" ? "tasks.newSprint" : "tasks.newVersion");

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const body = { ...form, projectId };
      if (kind === "milestones") await tasksApi.saveMilestone(body as Partial<Milestone> & { projectId: number });
      else if (kind === "sprints") await tasksApi.saveSprint(body as Partial<Sprint> & { projectId: number });
      else await tasksApi.saveVersion(body as Partial<Version> & { projectId: number });
      onSaved();
      onClose();
    } catch (err) {
      setError((err as Error).message);
      setBusy(false);
    }
  }

  const date = (key: "startDate" | "endDate" | "releaseDate", label: string) => (
    <Field label={label}>
      <input type="date" value={form[key] ?? ""} onChange={(e) => set({ [key]: e.target.value || null })} />
    </Field>
  );

  return (
    <Modal title={title} onClose={onClose}>
      <ModalContent onSubmit={submit}>
        <Field label={t("tasks.name")}>
          <input value={form.name ?? ""} onChange={(e) => set({ name: e.target.value })} maxLength={100} required autoFocus />
        </Field>
        {kind !== "versions" && (
          <div className={s.dates}>
            {date("startDate", t("tasks.start"))}
            {date("endDate", t("tasks.end"))}
          </div>
        )}
        {kind === "versions" && date("releaseDate", t("tasks.releaseDate"))}
        {kind !== "milestones" && (
          <Field label={t("tasks.status")}>
            <select value={form.status} onChange={(e) => set({ status: e.target.value })}>
              {(kind === "sprints" ? ["new", "current", "past"] : ["in_development", "upcoming", "released", "archived"]).map((st) => (
                <option key={st} value={st}>
                  {t((kind === "sprints" ? `tasks.sprintStatuses.${st}` : `tasks.versionStatuses.${st}`) as "tasks.sprintStatuses.new")}
                </option>
              ))}
            </select>
          </Field>
        )}
        {kind === "sprints" ? (
          <Field label={t("tasks.goal")}>
            <textarea rows={2} value={form.goal ?? ""} onChange={(e) => set({ goal: e.target.value })} />
          </Field>
        ) : (
          <Field label={t("tasks.description")}>
            <textarea rows={2} value={form.description ?? ""} onChange={(e) => set({ description: e.target.value })} />
          </Field>
        )}
        {error && <FormError>{error}</FormError>}
        <ModalActions>
          <Button type="button" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button variant="primary" disabled={busy}>
            {t("common.save")}
          </Button>
        </ModalActions>
      </ModalContent>
    </Modal>
  );
}
