import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { InstallApp } from "@/components/InstallApp/InstallApp";
import { Page } from "@/components/Page/Page";
import { PreferencesSettings } from "@/components/PreferencesSettings/PreferencesSettings";
import { ProfileSettings } from "@/components/ProfileSettings/ProfileSettings";
import { SecuritySettings } from "@/components/SecuritySettings/SecuritySettings";
import { buttonClass } from "@/components/ui/Button/Button";
import { Muted } from "@/components/ui/Muted/Muted";
import { Panel } from "@/components/ui/Panel/Panel";
import { useAuth } from "@/context/auth";
import { can } from "@/lib/companyRoles";
import s from "./Settings.module.scss";

export default function SettingsPage() {
  const { t } = useTranslation();
  const { user } = useAuth();
  return (
    <Page width="narrow">
      <div className={s.head}>
        <h1>{t("settings.title")}</h1>
        <Muted as="p" className={s.subtitle}>
          {t("settings.subtitle")}
        </Muted>
      </div>
      <ProfileSettings />
      <PreferencesSettings />
      <SecuritySettings />
      <InstallApp />
      {(user?.platformAdmin || can(user, "openBackwoods")) && (
        <Panel>
          <h2>{t("backwoods.title")}</h2>
          <Muted as="p">{t("backwoods.settingsHint")}</Muted>
          <Link to="/backwoods" className={buttonClass({ size: "sm" })}>
            {t("backwoods.open")} →
          </Link>
        </Panel>
      )}
    </Page>
  );
}
