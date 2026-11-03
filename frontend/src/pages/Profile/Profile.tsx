import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useParams, useSearchParams } from "react-router-dom";
import { PersonComments } from "@/components/PersonComments/PersonComments";
import { PersonDocuments } from "@/components/PersonDocuments/PersonDocuments";
import { PersonFeedback } from "@/components/PersonFeedback/PersonFeedback";
import { PersonFinance } from "@/components/PersonFinance/PersonFinance";
import { PersonHeader } from "@/components/PersonHeader/PersonHeader";
import { ProfileDetails } from "@/components/ProfileDetails/ProfileDetails";
import { Page } from "@/components/Page/Page";
import { StatusPage } from "@/components/StatusPage/StatusPage";
import { Center } from "@/components/ui/Center/Center";
import { Tabs, type Tab } from "@/components/ui/Tabs/Tabs";
import { useAuth } from "@/context/auth";
import { peopleApi } from "@/lib/api/people";
import { can } from "@/lib/companyRoles";
import type { Person } from "@/types/people";

type TabId = "details" | "comments" | "feedback" | "finance" | "documents";

/** A person's profile (/people/:id) or your own (/profile), with tabs. */
export default function Profile() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { id: param } = useParams();
  const id = param ? Number(param) : user!.id;
  const [person, setPerson] = useState<Person | null>(null);
  const [missing, setMissing] = useState(false);
  const [params, setParams] = useSearchParams();

  const load = useCallback(() => {
    peopleApi
      .get(id)
      .then((p) => (setPerson(p), setMissing(false)))
      .catch(() => setMissing(true));
  }, [id]);
  useEffect(load, [load, user?.avatarUrl, user?.role]);

  if (missing) return <StatusPage code={404} />;
  if (!person) return <Center>{t("common.loading")}</Center>;

  const self = person.access.self;
  const tabs: Tab<TabId>[] = [{ id: "details", label: t("people.tabDetails") }];
  if (!self && can(user, "viewUserComments")) tabs.push({ id: "comments", label: t("people.tabComments") });
  if (self || can(user, "viewUserComments")) tabs.push({ id: "feedback", label: t("people.tabFeedback") });
  if (person.access.seeFinance) tabs.push({ id: "finance", label: t("people.tabFinance") });
  tabs.push({ id: "documents", label: t("people.tabDocuments") });
  const requested = params.get("tab") as TabId | null;
  const tab = tabs.some((x) => x.id === requested) ? requested! : "details";

  return (
    <Page width="medium">
      <PersonHeader person={person} onChanged={load} />
      <Tabs tabs={tabs} value={tab} onChange={(next) => setParams(next === "details" ? {} : { tab: next }, { replace: true })} />
      {tab === "details" && <ProfileDetails person={person} onSaved={setPerson} />}
      {tab === "comments" && <PersonComments person={person} />}
      {tab === "feedback" && <PersonFeedback person={person} />}
      {tab === "finance" && <PersonFinance person={person} onSaved={setPerson} />}
      {tab === "documents" && <PersonDocuments person={person} />}
    </Page>
  );
}
