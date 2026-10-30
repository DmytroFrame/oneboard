package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/usecase/auth"
)

type AuthHandler struct {
	auth    *auth.Service
	mw      *Middleware
	present *Presenter
}

func NewAuthHandler(a *auth.Service, mw *Middleware, present *Presenter) *AuthHandler {
	return &AuthHandler{auth: a, mw: mw, present: present}
}

func (h *AuthHandler) Mount(public, private chi.Router) {
	public.Post("/auth/register", h.register)
	public.Post("/auth/login", h.login)
	public.Post("/auth/refresh", h.refresh)
	public.Post("/auth/logout", h.logout)
	private.Get("/auth/me", h.me)
}

type credentials struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// respond sends the access token in the body and (re)sets the refresh token cookie.
func (h *AuthHandler) respond(w http.ResponseWriter, status int, s auth.Session) {
	if s.Refresh != "" {
		h.mw.setRefreshCookie(w, s.Refresh)
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, status, sessionDTO{User: h.present.Me(s.User), AccessToken: s.Access.Value, ExpiresAt: s.Access.ExpiresAt})
}

func (h *AuthHandler) register(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if !decode(w, r, &in) {
		return
	}
	s, err := h.auth.Register(r.Context(), auth.RegisterInput{Name: in.Name, Email: in.Email, Password: in.Password})
	if err != nil {
		fail(w, r, err)
		return
	}
	h.respond(w, http.StatusCreated, s)
}

func (h *AuthHandler) login(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if !decode(w, r, &in) {
		return
	}
	s, err := h.auth.Login(r.Context(), in.Email, in.Password)
	if err != nil {
		fail(w, r, err)
		return
	}
	h.respond(w, http.StatusOK, s)
}

// refresh trades the refresh token cookie for a new access token and a rotated cookie.
func (h *AuthHandler) refresh(w http.ResponseWriter, r *http.Request) {
	s, err := h.auth.Refresh(r.Context(), refreshTokenFrom(r))
	if err != nil {
		h.mw.clearRefreshCookie(w)
		fail(w, r, err)
		return
	}
	h.respond(w, http.StatusOK, s)
}

// logout revokes the refresh token and the access token (if one was sent), even an expired session.
func (h *AuthHandler) logout(w http.ResponseWriter, r *http.Request) {
	if err := h.auth.Logout(r.Context(), refreshTokenFrom(r), bearerToken(r)); err != nil {
		fail(w, r, err)
		return
	}
	h.mw.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.present.Me(currentUser(r)))
}
