import { useEffect, useMemo, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/Button/Button";
import { Field } from "@/components/ui/Field/Field";
import { FormError } from "@/components/ui/FormError/FormError";
import { Modal, ModalActions, ModalContent } from "@/components/ui/Modal/Modal";
import { Muted } from "@/components/ui/Muted/Muted";
import { useAuth } from "@/context/auth";
import { addDays, timeApi } from "@/lib/api/time";
import type { User } from "@/types";
import { VACATION_TYPES, type Holiday, type Vacation, type VacationType } from "@/types/time";
import s from "./VacationModal.module.scss";

/** Working days between two dates, without weekends and the given holidays (as the server counts). */
function workingDays(start: string, end: string, holidays: Set<string>) {
  if (!start || !end || end < start) return 0;
  let n = 0;
  for (let d = start; d <= end; d = addDays(d, 1)) {
    const wd = new Date(`${d}T00:00:00`).getDay();
    if (wd !== 0 && wd !== 6 && !holidays.has(d)) n++;
  }
  return n;
}

/** A new absence request or an edit; asks to confirm the number of days before sending. */
export function VacationModal({ vacation, onSaved, onClose }: { vacation?: Vacation; onSaved: () => void; onClose: () => void }) {
  const { t } = useTranslation();
  const { user } = useAuth();
  const [type, setType] = useState<VacationType>(vacation?.type ?? "vacation");
  const [start, setStart] = useState(vacation?.start ?? "");
  const [end, setEnd] = useState(vacation?.end ?? "");
  const [comment, setComment] = useState(vacation?.comment ?? "");
  const [reviewerIds, setReviewerIds] = useState<number[]>(vacation?.reviewerIds ?? []);
  const [reviewers, setReviewers] = useState<User[]>([]);
  const [holidays, setHolidays] = useState<Holiday[]>([]);
  const [confirming, setConfirming] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    timeApi.reviewers().then((list) => setReviewers(list.filter((u) => u.id !== (vacation?.userId ?? user!.id)))).catch(() => {});
  }, [vacation, user]);
  const year = Number((start || new Date().toISOString()).slice(0, 4));
  useEffect(() => {
    Promise.all([timeApi.holidays(year), timeApi.holidays(year + 1)])
      .then(([a, b]) => setHolidays([...a, ...b]))
      .catch(() => {});
  }, [year]);
  const days = useMemo(() => workingDays(start, end, new Set(holidays.map((h) => h.day))), [start, end, holidays]);

  function review(e: FormEvent) {
    e.preventDefault();
    if (!reviewerIds.length) return setError(t("errors.vacation_reviewers_required"));
    if (end < start) return setError(t("errors.vacation_invalid"));
    setError("");
    setConfirming(true);
  }

  async function send() {
    setBusy(true);
    try {
      const input = { type, start, end, comment, reviewerIds };
      if (vacation) await timeApi.updateVacation(vacation.id, input);
      else await timeApi.createVacation(input);
      onSaved();
      onClose();
    } catch (err) {
      setError((err as Error).message);
      setConfirming(false);
      setBusy(false);
    }
  }

  return (
    <Modal title={t("vacations.newRequest")} onClose={onClose}>
      <ModalContent onSubmit={confirming ? (e) => (e.preventDefault(), send()) : review}>
        {confirming ? (
          <p className={s.confirm}>{t("vacations.confirm", { type: t(`vacations.types.${type}`), count: days })}</p>
        ) : (
          <>
            <Field label={t("vacations.type")}>
              <select value={type} onChange={(e) => setType(e.target.value as VacationType)}>
                {VACATION_TYPES.map((x) => (
                  <option key={x} value={x}>
                    {t(`vacations.types.${x}`)}
                  </option>
                ))}
              </select>
            </Field>
            <div className={s.dates}>
              <Field label={t("vacations.start")}>
                <input type="date" value={start} onChange={(e) => (setStart(e.target.value), !end && setEnd(e.target.value))} required />
              </Field>
              <Field label={t("vacations.end")}>
                <input type="date" value={end} min={start} onChange={(e) => setEnd(e.target.value)} required />
              </Field>
            </div>
            {days > 0 && <Muted as="p">{t("vacations.days", { count: days })}</Muted>}
            <Field as="div" label={t("vacations.reviewers")}>
              <div className={s.reviewers}>
                {reviewers.map((r) => (
                  <label key={r.id}>
                    <input
                      type="checkbox"
                      checked={reviewerIds.includes(r.id)}
                      onChange={(e) => setReviewerIds((ids) => (e.target.checked ? [...ids, r.id] : ids.filter((x) => x !== r.id)))}
                    />
                    {r.name}
                  </label>
                ))}
              </div>
            </Field>
            <Field label={t("vacations.comment")}>
              <textarea rows={2} value={comment} onChange={(e) => setComment(e.target.value)} maxLength={2000} />
            </Field>
          </>
        )}
        {error && <FormError>{error}</FormError>}
        <ModalActions>
          <Button type="button" onClick={confirming ? () => setConfirming(false) : onClose}>
            {t("common.cancel")}
          </Button>
          <Button variant="primary" disabled={busy}>
            {confirming ? t("common.create") : t("common.save")}
          </Button>
        </ModalActions>
      </ModalContent>
    </Modal>
  );
}
