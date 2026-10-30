import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Navigate, Outlet, useLocation } from "react-router-dom";
import { api, setSessionLostHandler } from "./api";
import { applySettings } from "./preferences";
import type { CurrentUser } from "./types";

interface AuthState {
  user: CurrentUser | null;
  loading: boolean;
  login: (email: string, password: string) => Promise<void>;
  register: (name: string, email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  /** Replaces the current user after a profile change (e.g. a new avatar). */
  setUser: (user: CurrentUser) => void;
}

const AuthContext = createContext<AuthState | null>(null);

// The last signed-in user's public profile (no tokens), so the installed app can open offline.
const USER_CACHE_KEY = "oneboard:user";

function cacheUser(user: CurrentUser | null) {
  try {
    if (user) localStorage.setItem(USER_CACHE_KEY, JSON.stringify(user));
    else localStorage.removeItem(USER_CACHE_KEY);
  } catch {
    // storage unavailable — offline start just won't be possible
  }
}

function cachedUser(): CurrentUser | null {
  try {
    const raw = localStorage.getItem(USER_CACHE_KEY);
    return raw ? (JSON.parse(raw) as CurrentUser) : null;
  } catch {
    return null;
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<CurrentUser | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setSessionLostHandler(() => setUser(null)); // RequireAuth then redirects to the login page
    let cancelled = false;
    const restore = () =>
      api
        .restoreSession()
        .then((u) => !cancelled && setUser(u))
        .catch(() => {
          // Offline: show the app with the last known profile and restore the session once back online.
          if (cancelled) return;
          setUser(cachedUser());
          window.addEventListener("online", () => void restore(), { once: true });
        })
        .finally(() => !cancelled && setLoading(false));
    void restore();
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (!loading) cacheUser(user);
  }, [user, loading]);

  // The account's settings win over whatever this browser had; signing out keeps them locally.
  const language = user?.settings.language;
  const theme = user?.settings.theme;
  useEffect(() => {
    if (language && theme) applySettings({ language, theme });
  }, [language, theme]);

  const value: AuthState = {
    user,
    loading,
    login: async (email, password) => setUser(await api.login(email, password)),
    register: async (name, email, password) => setUser(await api.register(name, email, password)),
    logout: async () => {
      await api.logout();
      setUser(null);
    },
    setUser,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used inside AuthProvider");
  return ctx;
}

export function RequireAuth({ children }: { children: ReactNode }) {
  const { t } = useTranslation();
  const { user, loading } = useAuth();
  const location = useLocation();
  if (loading) return <div className="center-screen">{t("common.loading")}</div>;
  if (!user) return <Navigate to="/login" replace state={{ from: location.pathname }} />;
  return <>{children}</>;
}

/** Pages for signed-out visitors (landing, sign-in, sign-up); signed-in users go to their home page. */
export function PublicOnly() {
  const { t } = useTranslation();
  const { user, loading } = useAuth();
  const location = useLocation();
  if (loading) return <div className="center-screen">{t("common.loading")}</div>;
  if (user) {
    const from = (location.state as { from?: string } | null)?.from;
    return <Navigate to={from ?? "/home"} replace />;
  }
  return <Outlet />;
}
