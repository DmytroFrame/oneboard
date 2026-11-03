import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { Avatar } from "@/components/Avatar/Avatar";
import { Muted } from "@/components/ui/Muted/Muted";
import { Panel } from "@/components/ui/Panel/Panel";
import { api } from "@/lib/api";
import { timeApi } from "@/lib/api/time";
import { formatDate } from "@/lib/format";
import type { CompanyWidgets as Widgets, PersonDate, User } from "@/types";
import type { Vacation } from "@/types/time";
import s from "./CompanyWidgets.module.scss";

interface Row {
  key: string;
  user: User;
  note: string;
}

/** The company at a glance: headcount, upcoming birthdays and anniversaries, trials, who is away. */
export function CompanyWidgets() {
  const { t, i18n } = useTranslation();
  const [data, setData] = useState<Widgets | null>(null);
  const [away, setAway] = useState<Vacation[]>([]);

  useEffect(() => {
    api.company().then(setData).catch(() => {});
    timeApi.thisWeek().then(setAway).catch(() => {});
  }, []);
  if (!data) return null;

  const day = (d: string) => formatDate(d, i18n.language, { day: "numeric", month: "long" });
  const dates = (list: PersonDate[], note: (pd: PersonDate) => string): Row[] =>
    list.map((pd) => ({ key: `${pd.user.id}-${pd.date}`, user: pd.user, note: note(pd) }));
  const c = data.counters;
  const tiles = [
    { label: t("company.employees"), value: c.employees },
    { label: t("company.contractors"), value: c.contractors },
    { label: t("company.myProjects"), value: c.myProjects },
    ...(c.candidates !== null ? [{ label: t("company.candidates"), value: c.candidates }] : []),
  ];
  const lists: { title: string; empty: string; rows: Row[] }[] = [
    ...(data.birthdays
      ? [{ title: t("company.birthdays"), empty: t("company.birthdaysEmpty"), rows: dates(data.birthdays, (pd) => day(pd.date)) }]
      : []),
    {
      title: t("company.anniversaries"),
      empty: t("company.anniversariesEmpty"),
      rows: dates(data.anniversaries, (pd) => `${day(pd.date)} · ${t("company.years", { count: pd.years ?? 0 })}`),
    },
    ...(data.onTrial
      ? [{ title: t("company.onTrial"), empty: t("company.onTrialEmpty"), rows: dates(data.onTrial, (pd) => t("company.trialUntil", { date: day(pd.date) })) }]
      : []),
    {
      title: t("company.awayThisWeek"),
      empty: t("company.awayEmpty"),
      rows: away.map((v) => ({
        key: `v${v.id}`,
        user: { id: v.userId, name: v.userName, email: "", color: "", avatarUrl: null },
        note: `${t(`vacations.types.${v.type}`)} · ${day(v.start)} – ${day(v.end)}`,
      })),
    },
  ];

  return (
    <>
      <div className={s.counters}>
        {tiles.map((tile) => (
          <div key={tile.label} className={s.counter}>
            <span className={s.value}>{tile.value}</span>
            <span className={s.label}>{tile.label}</span>
          </div>
        ))}
      </div>
      <div className={s.lists}>
        {lists.map((l) => (
          <Panel key={l.title} compact>
            <h2>{l.title}</h2>
            {l.rows.length === 0 && <Muted as="p" className={s.flush}>{l.empty}</Muted>}
            <div className={s.rows}>
              {l.rows.map((r) => (
                <Link key={r.key} to={`/people/${r.user.id}`} className={s.row}>
                  <Avatar user={r.user.color ? r.user : { ...r.user, color: "#8e8e93" }} size={30} />
                  <span className={s.main}>
                    <span className={s.name}>{r.user.name}</span>
                    <small>{r.note}</small>
                  </span>
                </Link>
              ))}
            </div>
          </Panel>
        ))}
      </div>
    </>
  );
}
