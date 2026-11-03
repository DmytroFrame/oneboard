import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { buttonClass } from "@/components/ui/Button/Button";
import { useAuth } from "@/context/auth";
import { homeRoute } from "@/lib/companyRoles";
import s from "./StatusPage.module.scss";

/** 403 / 404 inside the app shell, with a way back to the user's home page. */
export function StatusPage({ code }: { code: 403 | 404 }) {
  const { t } = useTranslation();
  const { user } = useAuth();
  const forbidden = code === 403;
  return (
    <div className={s.page}>
      <div className={s.code}>{code}</div>
      <h1>{forbidden ? t("status.forbiddenTitle") : t("status.notFoundTitle")}</h1>
      <p>{forbidden ? t("status.forbiddenText") : t("status.notFoundText")}</p>
      <Link className={buttonClass({ variant: "primary" })} to={homeRoute(user)}>
        {t("status.toHome")}
      </Link>
    </div>
  );
}
