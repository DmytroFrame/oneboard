import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Avatar } from "@/components/Avatar/Avatar";
import { Button } from "@/components/ui/Button/Button";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog/ConfirmDialog";
import { Field } from "@/components/ui/Field/Field";
import { FormError } from "@/components/ui/FormError/FormError";
import { Modal, ModalActions, ModalContent } from "@/components/ui/Modal/Modal";
import { Muted } from "@/components/ui/Muted/Muted";
import { Panel } from "@/components/ui/Panel/Panel";
import { WikiShell } from "@/components/WikiShell/WikiShell";
import { useAuth } from "@/context/auth";
import { peopleApi } from "@/lib/api/people";
import { wikiApi } from "@/lib/api/wiki";
import type { User } from "@/types";
import type { WikiGroup } from "@/types/wiki";
import s from "./WikiGroups.module.scss";

/** Groups share articles with several people at once. */
export default function WikiGroups() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const [groups, setGroups] = useState<WikiGroup[]>([]);
  const [people, setPeople] = useState<User[]>([]);
  const [editing, setEditing] = useState<{ id?: number; name: string; memberIds: number[] } | null>(null);
  const [removing, setRemoving] = useState<{ group: WikiGroup; usage: number } | null>(null);
  const [error, setError] = useState("");
  const manage = (g: WikiGroup) => g.ownerId === user!.id || user!.role === "OWNER" || user!.role === "ADMIN";

  const load = useCallback(() => {
    wikiApi.groups().then(setGroups).catch((err) => setError(err.message));
  }, []);
  useEffect(load, [load]);
  useEffect(() => {
    peopleApi
      .list({ q: "", quick: "", roles: [], specialization: "", grade: "", englishLevel: "", inactive: false, sort: "name", order: "asc" }, 0, 100)
      .then((r) => setPeople(r.people.filter((p) => p.role !== "CONTRACTOR" && p.role !== "CANDIDATE")))
      .catch(() => {});
  }, []);

  async function save() {
    try {
      await wikiApi.saveGroup(editing!);
      setEditing(null);
      setError("");
      load();
    } catch (err) {
      setError((err as Error).message);
    }
  }

  return (
    <WikiShell>
      <div className={s.head}>
        <h1>{t("wiki.groups")}</h1>
        <Button variant="primary" size="sm" onClick={() => setEditing({ name: "", memberIds: [user!.id] })}>
          + {t("wiki.newGroup")}
        </Button>
      </div>
      {error && !editing && <FormError>{error}</FormError>}
      <Panel compact>
        {groups.length === 0 && <Muted as="p" className={s.flush}>{t("wiki.groupsEmpty")}</Muted>}
        {groups.map((g) => (
          <div key={g.id} className={s.row}>
            <span className={s.name}>{g.name}</span>
            <Muted>{t("wiki.people", { count: g.memberIds.length })}</Muted>
            {manage(g) && (
              <span className={s.tools}>
                <Button size="sm" onClick={() => setEditing({ id: g.id, name: g.name, memberIds: g.memberIds })}>
                  {t("common.edit")}
                </Button>
                <Button size="sm" onClick={() => wikiApi.groupUsage(g.id).then((u) => setRemoving({ group: g, usage: u.articles }))}>
                  {t("common.delete")}
                </Button>
              </span>
            )}
          </div>
        ))}
      </Panel>
      {editing && (
        <Modal title={editing.id ? t("wiki.editGroup") : t("wiki.newGroup")} onClose={() => setEditing(null)}>
          <ModalContent>
            <Field label={t("common.name")}>
              <input value={editing.name} maxLength={100} onChange={(e) => setEditing({ ...editing, name: e.target.value })} autoFocus />
            </Field>
            <div className={s.members}>
              {people.map((p) => (
                <label key={p.id} className={s.member}>
                  <Avatar user={p} size={26} />
                  <span>{p.name}</span>
                  <input
                    type="checkbox"
                    role="switch"
                    checked={editing.memberIds.includes(p.id)}
                    onChange={(e) =>
                      setEditing({ ...editing, memberIds: e.target.checked ? [...editing.memberIds, p.id] : editing.memberIds.filter((x) => x !== p.id) })
                    }
                  />
                </label>
              ))}
            </div>
            {error && <FormError>{error}</FormError>}
            <ModalActions>
              <Button onClick={() => setEditing(null)}>{t("common.cancel")}</Button>
              <Button variant="primary" onClick={save}>
                {t("common.save")}
              </Button>
            </ModalActions>
          </ModalContent>
        </Modal>
      )}
      {removing && (
        <ConfirmDialog
          title={t("wiki.deleteGroup", { name: removing.group.name })}
          text={removing.usage ? t("wiki.groupInUse", { count: removing.usage }) : t("wiki.groupUnused")}
          confirmLabel={t("common.delete")}
          danger
          onClose={() => setRemoving(null)}
          onConfirm={async () => {
            await wikiApi.deleteGroup(removing.group.id);
            setRemoving(null);
            load();
          }}
        />
      )}
    </WikiShell>
  );
}
