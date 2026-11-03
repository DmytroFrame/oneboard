import { useEffect, useRef, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/Button/Button";
import { Chip } from "@/components/ui/Chip/Chip";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog/ConfirmDialog";
import { Field } from "@/components/ui/Field/Field";
import { FormError } from "@/components/ui/FormError/FormError";
import { Muted } from "@/components/ui/Muted/Muted";
import { Panel } from "@/components/ui/Panel/Panel";
import { Table } from "@/components/ui/Table/Table";
import { useAuth } from "@/context/auth";
import { peopleApi } from "@/lib/api/people";
import { can } from "@/lib/companyRoles";
import { formatSize } from "@/lib/format";
import { DOCUMENT_TYPES, type Person, type PersonDocument } from "@/types/people";
import s from "./PersonDocuments.module.scss";

/** A person's documents (files up to 20 MB) and links; private ones only for HR and the person. */
export function PersonDocuments({ person }: { person: Person }) {
  const { t, i18n } = useTranslation();
  const { user } = useAuth();
  const [items, setItems] = useState<PersonDocument[] | null>(null);
  const [adding, setAdding] = useState(false);
  const [removing, setRemoving] = useState<PersonDocument | null>(null);
  const [error, setError] = useState("");
  const canAdd = (person.access.self && can(user, "addOwnDocs")) || can(user, "addDocs");

  useEffect(() => {
    peopleApi.documents(person.id).then(setItems).catch((err) => setError(err.message));
  }, [person.id]);

  const mayDelete = (d: PersonDocument) => d.uploaderId === user?.id || can(user, "deleteDocs");

  return (
    <div className={s.wrap}>
      {canAdd && (
        <div className={s.toolbar}>
          <Button variant="primary" onClick={() => setAdding((v) => !v)}>
            {t("people.addDocument")}
          </Button>
        </div>
      )}
      {adding && (
        <AddDocumentForm
          personId={person.id}
          onAdded={(d) => {
            setItems((list) => [d, ...(list ?? [])]);
            setAdding(false);
          }}
        />
      )}
      {error && <FormError>{error}</FormError>}
      {items?.length === 0 && <Muted as="p">{t("people.noDocuments")}</Muted>}
      {!!items?.length && (
        <Table>
          <thead>
            <tr>
              <th>{t("people.docName")}</th>
              <th>{t("people.docType")}</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {items.map((d) => (
              <tr key={d.id}>
                <td>
                  <div className={s.name}>
                    {d.name} {d.private && <Chip tone="warning">{t("people.docPrivate")}</Chip>}
                  </div>
                  {d.description && <small className={s.desc}>{d.description}</small>}
                </td>
                <td>{t(`people.docTypes.${d.type}` as "people.docTypes.other")}</td>
                <td className={s.actions}>
                  {d.link ? (
                    <a href={d.link} target="_blank" rel="noreferrer">
                      {t("people.open")}
                    </a>
                  ) : (
                    <Button size="sm" variant="link" onClick={() => peopleApi.downloadDocument(d).catch((err) => setError(err.message))}>
                      {t("people.download")} ({formatSize(d.size, i18n.language)})
                    </Button>
                  )}
                  {mayDelete(d) && (
                    <Button size="sm" variant="danger" onClick={() => setRemoving(d)}>
                      {t("common.delete")}
                    </Button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </Table>
      )}
      {removing && (
        <ConfirmDialog
          title={t("people.deleteDocument", { name: removing.name })}
          confirmLabel={t("common.delete")}
          danger
          onClose={() => setRemoving(null)}
          onConfirm={async () => {
            await peopleApi.deleteDocument(removing.id);
            setItems((list) => list!.filter((x) => x.id !== removing.id));
          }}
        />
      )}
    </div>
  );
}

function AddDocumentForm({ personId, onAdded }: { personId: number; onAdded: (d: PersonDocument) => void }) {
  const { t } = useTranslation();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [type, setType] = useState("other");
  const [link, setLink] = useState("");
  const [isPrivate, setPrivate] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);

  async function submit(e: FormEvent) {
    e.preventDefault();
    const form = new FormData();
    form.append("name", name);
    form.append("description", description);
    form.append("type", type);
    form.append("link", link);
    if (isPrivate) form.append("private", "1");
    const file = fileRef.current?.files?.[0];
    if (file) form.append("file", file, file.name);
    setBusy(true);
    setError("");
    try {
      onAdded(await peopleApi.addDocument(personId, form));
    } catch (err) {
      setError((err as Error).message);
      setBusy(false);
    }
  }

  return (
    <Panel compact>
      <form className={s.form} onSubmit={submit}>
        <div className={s.grid}>
          <Field label={t("people.docName")}>
            <input value={name} onChange={(e) => setName(e.target.value)} maxLength={200} required />
          </Field>
          <Field label={t("people.docType")}>
            <select value={type} onChange={(e) => setType(e.target.value)}>
              {DOCUMENT_TYPES.map((d) => (
                <option key={d} value={d}>
                  {t(`people.docTypes.${d}` as "people.docTypes.other")}
                </option>
              ))}
            </select>
          </Field>
          <Field label={t("people.docFile")}>
            <input ref={fileRef} type="file" />
          </Field>
          <Field label={t("people.docLink")}>
            <input type="url" value={link} onChange={(e) => setLink(e.target.value)} placeholder="https://" />
          </Field>
        </div>
        <Field label={t("people.docDescription")}>
          <textarea rows={2} value={description} onChange={(e) => setDescription(e.target.value)} maxLength={2000} />
        </Field>
        <label className={s.check}>
          <input type="checkbox" checked={isPrivate} onChange={(e) => setPrivate(e.target.checked)} />
          {t("people.docPrivate")}
        </label>
        {error && <FormError>{error}</FormError>}
        <div>
          <Button variant="primary" disabled={busy}>
            {t("common.add")}
          </Button>
        </div>
      </form>
    </Panel>
  );
}
