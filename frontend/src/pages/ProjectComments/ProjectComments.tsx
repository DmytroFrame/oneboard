import { useCallback, useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { ProjectPage } from "@/components/ProjectPage/ProjectPage";
import { Banner } from "@/components/ui/Banner/Banner";
import { Button } from "@/components/ui/Button/Button";
import { Chip } from "@/components/ui/Chip/Chip";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog/ConfirmDialog";
import { Field } from "@/components/ui/Field/Field";
import { FormError } from "@/components/ui/FormError/FormError";
import { Modal, ModalActions, ModalContent } from "@/components/ui/Modal/Modal";
import { Muted } from "@/components/ui/Muted/Muted";
import { Table } from "@/components/ui/Table/Table";
import { useAuth } from "@/context/auth";
import { wikiApi } from "@/lib/api/wiki";
import { can } from "@/lib/companyRoles";
import { formatDate } from "@/lib/format";
import type { Project } from "@/types";
import { COMMENT_CATEGORIES, type CommentCategory, type ProjectComment } from "@/types/wiki";
import s from "./ProjectComments.module.scss";

export default function ProjectCommentsRoute() {
  return <ProjectPage>{(project) => <CommentsView key={project.id} project={project} />}</ProjectPage>;
}

type Form = { id?: number; category: CommentCategory; title: string; body: string };

/** Project notes by category (calls, requirements, design…); owners and admins edit them. */
function CommentsView({ project }: { project: Project }) {
  const { t, i18n } = useTranslation();
  const { user } = useAuth();
  const [category, setCategory] = useState<CommentCategory | "">("");
  const [items, setItems] = useState<ProjectComment[] | null>(null);
  const [form, setForm] = useState<Form | null>(null);
  const [removing, setRemoving] = useState<ProjectComment | null>(null);
  const [error, setError] = useState("");
  const [formError, setFormError] = useState("");
  const manage = can(user, "editProjectComment");

  const load = useCallback(() => {
    wikiApi.comments(project.id, category).then(setItems).catch((err) => setError(err.message));
  }, [project.id, category]);
  useEffect(load, [load]);

  async function submit(e: FormEvent) {
    e.preventDefault();
    try {
      const body = { category: form!.category, title: form!.title, body: form!.body };
      if (form!.id) await wikiApi.updateComment(form!.id, body);
      else await wikiApi.createComment(project.id, body);
      setForm(null);
      load();
    } catch (err) {
      setFormError((err as Error).message);
    }
  }

  return (
    <>
      <div className={s.head}>
        <select value={category} onChange={(e) => setCategory(e.target.value as CommentCategory | "")} aria-label={t("comments.category")}>
          <option value="">{t("comments.allCategories")}</option>
          {COMMENT_CATEGORIES.map((c) => (
            <option key={c} value={c}>
              {t(`comments.categories.${c}`)}
            </option>
          ))}
        </select>
        <Button variant="primary" size="sm" onClick={() => (setFormError(""), setForm({ category: "other", title: "", body: "" }))}>
          + {t("comments.add")}
        </Button>
      </div>
      {error && <Banner>{error}</Banner>}
      {items && items.length === 0 && <Muted as="p">{t("comments.empty")}</Muted>}
      {items && items.length > 0 && (
        <Table>
          <thead>
            <tr>
              <th>{t("comments.titleCol")}</th>
              <th>{t("comments.body")}</th>
              <th>{t("comments.date")}</th>
              {manage && <th />}
            </tr>
          </thead>
          <tbody>
            {items.map((c) => (
              <tr key={c.id}>
                <td className={s.titleCell}>
                  <Chip>{t(`comments.categories.${c.category}`)}</Chip> <strong>{c.title}</strong>
                </td>
                <td className={s.body}>{c.body}</td>
                <td className={s.date}>
                  {formatDate(c.createdAt, i18n.language)}
                  <small>{c.authorName}</small>
                </td>
                {manage && (
                  <td className={s.tools}>
                    <Button size="sm" onClick={() => (setFormError(""), setForm({ id: c.id, category: c.category, title: c.title, body: c.body }))}>
                      {t("common.edit")}
                    </Button>
                    <Button size="sm" onClick={() => setRemoving(c)}>
                      {t("common.delete")}
                    </Button>
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </Table>
      )}
      {form && (
        <Modal title={form.id ? t("comments.edit") : t("comments.add")} onClose={() => setForm(null)}>
          <ModalContent onSubmit={submit}>
            <Field label={t("comments.category")}>
              <select value={form.category} onChange={(e) => setForm({ ...form, category: e.target.value as CommentCategory })}>
                {COMMENT_CATEGORIES.map((c) => (
                  <option key={c} value={c}>
                    {t(`comments.categories.${c}`)}
                  </option>
                ))}
              </select>
            </Field>
            <Field label={t("comments.titleCol")}>
              <input value={form.title} maxLength={200} required onChange={(e) => setForm({ ...form, title: e.target.value })} autoFocus />
            </Field>
            <Field label={t("comments.body")}>
              <textarea rows={6} value={form.body} maxLength={20000} onChange={(e) => setForm({ ...form, body: e.target.value })} />
            </Field>
            {formError && <FormError>{formError}</FormError>}
            <ModalActions>
              <Button type="button" onClick={() => setForm(null)}>
                {t("common.cancel")}
              </Button>
              <Button variant="primary">{form.id ? t("common.save") : t("common.create")}</Button>
            </ModalActions>
          </ModalContent>
        </Modal>
      )}
      {removing && (
        <ConfirmDialog
          title={t("comments.deleteConfirm")}
          confirmLabel={t("common.delete")}
          danger
          onClose={() => setRemoving(null)}
          onConfirm={async () => {
            await wikiApi.deleteComment(removing.id);
            setRemoving(null);
            load();
          }}
        />
      )}
    </>
  );
}
