import { useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/Button/Button";
import { FormError } from "@/components/ui/FormError/FormError";
import { Panel } from "@/components/ui/Panel/Panel";
import { peopleApi, type ProfileInput } from "@/lib/api/people";
import { formatDate } from "@/lib/format";
import { EMERGENCY_RELATIONS, ENGLISH_LEVELS, GRADES, type DirectoryItem, type Person } from "@/types/people";
import s from "./ProfileDetails.module.scss";

type Kind = "text" | "textarea" | "date" | "hours" | "bool" | "select" | "stack" | "link";

interface FieldDef {
  path: string; // "name", "birthday", "details.phone", "details.emergency.name"
  label: string; // people.<label>
  kind?: Kind;
  options?: () => string[];
  optionLabel?: (v: string) => string;
  edit: "basics" | "work";
  see?: "contacts" | "grade" | "hr";
}

const get = (o: unknown, path: string): unknown =>
  path.split(".").reduce<unknown>((v, k) => (v && typeof v === "object" ? (v as Record<string, unknown>)[k] : undefined), o);

function set<T>(o: T, path: string, value: unknown): T {
  const [head, ...rest] = path.split(".");
  const copy = { ...(o as Record<string, unknown>) };
  copy[head!] = rest.length ? set(copy[head!] ?? {}, rest.join("."), value) : value;
  return copy as T;
}

const toInput = (p: Person): ProfileInput => ({
  name: p.name, nameCyrillic: p.nameCyrillic, birthday: p.birthday, startDate: p.startDate, trialUntil: p.trialUntil,
  workingHours: p.workingHours, billable: p.billable, details: { ...p.details, emergency: { ...p.details.emergency } }, finance: p.finance,
});

/** The details tab: sections of facts, edited in place by whoever may (the person, HR). */
export function ProfileDetails({ person, onSaved }: { person: Person; onSaved: (p: Person) => void }) {
  const { t, i18n } = useTranslation();
  const [editing, setEditing] = useState(false);
  const [form, setForm] = useState<ProfileInput>(() => toInput(person));
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [specializations, setSpecializations] = useState<DirectoryItem[]>([]);
  const [stacks, setStacks] = useState<DirectoryItem[]>([]);
  const a = person.access;

  useEffect(() => setForm(toInput(person)), [person]);
  useEffect(() => {
    if (!editing) return;
    peopleApi.directory("specialization").then(setSpecializations).catch(() => {});
    peopleApi.directory("tech_stack").then(setStacks).catch(() => {});
  }, [editing]);

  const sections: { title: string; fields: FieldDef[] }[] = [
    {
      title: "sectionWork",
      fields: [
        { path: "details.specialization", label: "specialization", kind: "select", options: () => specializations.map((x) => x.name), edit: "work" },
        { path: "details.grade", label: "grade", kind: "select", options: () => GRADES, edit: "work", see: "grade" },
        { path: "details.techStack", label: "techStack", kind: "stack", edit: "basics" },
        { path: "details.englishLevel", label: "englishLevel", kind: "select", options: () => ENGLISH_LEVELS, edit: "basics" },
        { path: "startDate", label: "startDate", kind: "date", edit: "work" },
        { path: "workingHours", label: "workingHours", kind: "hours", edit: "work" },
        { path: "billable", label: "billable", kind: "bool", edit: "work" },
        { path: "details.companyEmail", label: "companyEmail", edit: "work" },
      ],
    },
    {
      title: "sectionPersonal",
      fields: [
        { path: "name", label: "nameLatin", edit: a.self ? "basics" : "work" },
        { path: "nameCyrillic", label: "nameCyrillic", edit: "basics" },
        { path: "birthday", label: "birthday", kind: "date", edit: "basics", see: "contacts" },
        { path: "details.about", label: "about", kind: "textarea", edit: "basics" },
      ],
    },
    {
      title: "sectionContacts",
      fields: [
        { path: "details.phone", label: "phone", edit: "basics", see: "contacts" },
        { path: "details.telegram", label: "telegram", edit: "basics", see: "contacts" },
        { path: "details.whatsapp", label: "whatsapp", edit: "basics", see: "contacts" },
        { path: "details.country", label: "country", edit: "basics", see: "contacts" },
        { path: "details.city", label: "city", edit: "basics", see: "contacts" },
        { path: "details.postAddress", label: "postAddress", edit: "basics", see: "hr" },
      ],
    },
    {
      title: "sectionLinks",
      fields: [
        { path: "details.github", label: "github", kind: "link", edit: "basics" },
        { path: "details.gitlab", label: "gitlab", kind: "link", edit: "basics" },
        { path: "details.linkedin", label: "linkedin", kind: "link", edit: "basics" },
        { path: "details.upwork", label: "upwork", kind: "link", edit: "basics" },
        { path: "details.cvLink", label: "cvLink", kind: "link", edit: "basics" },
        { path: "details.otherLinks", label: "otherLinks", kind: "textarea", edit: "basics" },
      ],
    },
    {
      title: "sectionEmergency",
      fields: [
        { path: "details.emergency.name", label: "emergencyName", edit: "basics", see: "hr" },
        {
          path: "details.emergency.relation", label: "emergencyRelation", kind: "select", options: () => EMERGENCY_RELATIONS,
          optionLabel: (v) => t(`people.relations.${v}` as "people.relations.other"), edit: "basics", see: "hr",
        },
        { path: "details.emergency.phone", label: "phone", edit: "basics", see: "hr" },
        { path: "details.emergency.telegram", label: "telegram", edit: "basics", see: "hr" },
      ],
    },
    {
      title: "sectionHR",
      fields: [
        { path: "trialUntil", label: "trialUntil", kind: "date", edit: "work", see: "hr" },
        { path: "details.hrNotes", label: "hrNotes", kind: "textarea", edit: "work", see: "hr" },
      ],
    },
  ];

  const visible = (f: FieldDef) =>
    !f.see || (f.see === "contacts" && a.seeContacts) || (f.see === "grade" && a.seeGrade) || (f.see === "hr" && a.seeHR);
  const editable = (f: FieldDef) => (f.edit === "basics" ? a.editBasics : a.editWork);
  const canEdit = a.editBasics || a.editWork;

  async function save(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      onSaved(await peopleApi.update(person.id, form));
      setEditing(false);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  function show(f: FieldDef) {
    const v = get(person, f.path);
    if (v === undefined || v === null || v === "" || (Array.isArray(v) && v.length === 0)) return <span className={s.empty}>{t("people.notSet")}</span>;
    switch (f.kind) {
      case "date":
        return formatDate(String(v), i18n.language);
      case "bool":
        return v ? t("people.yes") : t("people.no");
      case "stack":
        return (v as string[]).join(", ");
      case "link":
        return /^https?:\/\//.test(String(v)) ? (
          <a href={String(v)} target="_blank" rel="noreferrer">
            {String(v)}
          </a>
        ) : (
          String(v)
        );
      case "select":
        return f.optionLabel ? f.optionLabel(String(v)) : String(v);
    }
    return String(v);
  }

  function input(f: FieldDef) {
    const v = get(form, f.path);
    const change = (value: unknown) => setForm((prev) => set(prev, f.path, value));
    const disabled = !editable(f);
    switch (f.kind) {
      case "textarea":
        return <textarea rows={3} value={(v as string) ?? ""} disabled={disabled} onChange={(e) => change(e.target.value)} />;
      case "date":
        return <input type="date" value={(v as string) ?? ""} disabled={disabled} onChange={(e) => change(e.target.value || null)} />;
      case "hours":
        return <input type="number" min={0} max={24} step={1} value={Number(v)} disabled={disabled} onChange={(e) => change(Number(e.target.value))} />;
      case "bool":
        return <input type="checkbox" checked={!!v} disabled={disabled} onChange={(e) => change(e.target.checked)} />;
      case "stack":
        return (
          <>
            <input
              list="tech-stacks"
              value={((v as string[]) ?? []).join(", ")}
              disabled={disabled}
              onChange={(e) => change(e.target.value.split(",").map((x) => x.trim()).filter(Boolean))}
            />
            <datalist id="tech-stacks">
              {stacks.map((x) => (
                <option key={x.id} value={x.name} />
              ))}
            </datalist>
          </>
        );
      case "select": {
        const options = f.options!();
        return (
          <select value={(v as string) ?? ""} disabled={disabled} onChange={(e) => change(e.target.value)}>
            <option value="">{t("people.notSet")}</option>
            {[...new Set([...(v ? [String(v)] : []), ...options])].map((o) => (
              <option key={o} value={o}>
                {f.optionLabel ? f.optionLabel(o) : o}
              </option>
            ))}
          </select>
        );
      }
    }
    return <input value={(v as string) ?? ""} disabled={disabled} onChange={(e) => change(e.target.value)} />;
  }

  return (
    <form className={s.sections} onSubmit={save}>
      {canEdit && (
        <div className={s.toolbar}>
          {editing ? (
            <>
              <Button type="button" onClick={() => (setEditing(false), setForm(toInput(person)), setError(""))}>
                {t("common.cancel")}
              </Button>
              <Button variant="primary" disabled={busy}>
                {t("people.save")}
              </Button>
            </>
          ) : (
            <Button type="button" onClick={() => setEditing(true)}>
              {t("people.edit")}
            </Button>
          )}
        </div>
      )}
      {error && <FormError>{error}</FormError>}
      {sections.map((section) => {
        const fields = section.fields.filter(visible);
        if (!fields.length) return null;
        return (
          <Panel key={section.title} compact>
            <h2>{t(`people.${section.title}` as "people.sectionWork")}</h2>
            <dl className={s.grid}>
              {fields.map((f) => (
                <div key={f.path} className={f.kind === "textarea" ? s.wide : undefined}>
                  <dt>{t(`people.${f.label}` as "people.phone")}</dt>
                  <dd>{editing ? input(f) : show(f)}</dd>
                </div>
              ))}
            </dl>
          </Panel>
        );
      })}
    </form>
  );
}
