package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"clodapp/backend/internal/usecase/auth"
)

type AuthHandler struct {
	auth *auth.Service
	mw   *Middleware
}

func NewAuthHandler(a *auth.Service, mw *Middleware) *AuthHandler {
	return &AuthHandler{auth: a, mw: mw}
}

func (h *AuthHandler) Mount(public, private chi.Router) {
	public.Post("/auth/register", h.register)
	public.Post("/auth/login", h.login)
	public.Post("/auth/logout", h.logout)
	private.Get("/auth/me", h.me)
}

type credentials struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) register(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if !decode(w, r, &in) {
		return
	}
	u, token, err := h.auth.Register(r.Context(), auth.RegisterInput{Name: in.Name, Email: in.Email, Password: in.Password})
	if err != nil {
		fail(w, r, err)
		return
	}
	h.mw.setSession(w, token)
	writeJSON(w, http.StatusCreated, toMe(u))
}

func (h *AuthHandler) login(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if !decode(w, r, &in) {
		return
	}
	u, token, err := h.auth.Login(r.Context(), in.Email, in.Password)
	if err != nil {
		fail(w, r, err)
		return
	}
	h.mw.setSession(w, token)
	writeJSON(w, http.StatusOK, toMe(u))
}

func (h *AuthHandler) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if err := h.auth.Logout(r.Context(), c.Value); err != nil {
			fail(w, r, err)
			return
		}
	}
	h.mw.clearSession(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, toMe(currentUser(r)))
}
