import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Avatar } from "@/components/Avatar/Avatar";
import { Button } from "@/components/ui/Button/Button";
import { Field } from "@/components/ui/Field/Field";
import { FormError } from "@/components/ui/FormError/FormError";
import { IconButton } from "@/components/ui/IconButton/IconButton";
import { Modal, ModalActions, ModalContent } from "@/components/ui/Modal/Modal";
import { Muted } from "@/components/ui/Muted/Muted";
import { peopleApi } from "@/lib/api/people";
import { wikiApi } from "@/lib/api/wiki";
import type { User } from "@/types";
import type { WikiAccess, WikiGrant, WikiGroup, WikiLevel, WikiPage } from "@/types/wiki";
import s from "./WikiShareModal.module.scss";

interface Props {
  page: WikiPage;
  onClose: () => void;
  onSaved: (p: WikiPage) => void;
}

/** Who can edit or read an article: people, groups, the whole company, or the parent's list. */
export function WikiShareModal({ page, onClose, onSaved }: Props) {
  const { t } = useTranslation();
  const [access, setAccess] = useState<WikiAccess | null>(null);
  const [people, setPeople] = useState<User[]>([]);
  const [groups, setGroups] = useState<WikiGroup[]>([]);
  const [adding, setAdding] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    wikiApi.access(page.id).then(setAccess).catch((err) => setError(err.message));
    wikiApi.groups().then(setGroups).catch(() => {});
    peopleApi
      .list({ q: "", quick: "", roles: [], specialization: "", grade: "", englishLevel: "", inactive: false, sort: "name", order: "asc" }, 0, 100)
      .then((r) => setPeople(r.people.filter((p) => p.role !== "CONTRACTOR" && p.role !== "CANDIDATE")))
      .catch(() => {});
  }, [page.id]);
  if (!access) return null;

  const key = (g: WikiGrant) => (g.userId ? `u${g.userId}` : `g${g.groupId}`);
  const taken = new Set(access.grants.map(key));
  const options = [
    ...people.filter((p) => p.id !== page.ownerId && !taken.has(`u${p.id}`)).map((p) => ({ value: `u${p.id}`, label: p.name })),
    ...groups.filter((g) => !taken.has(`g${g.id}`)).map((g) => ({ value: `g${g.id}`, label: t("wiki.groupLabel", { name: g.name }) })),
  ];
  const set = (grants: WikiGrant[]) => setAccess({ ...access, grants });
  const add = () => {
    if (!adding) return;
    const id = Number(adding.slice(1));
    set([...access.grants, adding[0] === "u" ? { userId: id, level: "read" } : { groupId: id, level: "read" }]);
    setAdding("");
  };

  async function save() {
    setBusy(true);
    setError("");
    try {
      onSaved(await wikiApi.setAccess(page.id, access!));
      onClose();
    } catch (err) {
      setError((err as Error).message);
      setBusy(false);
    }
  }

  return (
    <Modal title={t("wiki.accessTitle")} onClose={onClose}>
      <ModalContent>
        {page.parentId !== null && (
          <label className={s.check}>
            <input type="checkbox" checked={access.inherit} onChange={(e) => setAccess({ ...access, inherit: e.target.checked })} />
            {t("wiki.useParentAccess")}
          </label>
        )}
        {access.inherit ? (
          <Muted as="p">{t("wiki.inheritHint")}</Muted>
        ) : (
          <>
            <Field label={t("wiki.everyone")}>
              <select value={access.company} onChange={(e) => setAccess({ ...access, company: e.target.value as WikiLevel })}>
                <option value="none">{t("wiki.levels.none")}</option>
                <option value="read">{t("wiki.levels.read")}</option>
                <option value="edit">{t("wiki.levels.edit")}</option>
              </select>
            </Field>
            <div className={s.list}>
              {access.grants.length === 0 && <Muted as="p" className={s.flush}>{t("wiki.onlyYou")}</Muted>}
              {access.grants.map((g, i) => {
                const person = g.userId ? people.find((p) => p.id === g.userId) : undefined;
                const group = g.groupId ? groups.find((x) => x.id === g.groupId) : undefined;
                return (
                  <div key={key(g)} className={s.row}>
                    {person ? <Avatar user={person} size={26} /> : <span className={s.groupIcon}>👥</span>}
                    <span className={s.name}>{person?.name ?? (group ? t("wiki.groupLabel", { name: group.name }) : "…")}</span>
                    <select
                      value={g.level}
                      aria-label={t("wiki.level")}
                      onChange={(e) => set(access.grants.map((x, j) => (j === i ? { ...x, level: e.target.value as WikiGrant["level"] } : x)))}
                    >
                      <option value="read">{t("wiki.levels.read")}</option>
                      <option value="edit">{t("wiki.levels.edit")}</option>
                    </select>
                    <IconButton size="sm" aria-label={t("common.remove")} onClick={() => set(access.grants.filter((_, j) => j !== i))}>
                      ×
                    </IconButton>
                  </div>
                );
              })}
            </div>
            <div className={s.add}>
              <select value={adding} onChange={(e) => setAdding(e.target.value)} aria-label={t("wiki.addPeople")}>
                <option value="">{t("wiki.addPeople")}</option>
                {options.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </select>
              <Button size="sm" onClick={add} disabled={!adding}>
                {t("common.add")}
              </Button>
            </div>
          </>
        )}
        {error && <FormError>{error}</FormError>}
        <ModalActions>
          <Button onClick={onClose}>{t("common.cancel")}</Button>
          <Button variant="primary" onClick={save} disabled={busy}>
            {t("common.save")}
          </Button>
        </ModalActions>
      </ModalContent>
    </Modal>
  );
}
