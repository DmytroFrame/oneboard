package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/config"
	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/auth"
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

// currentUser is set by RequireAuth. It lives in the request context (not only in the echo context),
// next to the realtime origin that the use cases read.
func currentUser(c *echo.Context) domain.User {
	u, _ := c.Request().Context().Value(ctxKey{}).(domain.User)
	return u
}

// orgHeader is the organization the client works in (0 = not chosen yet).
func orgHeader(r *http.Request) int64 {
	id, err := strconv.ParseInt(r.Header.Get("X-Organization"), 10, 64)
	if err != nil || id < 0 {
		return 0
	}
	return id
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
func (m *Middleware) RequireAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		r := c.Request()
		token := bearerToken(r)
		if token == "" {
			return domain.ErrNotAuthenticated
		}
		u, err := m.auth.Authenticate(r.Context(), token)
		if err != nil {
			return err
		}
		// Every request works in one organization (X-Organization, else the user's first); the user
		// carries their role there, so permission checks and queries stay inside it.
		if u, err = m.auth.InOrg(r.Context(), u, orgHeader(r)); err != nil {
			return err
		}
		ctx := context.WithValue(r.Context(), ctxKey{}, u)
		// The tab's real-time connection id (hex, see newConnID), echoed in the board events this request causes.
		if origin := r.Header.Get("X-Realtime-Conn"); len(origin) <= 32 && isHex(origin) {
			ctx = usecase.WithOrigin(ctx, origin)
		}
		c.SetRequest(r.WithContext(ctx))
		return next(c)
	}
}

// Log writes one line per API request, with the status the error handler is going to send.
func (m *Middleware) Log(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		start := time.Now()
		err := next(c)
		_, status := echo.ResolveResponseStatus(c.Response(), err)
		if e := resolveError(err); err != nil && e != nil {
			status = e.status
		}
		r := c.Request()
		m.log.Info("request", "method", r.Method, "path", r.URL.Path, "status", status, "took", time.Since(start).Round(time.Microsecond))
		return err
	}
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
