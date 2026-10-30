package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"clodapp/backend/internal/config"
	"clodapp/backend/internal/domain"
	"clodapp/backend/internal/usecase"
	"clodapp/backend/internal/usecase/auth"
)

// The refresh token cookie is scoped to /api/auth, so it is only ever sent to refresh and logout.
const (
	refreshCookie     = "refresh_token"
	refreshCookiePath = "/api/auth"
)

type ctxKey struct{}

type Middleware struct {
	auth          *auth.Service
	log           *slog.Logger
	secureCookies bool
	refreshTTL    time.Duration
}

func NewMiddleware(a *auth.Service, log *slog.Logger, cfg config.Config) *Middleware {
	return &Middleware{auth: a, log: log, secureCookies: cfg.SecureCookies, refreshTTL: cfg.RefreshTokenTTL}
}

// currentUser is set by RequireAuth.
func currentUser(r *http.Request) domain.User {
	u, _ := r.Context().Value(ctxKey{}).(domain.User)
	return u
}

// bearerToken extracts the token from "Authorization: Bearer <token>".
func bearerToken(r *http.Request) string {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

// RequireAuth accepts only a valid, unrevoked JWT access token in the Authorization header.
// Cookies are deliberately ignored here, which also rules out CSRF on the API.
func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			fail(w, r, domain.ErrNotAuthenticated)
			return
		}
		u, err := m.auth.Authenticate(r.Context(), token)
		if err != nil {
			fail(w, r, err)
			return
		}
		ctx := context.WithValue(r.Context(), ctxKey{}, u)
		// The tab's real-time connection id (hex, see newConnID), echoed in the board events this request causes.
		if origin := r.Header.Get("X-Realtime-Conn"); len(origin) <= 32 && isHex(origin) {
			ctx = usecase.WithOrigin(ctx, origin)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
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

func (m *Middleware) setRefreshCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookie,
		Value:    token,
		Path:     refreshCookiePath,
		MaxAge:   int(m.refreshTTL.Seconds()),
		HttpOnly: true,
		Secure:   m.secureCookies,
		SameSite: http.SameSiteStrictMode,
	})
}

func (m *Middleware) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: refreshCookie, Value: "", Path: refreshCookiePath, MaxAge: -1,
		HttpOnly: true, Secure: m.secureCookies, SameSite: http.SameSiteStrictMode,
	})
	// Drop the cookie of the pre-JWT session auth, if the browser still has one.
	http.SetCookie(w, &http.Cookie{Name: "session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
}

func refreshTokenFrom(r *http.Request) string {
	if c, err := r.Cookie(refreshCookie); err == nil {
		return c.Value
	}
	return ""
}

func isHex(s string) bool {
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return s != ""
}
