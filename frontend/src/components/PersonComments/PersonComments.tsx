import { useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { NoteCard } from "@/components/NoteCard/NoteCard";
import { Button } from "@/components/ui/Button/Button";
import { Chip } from "@/components/ui/Chip/Chip";
import { FormError } from "@/components/ui/FormError/FormError";
import { IconButton } from "@/components/ui/IconButton/IconButton";
import { Muted } from "@/components/ui/Muted/Muted";
import { Panel } from "@/components/ui/Panel/Panel";
import { useAuth } from "@/context/auth";
import { peopleApi } from "@/lib/api/people";
import { can } from "@/lib/companyRoles";
import { COMMENT_KINDS, type Person, type PersonComment } from "@/types/people";
import s from "./PersonComments.module.scss";

/** Internal comments about a person (interviews, English check, dismissal…), never shown to them. */
export function PersonComments({ person }: { person: Person }) {
  const { t } = useTranslation();
  const { user } = useAuth();
  const [items, setItems] = useState<PersonComment[] | null>(null);
  const [kind, setKind] = useState("other");
  const [body, setBody] = useState("");
  const [anonymous, setAnonymous] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    peopleApi.comments(person.id).then(setItems).catch((err) => setError(err.message));
  }, [person.id]);

  async function add(e: FormEvent) {
    e.preventDefault();
    setError("");
    try {
      const c = await peopleApi.addComment(person.id, kind, body, anonymous);
      setItems((list) => [c, ...(list ?? [])]);
      setBody("");
    } catch (err) {
      setError((err as Error).message);
    }
  }

  return (
    <div className={s.list}>
      <Panel compact>
        <form className={s.form} onSubmit={add}>
          <textarea rows={3} value={body} onChange={(e) => setBody(e.target.value)} placeholder={t("people.commentPlaceholder")} maxLength={5000} required />
          <div className={s.row}>
            <select value={kind} onChange={(e) => setKind(e.target.value)} aria-label={t("people.commentKind")}>
              {COMMENT_KINDS.map((k) => (
                <option key={k} value={k}>
                  {t(`people.commentKinds.${k}` as "people.commentKinds.other")}
                </option>
              ))}
            </select>
            <label className={s.check}>
              <input type="checkbox" checked={anonymous} onChange={(e) => setAnonymous(e.target.checked)} />
              {t("people.anonymous")}
            </label>
            <Button variant="primary" size="sm">
              {t("people.addComment")}
            </Button>
          </div>
          {error && <FormError>{error}</FormError>}
        </form>
      </Panel>
      {items?.length === 0 && <Muted as="p">{t("people.noComments")}</Muted>}
      {items?.map((c) => (
        <NoteCard
          key={c.id}
          author={c.authorName}
          createdAt={c.createdAt}
          badge={<Chip>{t(`people.commentKinds.${c.kind}` as "people.commentKinds.other")}</Chip>}
          actions={
            (c.authorId === user?.id || can(user, "editUser")) && (
              <IconButton size="sm" aria-label={t("common.delete")} onClick={() => peopleApi.deleteComment(c.id).then(() => setItems((l) => l!.filter((x) => x.id !== c.id)))}>
                ✕
              </IconButton>
            )
          }
        >
          {c.body}
        </NoteCard>
      ))}
    </div>
  );
}
