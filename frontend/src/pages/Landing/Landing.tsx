import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { LanguageSelect } from "@/components/LanguageSelect/LanguageSelect";
import { Logo } from "@/components/Logo/Logo";
import { ThemeToggle } from "@/components/ThemeToggle/ThemeToggle";
import { buttonClass } from "@/components/ui/Button/Button";
import { useAuth } from "@/context/auth";
import s from "./Landing.module.scss";

const features = [
  { icon: "▤", title: "landing.features.boardTitle", text: "landing.features.boardText" },
  { icon: "◎", title: "landing.features.teamTitle", text: "landing.features.teamText" },
  { icon: "⚑", title: "landing.features.priorityTitle", text: "landing.features.priorityText" },
  { icon: "◐", title: "landing.features.prefsTitle", text: "landing.features.prefsText" },
] as const;

const preview = [
  { title: "landing.preview.todo", cards: ["landing.preview.card1", "landing.preview.card2"] },
  { title: "landing.preview.doing", cards: ["landing.preview.card3"] },
  { title: "landing.preview.done", cards: ["landing.preview.card4", "landing.preview.card5"] },
] as const;

export default function Landing() {
  const { t } = useTranslation();
  const { user } = useAuth();

  return (
    <div className={s.landing}>
      <header className={s.nav}>
        <Logo compact />
        <nav>
          <LanguageSelect />
          <ThemeToggle />
          {user ? (
            <Link className={buttonClass({ variant: "primary" })} to="/home">
              {t("landing.openBoard")}
            </Link>
          ) : (
            <>
              <Link className={buttonClass()} to="/login">
                {t("landing.signIn")}
              </Link>
              {/* Hidden on phones: the hero's "Create account" is right below. */}
              <Link className={buttonClass({ variant: "primary", className: s.getStarted })} to="/register">
                {t("landing.getStarted")}
              </Link>
            </>
          )}
        </nav>
      </header>

      <section className={s.hero}>
        <div>
          <span className={s.pill}>Go · PostgreSQL · React</span>
          <h1>
            {t("landing.titleStart")} <span className={s.accent}>{t("landing.titleAccent")}</span>
          </h1>
          <p>{t("landing.lead")}</p>
          <div className={s.cta}>
            <Link className={buttonClass({ variant: "primary", size: "lg" })} to={user ? "/home" : "/register"}>
              {user ? t("landing.goToBoard") : t("landing.createAccount")}
            </Link>
            {!user && (
              <Link className={buttonClass({ size: "lg" })} to="/login">
                {t("landing.demo")}
              </Link>
            )}
          </div>
        </div>

        <div className={s.preview} aria-hidden="true">
          {preview.map((col) => (
            <div className={s.previewCol} key={col.title}>
              <div className={s.previewColTitle}>{t(col.title)}</div>
              {col.cards.map((card, i) => (
                <div className={s.previewCard} key={card}>
                  <span>{t(card)}</span>
                  <span className={s.previewDot} style={{ background: ["#0052CC", "#FF5630", "#00875A"][i % 3] }} />
                </div>
              ))}
            </div>
          ))}
        </div>
      </section>

      <section className={s.features}>
        {features.map((f) => (
          <div className={s.feature} key={f.title}>
            <div className={s.featureIcon}>{f.icon}</div>
            <h3>{t(f.title)}</h3>
            <p>{t(f.text)}</p>
          </div>
        ))}
      </section>

      <section className={s.band}>
        <h2>{t("landing.ctaTitle")}</h2>
        <p>{t("landing.ctaText")}</p>
        <Link className={buttonClass({ variant: "light", size: "lg" })} to={user ? "/home" : "/register"}>
          {user ? t("landing.ctaBoard") : t("landing.ctaSignUp")}
        </Link>
      </section>

      <footer className={s.footer}>© {new Date().getFullYear()} OneBoard</footer>
    </div>
  );
}
