package httpapi

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/usecase/auth"
)

func init() { registerRoutes(NewAuthHandler) }

type AuthHandler struct {
	auth    *auth.Service
	mw      *Middleware
	present *Presenter
}

func NewAuthHandler(a *auth.Service, mw *Middleware, present *Presenter) *AuthHandler {
	return &AuthHandler{auth: a, mw: mw, present: present}
}

func (h *AuthHandler) Mount(public, private *echo.Group) {
	public.POST("/auth/register", h.register)
	public.POST("/auth/login", h.login)
	public.POST("/auth/refresh", h.refresh)
	public.POST("/auth/logout", h.logout)
	private.GET("/auth/me", h.me)
	public.POST("/auth/password-reset", h.requestReset)
	public.GET("/auth/password-reset/:token", h.checkReset)
	public.POST("/auth/password-reset/:token", h.reset)
	private.PUT("/users/me/password", h.changePassword)
	private.POST("/users/:id/password-reset", h.adminReset)
}

// requestReset always answers 204, so it can't be used to find out who has an account.
func (h *AuthHandler) requestReset(c *echo.Context) error {
	var in struct {
		Email string `json:"email"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	if err := h.auth.RequestPasswordReset(c.Request().Context(), in.Email); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *AuthHandler) checkReset(c *echo.Context) error {
	if err := h.auth.CheckResetToken(c.Request().Context(), c.Param("token")); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *AuthHandler) reset(c *echo.Context) error {
	var in struct {
		Password string `json:"password"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	if err := h.auth.ResetPassword(c.Request().Context(), c.Param("token"), in.Password); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *AuthHandler) changePassword(c *echo.Context) error {
	var in struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	if err := h.auth.ChangePassword(c.Request().Context(), currentUser(c).ID, in.CurrentPassword, in.NewPassword); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *AuthHandler) adminReset(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	if err := h.auth.AdminResetPassword(c.Request().Context(), currentUser(c), id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

type credentials struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// respond sends the access token in the body and (re)sets the refresh token cookie.
func (h *AuthHandler) respond(c *echo.Context, status int, s auth.Session) error {
	if s.Refresh != "" {
		h.mw.setRefreshCookie(c.Response(), s.Refresh)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	u, ms, err := h.auth.Account(c.Request().Context(), s.User, orgHeader(c.Request()))
	if err != nil {
		return err
	}
	return c.JSON(status, sessionDTO{User: h.present.Me(u, ms), AccessToken: s.Access.Value, ExpiresAt: s.Access.ExpiresAt})
}

func (h *AuthHandler) register(c *echo.Context) error {
	var in credentials
	if err := bind(c, &in); err != nil {
		return err
	}
	s, err := h.auth.Register(c.Request().Context(), auth.RegisterInput{Name: in.Name, Email: in.Email, Password: in.Password})
	if err != nil {
		return err
	}
	return h.respond(c, http.StatusCreated, s)
}

func (h *AuthHandler) login(c *echo.Context) error {
	var in credentials
	if err := bind(c, &in); err != nil {
		return err
	}
	s, err := h.auth.Login(c.Request().Context(), in.Email, in.Password)
	if err != nil {
		return err
	}
	return h.respond(c, http.StatusOK, s)
}

// refresh trades the refresh token cookie for a new access token and a rotated cookie.
func (h *AuthHandler) refresh(c *echo.Context) error {
	s, err := h.auth.Refresh(c.Request().Context(), refreshTokenFrom(c.Request()))
	if err != nil {
		h.mw.clearRefreshCookie(c.Response())
		return err
	}
	return h.respond(c, http.StatusOK, s)
}

// logout revokes the refresh token and the access token (if one was sent), even an expired session.
func (h *AuthHandler) logout(c *echo.Context) error {
	r := c.Request()
	if err := h.auth.Logout(r.Context(), refreshTokenFrom(r), bearerToken(r)); err != nil {
		return err
	}
	h.mw.clearRefreshCookie(c.Response())
	return c.NoContent(http.StatusNoContent)
}

func (h *AuthHandler) me(c *echo.Context) error {
	u := currentUser(c)
	u, ms, err := h.auth.Account(c.Request().Context(), u, u.OrgID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, h.present.Me(u, ms))
}
