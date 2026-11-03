import { useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/Button/Button";
import { FormError } from "@/components/ui/FormError/FormError";
import { IconButton } from "@/components/ui/IconButton/IconButton";
import { tasksApi } from "@/lib/api/tasks";
import type { Tag } from "@/types";
import s from "./TagManager.module.scss";

const COLORS = ["#ff3b30", "#ff9500", "#ffcc00", "#34c759", "#00c7be", "#007aff", "#5856d6", "#af52de", "#ff2d55", "#8e8e93"];

/** A project's tags: create, rename, recolour, delete (deleting removes it from every task). */
export function TagManager({ projectId }: { projectId: number }) {
  const { t } = useTranslation();
  const [tags, setTags] = useState<Tag[]>([]);
  const [name, setName] = useState("");
  const [color, setColor] = useState(COLORS[5]!);
  const [error, setError] = useState("");

  useEffect(() => {
    tasksApi.tags(projectId).then(setTags).catch((err) => setError(err.message));
  }, [projectId]);

  async function run(action: () => Promise<void>) {
    setError("");
    try {
      await action();
    } catch (err) {
      setError((err as Error).message);
    }
  }

  const add = (e: FormEvent) => {
    e.preventDefault();
    run(async () => {
      const tag = await tasksApi.createTag(projectId, name, color);
      setTags((l) => [...l, tag].sort((a, b) => a.name.localeCompare(b.name)));
      setName("");
    });
  };

  const update = (tag: Tag, patch: Partial<Tag>) =>
    run(async () => {
      const saved = await tasksApi.updateTag(tag.id, patch.name ?? tag.name, patch.color ?? tag.color);
      setTags((l) => l.map((x) => (x.id === saved.id ? saved : x)));
    });

  return (
    <div className={s.manager}>
      {tags.map((tag) => (
        <div key={tag.id} className={s.row}>
          <input type="color" value={tag.color} aria-label={t("tasks.color")} onChange={(e) => update(tag, { color: e.target.value })} />
          <input defaultValue={tag.name} maxLength={50} aria-label={t("tasks.tagName")} onBlur={(e) => e.target.value !== tag.name && update(tag, { name: e.target.value })} />
          <IconButton size="sm" aria-label={t("common.delete")} onClick={() => run(async () => (await tasksApi.deleteTag(tag.id), setTags((l) => l.filter((x) => x.id !== tag.id))))}>
            ✕
          </IconButton>
        </div>
      ))}
      <form className={s.row} onSubmit={add}>
        <input type="color" value={color} aria-label={t("tasks.color")} onChange={(e) => setColor(e.target.value)} />
        <input value={name} onChange={(e) => setName(e.target.value)} placeholder={t("tasks.newTag")} maxLength={50} required />
        <Button size="sm">{t("common.add")}</Button>
      </form>
      {error && <FormError>{error}</FormError>}
    </div>
  );
}
