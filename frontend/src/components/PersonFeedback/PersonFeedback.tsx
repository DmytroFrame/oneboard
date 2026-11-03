import { useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { NoteCard } from "@/components/NoteCard/NoteCard";
import { Button } from "@/components/ui/Button/Button";
import { Field } from "@/components/ui/Field/Field";
import { FormError } from "@/components/ui/FormError/FormError";
import { Muted } from "@/components/ui/Muted/Muted";
import { Panel } from "@/components/ui/Panel/Panel";
import { peopleApi } from "@/lib/api/people";
import type { Feedback, Person } from "@/types/people";
import s from "./PersonFeedback.module.scss";

const EMPTY = { startDoing: "", stopDoing: "", keepDoing: "", anonymous: false };

/** 360 feedback: start / stop / keep doing. The person reads it; colleagues write it. */
export function PersonFeedback({ person }: { person: Person }) {
  const { t } = useTranslation();
  const [items, setItems] = useState<Feedback[] | null>(null);
  const [form, setForm] = useState(EMPTY);
  const [error, setError] = useState("");

  useEffect(() => {
    peopleApi.feedback(person.id).then(setItems).catch((err) => setError(err.message));
  }, [person.id]);

  async function add(e: FormEvent) {
    e.preventDefault();
    setError("");
    try {
      const f = await peopleApi.addFeedback(person.id, form);
      setItems((list) => [f, ...(list ?? [])]);
      setForm(EMPTY);
    } catch (err) {
      setError((err as Error).message);
    }
  }

  const parts = (f: Pick<Feedback, "startDoing" | "stopDoing" | "keepDoing">) =>
    (["startDoing", "stopDoing", "keepDoing"] as const).filter((k) => f[k]).map((k) => (
      <p key={k} className={s.part}>
        <strong>{t(`people.${k}`)}:</strong> {f[k]}
      </p>
    ));

  return (
    <div className={s.list}>
      {!person.access.self && (
        <Panel compact>
          <form className={s.form} onSubmit={add}>
            {(["startDoing", "stopDoing", "keepDoing"] as const).map((k) => (
              <Field key={k} label={t(`people.${k}`)}>
                <textarea rows={2} value={form[k]} onChange={(e) => setForm({ ...form, [k]: e.target.value })} maxLength={2000} />
              </Field>
            ))}
            <div className={s.row}>
              <label className={s.check}>
                <input type="checkbox" checked={form.anonymous} onChange={(e) => setForm({ ...form, anonymous: e.target.checked })} />
                {t("people.anonymous")}
              </label>
              <Button variant="primary" size="sm">
                {t("people.addFeedback")}
              </Button>
            </div>
            {error && <FormError>{error}</FormError>}
          </form>
        </Panel>
      )}
      {items?.length === 0 && <Muted as="p">{t("people.noFeedback")}</Muted>}
      {items?.map((f) => (
        <NoteCard key={f.id} author={f.authorName} createdAt={f.createdAt}>
          {parts(f)}
        </NoteCard>
      ))}
    </div>
  );
}
