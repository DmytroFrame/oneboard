import { Link } from "react-router-dom";
import { useAuth } from "../auth";
import { Logo } from "../components/Logo";
import { ThemeToggle } from "../theme";

const features = [
  {
    icon: "▤",
    title: "Kanban-дошка",
    text: "Окрема дошка для кожного проєкту: чотири колонки від ідеї до релізу. Перетягуйте картки мишкою — порядок зберігається на сервері.",
  },
  {
    icon: "◎",
    title: "Вся команда на виду",
    text: "Запрошуйте людей у проєкти. Аватарки учасників як у Jira: один клік — і бачите лише задачі конкретної людини.",
  },
  {
    icon: "⚑",
    title: "Пріоритети та виконавці",
    text: "Автор, виконавець, пріоритет і опис для кожної картки з унікальним ключем KAN-N.",
  },
  {
    icon: "◐",
    title: "Світла й темна тема",
    text: "Перемикайте тему одним кліком. Вхід за email і паролем з bcrypt та HttpOnly-сесіями.",
  },
];

export default function Landing() {
  const { user } = useAuth();

  return (
    <div className="landing">
      <header className="landing-nav">
        <Logo />
        <nav>
          <ThemeToggle />
          {user ? (
            <Link className="btn btn-primary" to="/projects">
              Відкрити дошку
            </Link>
          ) : (
            <>
              <Link className="btn btn-ghost" to="/login">
                Увійти
              </Link>
              <Link className="btn btn-primary" to="/register">
                Почати безкоштовно
              </Link>
            </>
          )}
        </nav>
      </header>

      <section className="hero">
        <div className="hero-text">
          <span className="pill">Go · SQLite · React</span>
          <h1>
            Плануйте роботу команди <span className="accent">без хаосу</span>
          </h1>
          <p>
            Flowboard — легка kanban-дошка для невеликих команд. Задачі, виконавці та прогрес в одному місці, без
            зайвих налаштувань.
          </p>
          <div className="hero-cta">
            <Link className="btn btn-primary btn-lg" to={user ? "/projects" : "/register"}>
              {user ? "Перейти до дошки" : "Створити акаунт"}
            </Link>
            {!user && (
              <Link className="btn btn-ghost btn-lg" to="/login">
                Демо: demo@example.com
              </Link>
            )}
          </div>
        </div>

        <div className="hero-preview" aria-hidden="true">
          {[
            ["До виконання", ["Налаштувати CI/CD", "Дизайн профілю"]],
            ["В роботі", ["API коментарів"]],
            ["Готово", ["Лендінг", "Фікс логіну"]],
          ].map(([title, items]) => (
            <div className="preview-col" key={title as string}>
              <div className="preview-col-title">{title as string}</div>
              {(items as string[]).map((t, i) => (
                <div className="preview-card" key={t}>
                  <span>{t}</span>
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
            <h3>{f.title}</h3>
            <p>{f.text}</p>
          </div>
        ))}
      </section>

      <section className="cta-band">
        <h2>Готові спробувати?</h2>
        <p>Реєстрація займає 10 секунд.</p>
        <Link className="btn btn-light btn-lg" to={user ? "/projects" : "/register"}>
          {user ? "До дошки" : "Зареєструватися"}
        </Link>
      </section>

      <footer className="landing-footer">© {new Date().getFullYear()} Flowboard</footer>
    </div>
  );
}
