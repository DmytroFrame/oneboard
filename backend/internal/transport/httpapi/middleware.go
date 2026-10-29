package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"clodapp/backend/internal/config"
	"clodapp/backend/internal/domain"
	"clodapp/backend/internal/usecase/auth"
)

const sessionCookie = "session"

type ctxKey struct{}

type Middleware struct {
	auth          *auth.Service
	log           *slog.Logger
	secureCookies bool
}

func NewMiddleware(a *auth.Service, log *slog.Logger, cfg config.Config) *Middleware {
	return &Middleware{auth: a, log: log, secureCookies: cfg.SecureCookies}
}

// currentUser is set by RequireAuth.
func currentUser(r *http.Request) domain.User {
	u, _ := r.Context().Value(ctxKey{}).(domain.User)
	return u
}

func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "не авторизовано")
			return
		}
		u, err := m.auth.Authenticate(r.Context(), c.Value)
		if err != nil {
			fail(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, u)))
	})
}

func (m *Middleware) Log(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		m.log.Info("request", "method", r.Method, "path", r.URL.Path, "status", ww.Status(), "took", time.Since(start).Round(time.Microsecond))
	})
}

func (m *Middleware) setSession(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(auth.SessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   m.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

func (m *Middleware) clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: m.secureCookies})
}
