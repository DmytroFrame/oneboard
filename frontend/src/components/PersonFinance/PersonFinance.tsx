import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/Button/Button";
import { Field } from "@/components/ui/Field/Field";
import { FormError } from "@/components/ui/FormError/FormError";
import { Panel } from "@/components/ui/Panel/Panel";
import { peopleApi } from "@/lib/api/people";
import type { Person, ProfileFinance } from "@/types/people";
import s from "./PersonFinance.module.scss";

/** Salary, rate and payment details. HR edits all of it; the person keeps their own payment details. */
export function PersonFinance({ person, onSaved }: { person: Person; onSaved: (p: Person) => void }) {
  const { t } = useTranslation();
  const [f, setF] = useState<ProfileFinance>(person.finance ?? {});
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);
  const hr = person.access.editFinance;
  const own = person.access.self;

  async function save(e: FormEvent) {
    e.preventDefault();
    setError("");
    setSaved(false);
    try {
      onSaved(
        await peopleApi.update(person.id, {
          name: person.name, nameCyrillic: person.nameCyrillic, birthday: person.birthday, startDate: person.startDate,
          trialUntil: person.trialUntil, workingHours: person.workingHours, billable: person.billable, details: person.details, finance: f,
        }),
      );
      setSaved(true);
    } catch (err) {
      setError((err as Error).message);
    }
  }

  const text = (key: keyof ProfileFinance, label: string, editable: boolean) => (
    <Field label={t(`people.${label}` as "people.salary")}>
      <input value={(f[key] as string | undefined) ?? ""} disabled={!editable} onChange={(e) => setF({ ...f, [key]: e.target.value })} />
    </Field>
  );
  const money = (key: "salary" | "rate", label: string) => (
    <Field label={t(`people.${label}` as "people.salary")}>
      <input type="number" min={0} step="0.01" value={f[key] ?? ""} disabled={!hr} onChange={(e) => setF({ ...f, [key]: e.target.value === "" ? undefined : Number(e.target.value) })} />
    </Field>
  );

  return (
    <Panel>
      <form className={s.form} onSubmit={save}>
        <div className={s.grid}>
          {money("salary", "salary")}
          {money("rate", "rate")}
          {text("currency", "currency", hr)}
          <label className={s.check}>
            <input type="checkbox" checked={!!f.fop} disabled={!hr && !own} onChange={(e) => setF({ ...f, fop: e.target.checked })} />
            {t("people.fop")}
          </label>
          {text("payoneerEmail", "payoneer", hr || own)}
          {text("monobankCard", "monobank", hr || own)}
          {text("privatBankCard", "privatbank", hr || own)}
          {text("taxId", "taxId", hr || own)}
        </div>
        {error && <FormError>{error}</FormError>}
        {(hr || own) && (
          <div className={s.actions}>
            {saved && <span className={s.saved}>✓ {t("people.saved")}</span>}
            <Button variant="primary">{t("people.save")}</Button>
          </div>
        )}
      </form>
    </Panel>
  );
}
