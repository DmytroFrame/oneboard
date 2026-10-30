import { useState, type FormEvent, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link, Navigate, useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "../auth";
import { LanguageSelect } from "../components/LanguageSelect";
import { Logo } from "../components/Logo";

function AuthLayout({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="auth-page">
      <div className="auth-card">
        <div className="auth-card-head">
          <Logo />
          <LanguageSelect />
        </div>
        <h1>{title}</h1>
        {children}
      </div>
    </div>
  );
}

function useAfterAuth() {
  const navigate = useNavigate();
  const location = useLocation();
  const from = (location.state as { from?: string } | null)?.from ?? "/projects";
  return () => navigate(from, { replace: true });
}

export function Login() {
  const { t } = useTranslation();
  const { user, login } = useAuth();
  const done = useAfterAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  if (user) return <Navigate to="/projects" replace />;

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await login(email, password);
      done();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <AuthLayout title={t("auth.signInTitle")}>
      <form onSubmit={onSubmit} className="form">
        <label>
          {t("common.email")}
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required autoFocus />
        </label>
        <label>
          {t("common.password")}
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
        </label>
        {error && <div className="form-error">{error}</div>}
        <button className="btn btn-primary btn-block" disabled={busy}>
          {busy ? t("auth.signingIn") : t("auth.signIn")}
        </button>
      </form>
      <p className="auth-hint">
        {t("auth.demoAccount")} <code>demo@example.com</code> / <code>demo1234</code>
      </p>
      <p className="auth-switch">
        {t("auth.noAccount")} <Link to="/register">{t("auth.signUp")}</Link>
      </p>
    </AuthLayout>
  );
}

export function Register() {
  const { t } = useTranslation();
  const { user, register } = useAuth();
  const done = useAfterAuth();
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  if (user) return <Navigate to="/projects" replace />;

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await register(name, email, password);
      done();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <AuthLayout title={t("auth.signUpTitle")}>
      <form onSubmit={onSubmit} className="form">
        <label>
          {t("common.name")}
          <input value={name} onChange={(e) => setName(e.target.value)} required autoFocus maxLength={100} />
        </label>
        <label>
          {t("common.email")}
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
        </label>
        <label>
          {t("common.password")}
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            minLength={8}
            maxLength={72}
          />
          <small>{t("auth.passwordHint")}</small>
        </label>
        {error && <div className="form-error">{error}</div>}
        <button className="btn btn-primary btn-block" disabled={busy}>
          {busy ? t("auth.creating") : t("auth.createAccount")}
        </button>
      </form>
      <p className="auth-switch">
        {t("auth.haveAccount")} <Link to="/login">{t("auth.signIn")}</Link>
      </p>
    </AuthLayout>
  );
}
