import { useTranslation } from "react-i18next";
import { Navigate, useNavigate, useParams } from "react-router-dom";
import { Page } from "@/components/Page/Page";
import { PeopleReport } from "@/components/PeopleReport/PeopleReport";
import { TimeReport } from "@/components/TimeReport/TimeReport";
import { Tabs } from "@/components/ui/Tabs/Tabs";
import { useAuth } from "@/context/auth";
import { can } from "@/lib/companyRoles";
import s from "./Reports.module.scss";

type Kind = "time" | "billability" | "bench";

/** The time report for everyone; billability and bench for staff. */
export default function Reports() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const navigate = useNavigate();
  const { kind = "time" } = useParams();
  const crew = can(user, "viewCrewBlock");
  const kinds: Kind[] = crew ? ["time", "billability", "bench"] : ["time"];
  if (!kinds.includes(kind as Kind)) return <Navigate to="/reports" replace />;

  return (
    <Page>
      <h1 className={s.title}>{t("reports.title")}</h1>
      {kinds.length > 1 && (
        <Tabs
          tabs={kinds.map((k) => ({ id: k, label: t(`reports.kinds.${k}`) }))}
          value={kind as Kind}
          onChange={(k) => navigate(k === "time" ? "/reports" : `/reports/${k}`)}
        />
      )}
      {kind === "time" ? <TimeReport /> : <PeopleReport key={kind} mode={kind as "billability" | "bench"} />}
    </Page>
  );
}
