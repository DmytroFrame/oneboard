import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { Button } from "@/components/ui/Button/Button";
import { Field } from "@/components/ui/Field/Field";
import { FormError } from "@/components/ui/FormError/FormError";
import { Modal, ModalActions, ModalContent } from "@/components/ui/Modal/Modal";
import { useAuth } from "@/context/auth";
import { hrApi } from "@/lib/api/hr";
import { can } from "@/lib/companyRoles";
import { cx } from "@/lib/cx";
import type { User } from "@/types";
import { CANDIDATE_ATTRIBUTES, CANDIDATE_STATUSES, type Candidate, type CandidateInput } from "@/types/hr";
import type { DirectoryItem } from "@/types/people";
import s from "./CandidateModal.module.scss";

interface Props {
  candidate: Candidate;
  channels: DirectoryItem[];
  recruiters: User[];
  onClose: () => void;
  onSaved: (c: Candidate) => void;
}

const num = (v: string) => (v.trim() === "" ? null : Number(v));

/** The recruiting side of a candidate; contacts and links live in their profile. */
export function CandidateModal({ candidate, channels, recruiters, onClose, onSaved }: Props) {
  const { t } = useTranslation();
  const { user } = useAuth();
  const money = can(user, "viewSalary");
  const [form, setForm] = useState<CandidateInput>({
    status: candidate.status,
    attributes: candidate.attributes,
    channel: candidate.channel,
    minSalary: candidate.minSalary,
    maxSalary: candidate.maxSalary,
    minRate: candidate.minRate,
    maxRate: candidate.maxRate,
    hiringCommission: candidate.hiringCommission,
    commissionReceiver: candidate.commissionReceiver,
    trelloLink: candidate.trelloLink,
    recruiterId: candidate.recruiter?.id ?? null,
  });
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const set = <K extends keyof CandidateInput>(k: K, v: CandidateInput[K]) => setForm((f) => ({ ...f, [k]: v }));
  const moneyField = (k: "minSalary" | "maxSalary" | "minRate" | "maxRate" | "hiringCommission") => (
    <Field label={t(`candidates.${k}`)}>
      <input type="number" min={0} step="any" value={form[k] ?? ""} onChange={(e) => set(k, num(e.target.value))} />
    </Field>
  );

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      onSaved(await hrApi.update(candidate.user.id, form));
      onClose();
    } catch (err) {
      setError((err as Error).message);
      setBusy(false);
    }
  }

  return (
    <Modal title={t("candidates.editTitle", { name: candidate.user.name })} onClose={onClose} wide>
      <ModalContent onSubmit={submit}>
        <div className={s.grid}>
          <Field label={t("candidates.status")}>
            <select value={form.status} onChange={(e) => set("status", e.target.value as CandidateInput["status"])}>
              {CANDIDATE_STATUSES.map((st) => (
                <option key={st} value={st}>
                  {t(`candidates.statuses.${st}`)}
                </option>
              ))}
            </select>
          </Field>
          <Field label={t("candidates.channel")}>
            <select value={form.channel} onChange={(e) => set("channel", e.target.value)}>
              <option value="">—</option>
              {channels.map((c) => (
                <option key={c.id} value={c.name}>
                  {c.name}
                </option>
              ))}
              {form.channel && !channels.some((c) => c.name === form.channel) && <option value={form.channel}>{form.channel}</option>}
            </select>
          </Field>
          <Field label={t("candidates.recruiter")}>
            <select value={form.recruiterId ?? 0} onChange={(e) => set("recruiterId", Number(e.target.value) || null)}>
              <option value={0}>—</option>
              {recruiters.map((r) => (
                <option key={r.id} value={r.id}>
                  {r.name}
                </option>
              ))}
            </select>
          </Field>
          <Field label={t("candidates.trelloLink")}>
            <input type="url" value={form.trelloLink} maxLength={500} onChange={(e) => set("trelloLink", e.target.value)} placeholder="https://trello.com/…" />
          </Field>
        </div>
        <Field as="div" label={t("candidates.attributes")}>
          <div className={s.attrs}>
            {CANDIDATE_ATTRIBUTES.map((a) => {
              const on = form.attributes.includes(a);
              return (
                <button
                  key={a}
                  type="button"
                  aria-pressed={on}
                  className={cx(s.attr, on && s.on)}
                  onClick={() => set("attributes", on ? form.attributes.filter((x) => x !== a) : [...form.attributes, a])}
                >
                  {t(`candidates.attributeNames.${a}`)}
                </button>
              );
            })}
          </div>
        </Field>
        {money && (
          <div className={s.grid}>
            {moneyField("minSalary")}
            {moneyField("maxSalary")}
            {moneyField("minRate")}
            {moneyField("maxRate")}
            {moneyField("hiringCommission")}
            <Field label={t("candidates.commissionReceiver")}>
              <input value={form.commissionReceiver} maxLength={200} onChange={(e) => set("commissionReceiver", e.target.value)} />
            </Field>
          </div>
        )}
        <Link to={`/people/${candidate.user.id}`} className={s.profile}>
          {t("candidates.profileHint")} →
        </Link>
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
