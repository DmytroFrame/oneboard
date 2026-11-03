import { useEffect, useMemo, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { DescriptionEditor } from "@/components/DescriptionEditor/DescriptionEditor";
import { Page } from "@/components/Page/Page";
import { Button } from "@/components/ui/Button/Button";
import { Field } from "@/components/ui/Field/Field";
import { FormError } from "@/components/ui/FormError/FormError";
import { useAuth } from "@/context/auth";
import { wikiApi } from "@/lib/api/wiki";
import type { WikiDraft, WikiItem } from "@/types/wiki";
import s from "./WikiEditor.module.scss";

/** Creating or editing an article: title, where it sits, and the text. */
export default function WikiEditor() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const navigate = useNavigate();
  const { id } = useParams();
  const [params] = useSearchParams();
  const editing = id ? Number(id) : null;
  const [draft, setDraft] = useState<WikiDraft | null>(null);
  const [items, setItems] = useState<WikiItem[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    wikiApi.list().then(setItems).catch(() => {});
    if (editing) {
      wikiApi
        .get(editing)
        .then((p) => setDraft({ title: p.title, content: p.content, parentId: p.parentId, projectId: p.projectId }))
        .catch((err) => setError(err.message));
    } else {
      const parent = params.get("parent");
      const project = params.get("project");
      setDraft({ title: "", content: "", parentId: parent ? Number(parent) : null, projectId: project ? Number(project) : null });
    }
  }, [editing, params]);

  // Possible parents: articles you can edit, minus this one and everything under it.
  const parents = useMemo(() => {
    const below = new Set<number>(editing ? [editing] : []);
    let grew = true;
    while (grew) {
      grew = false;
      for (const a of items)
        if (a.parentId !== null && below.has(a.parentId) && !below.has(a.id)) {
          below.add(a.id);
          grew = true;
        }
    }
    return items.filter((a) => a.level === "edit" && !a.archived && !below.has(a.id));
  }, [items, editing]);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const page = editing ? await wikiApi.update(editing, draft!) : await wikiApi.create(draft!);
      navigate(`/wiki/${page.id}`);
    } catch (err) {
      setError((err as Error).message);
      setBusy(false);
    }
  }

  if (!draft) return <Page>{error && <FormError>{error}</FormError>}</Page>;
  return (
    <Page width="medium">
      <h1 className={s.title}>{editing ? t("wiki.editTitle") : t("wiki.newArticle")}</h1>
      <form className={s.form} onSubmit={submit}>
        <Field label={t("wiki.articleTitle")}>
          <input value={draft.title} onChange={(e) => setDraft({ ...draft, title: e.target.value })} maxLength={200} required autoFocus />
        </Field>
        <Field label={t("wiki.location")}>
          <select value={draft.parentId ?? 0} onChange={(e) => setDraft({ ...draft, parentId: Number(e.target.value) || null })}>
            <option value={0}>{t("wiki.topLevel")}</option>
            {parents.map((a) => (
              <option key={a.id} value={a.id}>
                {a.title}
              </option>
            ))}
          </select>
        </Field>
        <div className={s.editor}>
          <DescriptionEditor user={{ name: user!.name, color: user!.color }} initialMarkdown={draft.content} onChange={(content) => setDraft((d) => d && { ...d, content })} />
        </div>
        {error && <FormError>{error}</FormError>}
        <div className={s.actions}>
          <Button type="button" onClick={() => navigate(editing ? `/wiki/${editing}` : "/wiki")}>
            {t("common.cancel")}
          </Button>
          <Button variant="primary" disabled={busy}>
            {editing ? t("common.save") : t("common.create")}
          </Button>
        </div>
      </form>
    </Page>
  );
}
