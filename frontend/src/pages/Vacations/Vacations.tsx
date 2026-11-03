import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router-dom";
import { Page } from "@/components/Page/Page";
import { VacationList } from "@/components/VacationList/VacationList";
import { VacationModal } from "@/components/VacationModal/VacationModal";
import { Banner } from "@/components/ui/Banner/Banner";
import { Button } from "@/components/ui/Button/Button";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog/ConfirmDialog";
import { Panel } from "@/components/ui/Panel/Panel";
import { Tabs, type Tab } from "@/components/ui/Tabs/Tabs";
import { useAuth } from "@/context/auth";
import { peopleApi } from "@/lib/api/people";
import { timeApi } from "@/lib/api/time";
import { can } from "@/lib/companyRoles";
import type { Person } from "@/types/people";
import { VACATION_TYPES, type Vacation, type VacationStats } from "@/types/time";
import s from "./Vacations.module.scss";

type TabId = "mine" | "all" | "review" | "stats";

/** Absence requests: your own, everyone's (managers, HR), those you review, and days taken / left. */
export default function Vacations() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const [params, setParams] = useSearchParams();
  const seeAll = can(user, "seeVacationsRequests") || user!.role === "HR";
  const [mine, setMine] = useState<Vacation[]>([]);
  const [review, setReview] = useState<Vacation[]>([]);
  const [all, setAll] = useState<Vacation[]>([]);
  const [year, setYear] = useState(new Date().getFullYear());
  const [filter, setFilter] = useState({ month: 0, status: "", type: "", name: "" });
  const [editing, setEditing] = useState<Vacation | null | undefined>(undefined); // null = new
  const [removing, setRemoving] = useState<Vacation | null>(null);
  const [error, setError] = useState("");

  const tabs: Tab<TabId>[] = [{ id: "mine", label: t("vacations.tabMine") }];
  if (seeAll) tabs.push({ id: "all", label: t("vacations.tabAll") });
  tabs.push({ id: "review", label: t("vacations.tabReview"), alert: review.length > 0 }, { id: "stats", label: t("vacations.tabStats") });
  const tab = (tabs.find((x) => x.id === params.get("tab"))?.id ?? "mine") as TabId;

  const load = useCallback(() => {
    timeApi.myVacations(year).then(setMine).catch((err) => setError(err.message));
    timeApi.toReview().then(setReview).catch(() => {});
    if (seeAll) timeApi.allVacations({ year, ...filter }).then(setAll).catch((err) => setError(err.message));
  }, [year, filter, seeAll]);
  useEffect(load, [load]);

  const decide = (v: Vacation, status: "approved" | "declined") =>
    timeApi.decide(v.id, status).then(load, (err) => setError(err.message));

  const years = [year - 1, year, year + 1];
  return (
    <Page>
      <div className={s.head}>
        <h1>{t("vacations.title")}</h1>
        <Button variant="primary" onClick={() => setEditing(null)}>
          {t("vacations.newRequest")}
        </Button>
      </div>
      <Tabs tabs={tabs} value={tab} onChange={(id) => setParams(id === "mine" ? {} : { tab: id }, { replace: true })} />
      {error && <Banner onDismiss={() => setError("")}>{error}</Banner>}

      {(tab === "mine" || tab === "all" || tab === "stats") && (
        <div className={s.filters}>
          <select value={year} onChange={(e) => setYear(Number(e.target.value))} aria-label={t("vacations.year")}>
            {years.map((y) => (
              <option key={y}>{y}</option>
            ))}
          </select>
          {tab === "all" && (
            <>
              <select value={filter.month} onChange={(e) => setFilter({ ...filter, month: Number(e.target.value) })} aria-label={t("vacations.month")}>
                <option value={0}>{t("vacations.anyMonth")}</option>
                {Array.from({ length: 12 }, (_, i) => (
                  <option key={i + 1} value={i + 1}>
                    {new Intl.DateTimeFormat(undefined, { month: "long" }).format(new Date(2026, i, 1))}
                  </option>
                ))}
              </select>
              <select value={filter.status} onChange={(e) => setFilter({ ...filter, status: e.target.value })}>
                <option value="">{t("vacations.anyStatus")}</option>
                {(["pending", "approved", "declined"] as const).map((st) => (
                  <option key={st} value={st}>
                    {t(`vacations.statuses.${st}`)}
                  </option>
                ))}
              </select>
              <select value={filter.type} onChange={(e) => setFilter({ ...filter, type: e.target.value })}>
                <option value="">{t("vacations.anyType")}</option>
                {VACATION_TYPES.map((x) => (
                  <option key={x} value={x}>
                    {t(`vacations.types.${x}`)}
                  </option>
                ))}
              </select>
              <input placeholder={t("vacations.name")} value={filter.name} onChange={(e) => setFilter({ ...filter, name: e.target.value })} />
            </>
          )}
        </div>
      )}

      {tab === "mine" && <VacationList items={mine} onEdit={setEditing} onDelete={setRemoving} />}
      {tab === "all" && <VacationList items={all} showRequester onDecide={user!.role === "OWNER" || user!.role === "ADMIN" ? decide : undefined} onDelete={setRemoving} />}
      {tab === "review" && <VacationList items={review} showRequester onDecide={decide} />}
      {tab === "stats" && <StatsView year={year} />}

      {editing !== undefined && <VacationModal vacation={editing ?? undefined} onSaved={load} onClose={() => setEditing(undefined)} />}
      {removing && (
        <ConfirmDialog
          title={t("vacations.deleteConfirm")}
          confirmLabel={t("vacations.delete")}
          danger
          onClose={() => setRemoving(null)}
          onConfirm={async () => {
            await timeApi.deleteVacation(removing.id);
            load();
          }}
        />
      )}
    </Page>
  );
}

/** Days taken and left per type; people with the permission can look at anyone. */
function StatsView({ year }: { year: number }) {
  const { t } = useTranslation();
  const { user } = useAuth();
  const [userId, setUserId] = useState(user!.id);
  const [people, setPeople] = useState<Person[]>([]);
  const [stats, setStats] = useState<VacationStats | null>(null);
  const pick = can(user, "seeFilterInVacationStatistics");

  useEffect(() => {
    timeApi.stats(year, userId === user!.id ? undefined : userId).then(setStats).catch(() => setStats(null));
  }, [year, userId, user]);
  useEffect(() => {
    if (pick) peopleApi.list({ q: "", quick: "", roles: [], specialization: "", grade: "", englishLevel: "", inactive: false, sort: "name", order: "asc" }, 0, 100)
      .then((r) => setPeople(r.people)).catch(() => {});
  }, [pick]);

  return (
    <>
      {pick && (
        <select className={s.person} value={userId} onChange={(e) => setUserId(Number(e.target.value))}>
          {[user!, ...people.filter((p) => p.id !== user!.id)].map((p) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </select>
      )}
      {stats && (
        <div className={s.stats}>
          {VACATION_TYPES.map((x) => (
            <Panel key={x} compact>
              <h2>{t(`vacations.types.${x}`)}</h2>
              <div className={s.statRow}>
                <span>{t("vacations.taken")}</span>
                <strong>{stats.taken[x]}</strong>
              </div>
              <div className={s.statRow}>
                <span>{t("vacations.left")}</span>
                <strong>
                  {stats.left[x]} / {stats.allowance[x]}
                </strong>
              </div>
            </Panel>
          ))}
        </div>
      )}
    </>
  );
}
