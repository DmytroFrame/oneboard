import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/Button/Button";
import { Field } from "@/components/ui/Field/Field";
import { FormError } from "@/components/ui/FormError/FormError";
import { useAuth } from "@/context/auth";
import { api } from "@/lib/api";
import { can } from "@/lib/companyRoles";
import type { Project, ProjectSections, ProjectSettings } from "@/types";
import s from "./ProjectDetailsForm.module.scss";

const SECTIONS: (keyof ProjectSections)[] = ["backlog", "planning", "comments", "statistics", "wiki"];

/** OmiTeam project fields: client, dates, active / billable, budget, main project, tabs. */
export function ProjectDetailsForm({ project, onSaved }: { project: Project; onSaved: () => Promise<void> }) {
  const { t } = useTranslation();
  const { user } = useAuth();
  const [form, setForm] = useState<ProjectSettings>({
    client: project.client, description: project.description, startDate: project.startDate, active: project.active,
    billable: project.billable, budgetHours: project.budgetHours, isMain: project.isMain, sections: { ...project.sections },
  });
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);
  const [busy, setBusy] = useState(false);
  const set = (patch: Partial<ProjectSettings>) => (setForm((f) => ({ ...f, ...patch })), setSaved(false));

  async function save(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api.updateProjectSettings(project.id, form);
      await onSaved();
      setSaved(true);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <form className={s.form} onSubmit={save}>
      <div className={s.grid}>
        <Field label={t("projects.client")}>
          <input value={form.client} maxLength={100} onChange={(e) => set({ client: e.target.value })} />
        </Field>
        <Field label={t("projects.startDate")}>
          <input type="date" value={form.startDate ?? ""} onChange={(e) => set({ startDate: e.target.value || null })} />
        </Field>
        <Field label={t("projects.budget")}>
          <input
            type="number"
            min={0}
            step="0.5"
            value={form.budgetHours ?? ""}
            disabled={!can(user, "editBudget")}
            onChange={(e) => set({ budgetHours: e.target.value === "" ? null : Number(e.target.value) })}
          />
        </Field>
      </div>
      <Field label={t("projects.description")}>
        <textarea rows={2} value={form.description} maxLength={5000} onChange={(e) => set({ description: e.target.value })} />
      </Field>
      <label className={s.check}>
        <input type="checkbox" checked={form.active} onChange={(e) => set({ active: e.target.checked })} />
        {t("projects.active")}
      </label>
      <label className={s.check}>
        <input type="checkbox" checked={form.billable} onChange={(e) => set({ billable: e.target.checked })} />
        <span>
          {t("projects.billable")} <small>{t("projects.billableHint")}</small>
        </span>
      </label>
      {can(user, "openBackwoods") && (
        <label className={s.check}>
          <input type="checkbox" checked={form.isMain} onChange={(e) => set({ isMain: e.target.checked })} />
          {t("projects.isMain")}
        </label>
      )}
      <div className={s.sections}>
        <span className={s.label}>{t("projects.sections")}</span>
        {SECTIONS.map((key) => (
          <label key={key} className={s.check}>
            <input type="checkbox" checked={form.sections[key]} onChange={(e) => set({ sections: { ...form.sections, [key]: e.target.checked } })} />
            {t(`projects.sectionNames.${key}`)}
          </label>
        ))}
      </div>
      {error && <FormError>{error}</FormError>}
      <div className={s.actions}>
        {saved && <span className={s.saved}>✓ {t("projects.detailsSaved")}</span>}
        <Button variant="primary" disabled={busy}>
          {t("projects.saveDetails")}
        </Button>
      </div>
    </form>
  );
}
