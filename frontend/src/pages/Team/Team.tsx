import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { AddPersonModal } from "@/components/AddPersonModal/AddPersonModal";
import { Avatar } from "@/components/Avatar/Avatar";
import { DirectoriesModal } from "@/components/DirectoriesModal/DirectoriesModal";
import { Page } from "@/components/Page/Page";
import { Banner } from "@/components/ui/Banner/Banner";
import { Button } from "@/components/ui/Button/Button";
import { Chip } from "@/components/ui/Chip/Chip";
import { Muted } from "@/components/ui/Muted/Muted";
import { Pagination } from "@/components/ui/Pagination/Pagination";
import { Segmented } from "@/components/ui/Segmented/Segmented";
import { SortHeader, Table } from "@/components/ui/Table/Table";
import { useAuth } from "@/context/auth";
import { useDebounced } from "@/hooks/useDebounced";
import { peopleApi } from "@/lib/api/people";
import { can, COMPANY_ROLES } from "@/lib/companyRoles";
import { formatDate } from "@/lib/format";
import type { CompanyRole } from "@/types";
import { ENGLISH_LEVELS, GRADES, type DirectoryItem, type Person, type PersonFilters } from "@/types/people";
import s from "./Team.module.scss";

const PAGE = 20;

const INITIAL: PersonFilters = {
  q: "", quick: "", roles: [], specialization: "", grade: "", englishLevel: "", inactive: false, sort: "name", order: "asc",
};

/** The team directory: search, quick and detailed filters, sortable table. */
export default function Team() {
  const { t, i18n } = useTranslation();
  const { user } = useAuth();
  const [filters, setFilters] = useState(INITIAL);
  const [offset, setOffset] = useState(0);
  const [data, setData] = useState<{ people: Person[]; total: number } | null>(null);
  const [error, setError] = useState("");
  const [specializations, setSpecializations] = useState<DirectoryItem[]>([]);
  const [adding, setAdding] = useState(false);
  const [editingLists, setEditingLists] = useState(false);
  const q = useDebounced(filters.q, 300);

  const load = useCallback(() => {
    peopleApi
      .list({ ...filters, q }, offset, PAGE)
      .then(setData)
      .catch((err) => setError((err as Error).message));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [q, filters.quick, filters.roles, filters.specialization, filters.grade, filters.englishLevel, filters.inactive, filters.sort, filters.order, offset]);

  useEffect(load, [load]);
  useEffect(() => {
    peopleApi.directory("specialization").then(setSpecializations).catch(() => {});
  }, [editingLists]);

  const set = (patch: Partial<PersonFilters>) => {
    setFilters((f) => ({ ...f, ...patch }));
    setOffset(0);
  };
  const sortBy = (id: PersonFilters["sort"]) => set({ sort: id, order: filters.sort === id && filters.order === "asc" ? "desc" : "asc" });
  const showGrade = can(user, "viewGrades");

  return (
    <Page>
      <div className={s.head}>
        <h1>{t("people.team")}</h1>
        <div className={s.headActions}>
          {can(user, "editUser") && <Button onClick={() => setEditingLists(true)}>{t("people.directories")}</Button>}
          {can(user, "addUser") && (
            <Button variant="primary" onClick={() => setAdding(true)}>
              {t("people.addPerson")}
            </Button>
          )}
        </div>
      </div>

      <div className={s.filters}>
        <input className={s.search} placeholder={t("people.searchTeam")} value={filters.q} onChange={(e) => set({ q: e.target.value })} />
        <Segmented
          label={t("people.team")}
          value={filters.quick}
          onChange={(quick) => set({ quick })}
          options={[
            { id: "", label: t("people.quickAll") },
            { id: "billable", label: t("people.quickBillable") },
            { id: "nonbillable", label: t("people.quickNonBillable") },
            { id: "contractors", label: t("people.quickContractors") },
          ]}
        />
      </div>
      <div className={s.filters}>
        <select value={filters.roles[0] ?? ""} onChange={(e) => set({ roles: e.target.value ? [e.target.value as CompanyRole] : [] })}>
          <option value="">{t("people.anyRole")}</option>
          {COMPANY_ROLES.filter((r) => r !== "CANDIDATE").map((r) => (
            <option key={r} value={r}>
              {t(`companyRoles.${r}`)}
            </option>
          ))}
        </select>
        <select value={filters.specialization} onChange={(e) => set({ specialization: e.target.value })}>
          <option value="">{t("people.anySpecialization")}</option>
          {specializations.map((sp) => (
            <option key={sp.id} value={sp.name}>
              {sp.name}
            </option>
          ))}
        </select>
        {showGrade && (
          <select value={filters.grade} onChange={(e) => set({ grade: e.target.value })}>
            <option value="">{t("people.anyGrade")}</option>
            {GRADES.map((g) => (
              <option key={g}>{g}</option>
            ))}
          </select>
        )}
        <select value={filters.englishLevel} onChange={(e) => set({ englishLevel: e.target.value })}>
          <option value="">{t("people.anyEnglish")}</option>
          {ENGLISH_LEVELS.map((l) => (
            <option key={l}>{l}</option>
          ))}
        </select>
        {can(user, "editUser") && (
          <label className={s.check}>
            <input type="checkbox" checked={filters.inactive} onChange={(e) => set({ inactive: e.target.checked })} />
            {t("people.showInactive")}
          </label>
        )}
      </div>

      {error && <Banner onDismiss={() => setError("")}>{error}</Banner>}

      <Table>
        <thead>
          <tr>
            <SortHeader id="name" sort={filters.sort} desc={filters.order === "desc"} onSort={sortBy}>
              {t("people.colName")}
            </SortHeader>
            <SortHeader id="role" sort={filters.sort} desc={filters.order === "desc"} onSort={sortBy}>
              {t("people.colRole")}
            </SortHeader>
            <th>{t("people.colSpecialization")}</th>
            {showGrade && <th>{t("people.colGrade")}</th>}
            <th>{t("people.colStack")}</th>
            <th>{t("people.colEnglish")}</th>
            <th>{t("people.colHours")}</th>
            <th>{t("people.colBillable")}</th>
            <SortHeader id="startDate" sort={filters.sort} desc={filters.order === "desc"} onSort={sortBy}>
              {t("people.colStart")}
            </SortHeader>
          </tr>
        </thead>
        <tbody>
          {data?.people.map((p) => (
            <tr key={p.id} className={p.active ? undefined : s.inactive}>
              <td>
                <Link to={`/people/${p.id}`} className={s.person}>
                  <Avatar user={p} size={30} />
                  <span className={s.who}>
                    <span className={s.name}>{p.name}</span>
                    <small>{p.email}</small>
                  </span>
                </Link>
              </td>
              <td>
                {t(`companyRoles.${p.role}`)}
                {p.additionRoles.includes("LEAD") && (
                  <>
                    {" "}
                    <Chip tone="accent">{t("companyRoles.LEAD")}</Chip>
                  </>
                )}
                {!p.active && (
                  <>
                    {" "}
                    <Chip tone="danger">{p.dismissedAt ? t("people.dismissed") : t("people.inactive")}</Chip>
                  </>
                )}
              </td>
              <td>{p.details.specialization || t("people.notSet")}</td>
              {showGrade && <td>{p.details.grade || t("people.notSet")}</td>}
              <td className={s.stack}>
                {(p.details.techStack ?? []).slice(0, 4).map((tech) => (
                  <Chip key={tech}>{tech}</Chip>
                ))}
              </td>
              <td>{p.details.englishLevel || t("people.notSet")}</td>
              <td>{p.workingHours}</td>
              <td>{p.billable ? t("people.yes") : t("people.no")}</td>
              <td>{p.startDate ? formatDate(p.startDate, i18n.language) : t("people.notSet")}</td>
            </tr>
          ))}
        </tbody>
      </Table>
      {data && data.people.length === 0 && <Muted as="p">{t("people.empty")}</Muted>}
      {data && <Pagination offset={offset} limit={PAGE} total={data.total} onChange={setOffset} />}

      {adding && <AddPersonModal onClose={() => setAdding(false)} onCreated={load} />}
      {editingLists && <DirectoriesModal onClose={() => setEditingLists(false)} />}
    </Page>
  );
}
