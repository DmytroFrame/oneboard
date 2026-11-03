import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { Avatar } from "@/components/Avatar/Avatar";
import { HRChart } from "@/components/HRChart/HRChart";
import { Page } from "@/components/Page/Page";
import { Banner } from "@/components/ui/Banner/Banner";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog/ConfirmDialog";
import { IconButton } from "@/components/ui/IconButton/IconButton";
import { Muted } from "@/components/ui/Muted/Muted";
import { Panel } from "@/components/ui/Panel/Panel";
import { useAuth } from "@/context/auth";
import { hrApi } from "@/lib/api/hr";
import { can } from "@/lib/companyRoles";
import { formatDate } from "@/lib/format";
import type { HREvent, HRStats } from "@/types/hr";
import s from "./HRStatistics.module.scss";

/** Hires and dismissals by month, candidates closed, and who was hired or dismissed. */
export default function HRStatistics() {
  const { t, i18n } = useTranslation();
  const { user } = useAuth();
  const thisYear = new Date().getFullYear();
  const [year, setYear] = useState(thisYear);
  const [from, setFrom] = useState(1);
  const [to, setTo] = useState(12);
  const [stats, setStats] = useState<HRStats | null>(null);
  const [removing, setRemoving] = useState<HREvent | null>(null);
  const [error, setError] = useState("");
  const canDelete = can(user, "deleteItemsFromStatistics");

  const load = useCallback(() => {
    hrApi
      .stats(year, from, to)
      .then((r) => (setStats(r), setError("")))
      .catch((err) => setError(err.message));
  }, [year, from, to]);
  useEffect(load, [load]);

  const month = (m: number) => new Intl.DateTimeFormat(i18n.language, { month: "short" }).format(new Date(2000, m - 1, 1));
  const list = (title: string, items: HREvent[], dateLabel: string) => (
    <Panel compact>
      <h2>
        {title} <span className={s.count}>{items.length}</span>
      </h2>
      {items.length === 0 && <Muted as="p" className={s.flush}>{t("hrStats.none")}</Muted>}
      <div className={s.rows}>
        {items.map((e) => (
          <div key={e.id} className={s.row}>
            <Link to={`/people/${e.user.id}`} className={s.person}>
              <Avatar user={e.user} size={30} />
              <span className={s.who}>
                <strong>{e.user.name}</strong>
                <small>
                  {dateLabel} {formatDate(e.day, i18n.language)}
                  {e.reason && ` · ${e.reason}`}
                </small>
              </span>
            </Link>
            {canDelete && (
              <IconButton size="sm" aria-label={t("hrStats.remove")} onClick={() => setRemoving(e)}>
                ×
              </IconButton>
            )}
          </div>
        ))}
      </div>
    </Panel>
  );

  return (
    <Page>
      <div className={s.head}>
        <h1 className={s.title}>{t("hrStats.title")}</h1>
        <div className={s.controls}>
          <label>
            {t("hrStats.year")}
            <select value={year} onChange={(e) => setYear(Number(e.target.value))}>
              {Array.from({ length: 6 }, (_, i) => thisYear - i).map((y) => (
                <option key={y} value={y}>
                  {y}
                </option>
              ))}
            </select>
          </label>
          <label>
            {t("hrStats.fromMonth")}
            <select value={from} onChange={(e) => setFrom(Number(e.target.value))}>
              {Array.from({ length: to }, (_, i) => i + 1).map((m) => (
                <option key={m} value={m}>
                  {month(m)}
                </option>
              ))}
            </select>
          </label>
          <label>
            {t("hrStats.toMonth")}
            <select value={to} onChange={(e) => setTo(Number(e.target.value))}>
              {Array.from({ length: 13 - from }, (_, i) => i + from).map((m) => (
                <option key={m} value={m}>
                  {month(m)}
                </option>
              ))}
            </select>
          </label>
        </div>
      </div>
      {error && <Banner>{error}</Banner>}
      {stats && (
        <>
          <Panel compact>
            <h2>{t("hrStats.chartTitle")}</h2>
            <HRChart months={stats.months} year={year} />
            <div className={s.closed}>
              <span>
                {t("hrStats.closedSuccess")}: <strong>{stats.closedSuccess}</strong>
              </span>
              <span>
                {t("hrStats.closedFail")}: <strong>{stats.closedFail}</strong>
              </span>
            </div>
          </Panel>
          <div className={s.lists}>
            {list(t("hrStats.hiredList"), stats.hired, t("hrStats.hiredOn"))}
            {list(t("hrStats.dismissedList"), stats.dismissed, t("hrStats.dismissedOn"))}
          </div>
        </>
      )}
      {removing && (
        <ConfirmDialog
          title={t("hrStats.removeConfirm", { name: removing.user.name })}
          confirmLabel={t("common.delete")}
          danger
          onClose={() => setRemoving(null)}
          onConfirm={async () => {
            await hrApi.deleteEvent(removing.id);
            setRemoving(null);
            load();
          }}
        />
      )}
    </Page>
  );
}
