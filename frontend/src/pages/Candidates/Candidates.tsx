import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { AddPersonModal } from "@/components/AddPersonModal/AddPersonModal";
import { Avatar } from "@/components/Avatar/Avatar";
import { CandidateModal } from "@/components/CandidateModal/CandidateModal";
import { Page } from "@/components/Page/Page";
import { Banner } from "@/components/ui/Banner/Banner";
import { Button } from "@/components/ui/Button/Button";
import { Chip } from "@/components/ui/Chip/Chip";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog/ConfirmDialog";
import { Field } from "@/components/ui/Field/Field";
import { Muted } from "@/components/ui/Muted/Muted";
import { Pagination } from "@/components/ui/Pagination/Pagination";
import { Table } from "@/components/ui/Table/Table";
import { useAuth } from "@/context/auth";
import { hrApi } from "@/lib/api/hr";
import { peopleApi } from "@/lib/api/people";
import { can } from "@/lib/companyRoles";
import type { CompanyRole, User } from "@/types";
import { CANDIDATE_ATTRIBUTES, CANDIDATE_STATUSES, REJECTION_REASONS, type Candidate, type CandidateFilters, type CandidateStatus, type RejectionReason } from "@/types/hr";
import type { DirectoryItem } from "@/types/people";
import s from "./Candidates.module.scss";

const PAGE = 30;
const EMPTY: CandidateFilters = { q: "", status: "", attribute: "", specialization: "", recruiter: 0 };

/** The hiring funnel: candidates with their status, labels, expectations and recruiter. */
export default function Candidates() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const [filters, setFilters] = useState<CandidateFilters>(EMPTY);
  const [offset, setOffset] = useState(0);
  const [data, setData] = useState<{ candidates: Candidate[]; total: number } | null>(null);
  const [channels, setChannels] = useState<DirectoryItem[]>([]);
  const [specializations, setSpecializations] = useState<DirectoryItem[]>([]);
  const [recruiters, setRecruiters] = useState<User[]>([]);
  const [adding, setAdding] = useState(false);
  const [editing, setEditing] = useState<Candidate | null>(null);
  const [rejecting, setRejecting] = useState<{ c: Candidate; reason: RejectionReason } | null>(null);
  const [hiring, setHiring] = useState<{ c: Candidate; role: CompanyRole } | null>(null);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  const allowed = can(user, "viewCandidates");
  const edit = can(user, "editCandidate");
  const money = can(user, "viewSalary");
  const roles = user?.assignableRoles ?? [];
  const canHire = can(user, "addUser") && roles.length > 0;

  const load = useCallback(() => {
    if (!allowed) return;
    hrApi
      .candidates(filters, offset, PAGE)
      .then((d) => (setData(d), setError("")))
      .catch((err) => setError(err.message));
  }, [filters, offset, allowed]);
  useEffect(load, [load]);
  useEffect(() => {
    if (!allowed) return;
    peopleApi.directory("candidate_channel").then(setChannels).catch(() => {});
    peopleApi.directory("specialization").then(setSpecializations).catch(() => {});
    peopleApi
      .list({ q: "", quick: "", roles: ["RECRUITER"], specialization: "", grade: "", englishLevel: "", inactive: false, sort: "name", order: "asc" }, 0, 100)
      .then((r) => setRecruiters(r.people))
      .catch(() => {});
  }, [allowed]);

  const filter = (patch: Partial<CandidateFilters>) => (setFilters((f) => ({ ...f, ...patch })), setOffset(0));
  const replace = (c: Candidate) => setData((d) => d && { ...d, candidates: d.candidates.map((x) => (x.user.id === c.user.id ? c : x)) });
  const range = (min: number | null, max: number | null) => (min === null && max === null ? "" : `${min ?? "…"}–${max ?? "…"}`);

  if (!allowed) {
    return (
      <Page>
        <h1 className={s.title}>{t("candidates.title")}</h1>
        <Muted as="p">{t("candidates.noAccess")}</Muted>
      </Page>
    );
  }

  return (
    <Page>
      <div className={s.head}>
        <h1 className={s.title}>{t("candidates.title")}</h1>
        {can(user, "addUser") && (
          <Button variant="primary" onClick={() => setAdding(true)}>
            + {t("candidates.add")}
          </Button>
        )}
      </div>
      <div className={s.filters}>
        <input className={s.search} type="search" value={filters.q} placeholder={t("candidates.search")} onChange={(e) => filter({ q: e.target.value })} />
        <select value={filters.status} onChange={(e) => filter({ status: e.target.value as CandidateFilters["status"] })} aria-label={t("candidates.status")}>
          <option value="">{t("candidates.allStatuses")}</option>
          {CANDIDATE_STATUSES.map((st) => (
            <option key={st} value={st}>
              {t(`candidates.statuses.${st}`)}
            </option>
          ))}
        </select>
        <select value={filters.attribute} onChange={(e) => filter({ attribute: e.target.value as CandidateFilters["attribute"] })} aria-label={t("candidates.attributes")}>
          <option value="">{t("candidates.allAttributes")}</option>
          {CANDIDATE_ATTRIBUTES.map((a) => (
            <option key={a} value={a}>
              {t(`candidates.attributeNames.${a}`)}
            </option>
          ))}
        </select>
        <select value={filters.specialization} onChange={(e) => filter({ specialization: e.target.value })} aria-label={t("people.specialization")}>
          <option value="">{t("candidates.allSpecializations")}</option>
          {specializations.map((sp) => (
            <option key={sp.id} value={sp.name}>
              {sp.name}
            </option>
          ))}
        </select>
        <select value={filters.recruiter} onChange={(e) => filter({ recruiter: Number(e.target.value) })} aria-label={t("candidates.recruiter")}>
          <option value={0}>{t("candidates.allRecruiters")}</option>
          {recruiters.map((r) => (
            <option key={r.id} value={r.id}>
              {r.name}
            </option>
          ))}
        </select>
        {JSON.stringify(filters) !== JSON.stringify(EMPTY) && (
          <Button size="sm" onClick={() => filter(EMPTY)}>
            {t("candidates.reset")}
          </Button>
        )}
      </div>
      {notice && <Banner tone="success">{notice}</Banner>}
      {error && <Banner>{error}</Banner>}
      {data && data.candidates.length === 0 && <Muted as="p">{JSON.stringify(filters) === JSON.stringify(EMPTY) ? t("candidates.empty") : t("candidates.notFound")}</Muted>}
      {data && data.candidates.length > 0 && (
        <Table>
          <thead>
            <tr>
              <th>{t("candidates.candidate")}</th>
              <th>{t("candidates.status")}</th>
              <th>{t("candidates.attributes")}</th>
              <th>{t("candidates.channel")}</th>
              {money && <th>{t("candidates.salary")}</th>}
              <th>{t("candidates.recruiter")}</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {data.candidates.map((c) => (
              <tr key={c.user.id}>
                <td>
                  <Link to={`/people/${c.user.id}`} className={s.person}>
                    <Avatar user={c.user} size={30} />
                    <span className={s.who}>
                      <strong>{c.user.name}</strong>
                      <small>{[c.specialization, c.grade].filter(Boolean).join(" · ")}</small>
                    </span>
                  </Link>
                </td>
                <td>
                  {edit ? (
                    <select
                      className={s.status}
                      value={c.status}
                      aria-label={t("candidates.status")}
                      onChange={(e) => hrApi.setStatus(c.user.id, e.target.value as CandidateStatus).then(replace).catch((err) => setError(err.message))}
                    >
                      {CANDIDATE_STATUSES.map((st) => (
                        <option key={st} value={st}>
                          {t(`candidates.statuses.${st}`)}
                        </option>
                      ))}
                    </select>
                  ) : (
                    t(`candidates.statuses.${c.status}`)
                  )}
                  {c.rejectionReason && (
                    <div className={s.rejected}>
                      <Chip tone="danger">{t(`candidates.reasons.${c.rejectionReason}`)}</Chip>
                    </div>
                  )}
                </td>
                <td>
                  <div className={s.chips}>
                    {c.attributes.map((a) => (
                      <Chip key={a} tone={a === "promising" ? "accent" : "neutral"}>
                        {a === "promising" ? "★ " : ""}
                        {t(`candidates.attributeNames.${a}`)}
                      </Chip>
                    ))}
                  </div>
                </td>
                <td>{c.channel}</td>
                {money && (
                  <td className={s.money}>
                    {range(c.minSalary, c.maxSalary) && <span>{range(c.minSalary, c.maxSalary)} $</span>}
                    {range(c.minRate, c.maxRate) && <small>{t("candidates.rateShort", { range: range(c.minRate, c.maxRate) })}</small>}
                  </td>
                )}
                <td>{c.recruiter && <Avatar user={c.recruiter} size={24} title={c.recruiter.name} />}</td>
                <td>
                  <div className={s.actions}>
                    {edit && (
                      <Button size="sm" onClick={() => setEditing(c)}>
                        {t("common.edit")}
                      </Button>
                    )}
                    {edit &&
                      (c.rejectionReason ? (
                        <Button size="sm" onClick={() => hrApi.reject(c.user.id, "").then(replace).catch((err) => setError(err.message))}>
                          {t("candidates.reopen")}
                        </Button>
                      ) : (
                        <Button size="sm" onClick={() => setRejecting({ c, reason: REJECTION_REASONS[0] })}>
                          {t("candidates.reject")}
                        </Button>
                      ))}
                    {canHire && !c.rejectionReason && (
                      <Button size="sm" variant="primary" onClick={() => setHiring({ c, role: roles.includes("EMPLOYEE") ? "EMPLOYEE" : roles[0]! })}>
                        {t("candidates.hire")}
                      </Button>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </Table>
      )}
      {data && <Pagination offset={offset} limit={PAGE} total={data.total} onChange={setOffset} />}

      {adding && <AddPersonModal role="CANDIDATE" onClose={() => setAdding(false)} onCreated={load} />}
      {editing && <CandidateModal candidate={editing} channels={channels} recruiters={recruiters} onClose={() => setEditing(null)} onSaved={replace} />}
      {rejecting && (
        <ConfirmDialog
          title={t("candidates.rejectTitle", { name: rejecting.c.user.name })}
          confirmLabel={t("candidates.reject")}
          danger
          onClose={() => setRejecting(null)}
          onConfirm={async () => {
            replace(await hrApi.reject(rejecting.c.user.id, rejecting.reason));
            setRejecting(null);
          }}
        >
          <Field label={t("candidates.rejectionReason")}>
            <select value={rejecting.reason} onChange={(e) => setRejecting({ ...rejecting, reason: e.target.value as RejectionReason })}>
              {REJECTION_REASONS.map((r) => (
                <option key={r} value={r}>
                  {t(`candidates.reasons.${r}`)}
                </option>
              ))}
            </select>
          </Field>
        </ConfirmDialog>
      )}
      {hiring && (
        <ConfirmDialog
          title={t("candidates.hireTitle", { name: hiring.c.user.name })}
          confirmLabel={t("candidates.hire")}
          onClose={() => setHiring(null)}
          onConfirm={async () => {
            await hrApi.hire(hiring.c.user.id, hiring.role);
            setHiring(null);
            setNotice(t("candidates.hired", { name: hiring.c.user.name }));
            load();
          }}
        >
          <Field label={t("candidates.chooseRole")}>
            <select value={hiring.role} onChange={(e) => setHiring({ ...hiring, role: e.target.value as CompanyRole })}>
              {roles.map((r) => (
                <option key={r} value={r}>
                  {t(`companyRoles.${r}`)}
                </option>
              ))}
            </select>
          </Field>
        </ConfirmDialog>
      )}
    </Page>
  );
}
