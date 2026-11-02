import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/Button/Button";
import { Muted } from "@/components/ui/Muted/Muted";
import { Panel } from "@/components/ui/Panel/Panel";
import { isIOS, promptInstall, useInstallState } from "@/lib/pwa";
import s from "./InstallApp.module.scss";

/** Installing OneBoard as an app (PWA): a button where the browser allows it, instructions on iOS. */
export function InstallApp() {
  const { t } = useTranslation();
  const { canInstall, installed } = useInstallState();
  if (!installed && !canInstall && !isIOS) return null; // browser can't install (or already offered elsewhere)
  return (
    <Panel className={s.card}>
      <img src="/pwa-192x192.png" alt="" width={56} height={56} />
      <div className={s.text}>
        <h2>{t("pwa.title")}</h2>
        <Muted as="p">{installed ? t("pwa.installed") : isIOS && !canInstall ? t("pwa.iosHint") : t("pwa.text")}</Muted>
      </div>
      {canInstall && !installed && (
        <Button variant="primary" onClick={() => void promptInstall()}>
          {t("pwa.install")}
        </Button>
      )}
    </Panel>
  );
}
