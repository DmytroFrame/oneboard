import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { useAuth } from "../auth";
import { LanguageSelect } from "../components/LanguageSelect";
import { Logo } from "../components/Logo";
import { ThemeToggle } from "../theme";

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
    <div className="landing">
      <header className="landing-nav">
        <Logo />
        <nav>
          <LanguageSelect />
          <ThemeToggle />
          {user ? (
            <Link className="btn btn-primary" to="/home">
              {t("landing.openBoard")}
            </Link>
          ) : (
            <>
              <Link className="btn btn-ghost" to="/login">
                {t("landing.signIn")}
              </Link>
              <Link className="btn btn-primary" to="/register">
                {t("landing.getStarted")}
              </Link>
            </>
          )}
        </nav>
      </header>

      <section className="hero">
        <div className="hero-text">
          <span className="pill">Go · PostgreSQL · React</span>
          <h1>
            {t("landing.titleStart")} <span className="accent">{t("landing.titleAccent")}</span>
          </h1>
          <p>{t("landing.lead")}</p>
          <div className="hero-cta">
            <Link className="btn btn-primary btn-lg" to={user ? "/home" : "/register"}>
              {user ? t("landing.goToBoard") : t("landing.createAccount")}
            </Link>
            {!user && (
              <Link className="btn btn-ghost btn-lg" to="/login">
                {t("landing.demo")}
              </Link>
            )}
          </div>
        </div>

        <div className="hero-preview" aria-hidden="true">
          {preview.map((col) => (
            <div className="preview-col" key={col.title}>
              <div className="preview-col-title">{t(col.title)}</div>
              {col.cards.map((card, i) => (
                <div className="preview-card" key={card}>
                  <span>{t(card)}</span>
                  <span className="preview-dot" style={{ background: ["#0052CC", "#FF5630", "#00875A"][i % 3] }} />
                </div>
              ))}
            </div>
          ))}
        </div>
      </section>

      <section className="features">
        {features.map((f) => (
          <div className="feature" key={f.title}>
            <div className="feature-icon">{f.icon}</div>
            <h3>{t(f.title)}</h3>
            <p>{t(f.text)}</p>
          </div>
        ))}
      </section>

      <section className="cta-band">
        <h2>{t("landing.ctaTitle")}</h2>
        <p>{t("landing.ctaText")}</p>
        <Link className="btn btn-light btn-lg" to={user ? "/home" : "/register"}>
          {user ? t("landing.ctaBoard") : t("landing.ctaSignUp")}
        </Link>
      </section>

      <footer className="landing-footer">© {new Date().getFullYear()} Flowboard</footer>
    </div>
  );
}
