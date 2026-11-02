import { useTranslation } from "react-i18next";
import { InstallApp } from "@/components/InstallApp/InstallApp";
import { Page } from "@/components/Page/Page";
import { PreferencesSettings } from "@/components/PreferencesSettings/PreferencesSettings";
import { ProfileSettings } from "@/components/ProfileSettings/ProfileSettings";
import { Muted } from "@/components/ui/Muted/Muted";
import s from "./Settings.module.scss";

export default function SettingsPage() {
  const { t } = useTranslation();
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
      <InstallApp />
    </Page>
  );
}
