package httpapi

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/auth"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/organization"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/user"
)

func init() { registerRoutes(NewUserHandler) }

type UserHandler struct {
	users   *user.Service
	orgs    *organization.Service
	auth    *auth.Service
	present *Presenter
}

func NewUserHandler(users *user.Service, orgs *organization.Service, a *auth.Service, present *Presenter) *UserHandler {
	return &UserHandler{users: users, orgs: orgs, auth: a, present: present}
}

// me answers with the account after a change to it (avatar, settings), in the current organization.
func (h *UserHandler) me(c *echo.Context, changed domain.User) error {
	u := currentUser(c)
	u.Name, u.Email, u.Color, u.AvatarKey, u.AvatarUpdatedAt, u.Settings =
		changed.Name, changed.Email, changed.Color, changed.AvatarKey, changed.AvatarUpdatedAt, changed.Settings
	u, ms, err := h.auth.Account(c.Request().Context(), u, u.OrgID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, h.present.Me(u, ms))
}

func (h *UserHandler) Mount(public, private *echo.Group) {
	private.PUT("/users/me/settings", h.updateSettings)
	private.PUT("/users/me/avatar", h.uploadAvatar)
	private.DELETE("/users/me/avatar", h.removeAvatar)
	private.PUT("/users/:id/role", h.changeRole)
	private.POST("/users/:id/dismiss", h.dismiss)
	private.POST("/users/:id/activate", h.setActive(true))
	private.POST("/users/:id/deactivate", h.setActive(false))
	public.GET("/users/:id/avatar", h.avatar) // authorised by the URL signature, see Presenter
}

// uploadAvatar accepts multipart/form-data with the image in the "avatar" field.
func (h *UserHandler) uploadAvatar(c *echo.Context) error {
	r := c.Request()
	r.Body = http.MaxBytesReader(c.Response(), r.Body, user.MaxAvatarBytes+64<<10) // room for multipart headers
	file, _, err := r.FormFile("avatar")
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return domain.ErrAvatarTooLarge
		}
		return domain.ErrAvatarMissing
	}
	defer file.Close()

	u, err := h.users.UploadAvatar(r.Context(), currentUser(c).ID, file)
	if err != nil {
		return err
	}
	return h.me(c, u)
}

func (h *UserHandler) removeAvatar(c *echo.Context) error {
	u, err := h.users.RemoveAvatar(c.Request().Context(), currentUser(c).ID)
	if err != nil {
		return err
	}
	return h.me(c, u)
}

func (h *UserHandler) updateSettings(c *echo.Context) error {
	var in settingsDTO
	if err := bind(c, &in); err != nil {
		return err
	}
	u, err := h.users.UpdateSettings(c.Request().Context(), currentUser(c).ID, domain.Settings{Language: in.Language, Theme: in.Theme})
	if err != nil {
		return err
	}
	return h.me(c, u)
}

// avatar streams the image from object storage. URLs carry ?v=<version>, so it is cached for good.
func (h *UserHandler) avatar(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	v, _ := strconv.ParseInt(c.QueryParam("v"), 10, 64)
	if !h.present.validAvatarSig(id, v, c.QueryParam("sig")) {
		return domain.ErrAvatarNotFound
	}
	body, info, err := h.users.Avatar(c.Request().Context(), id)
	if err != nil {
		return err
	}
	defer body.Close()

	w := c.Response()
	w.Header().Set("Content-Type", info.ContentType)
	if info.Size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
	}
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, body)
	return nil
}

type roleRequest struct {
	Role          domain.CompanyRole    `json:"role"`
	AdditionRoles []domain.AdditionRole `json:"additionRoles"` // omitted or null: unchanged
}

func (h *UserHandler) changeRole(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var req roleRequest
	if err := bind(c, &req); err != nil {
		return err
	}
	// A role is a membership of the organization the request works in.
	actor := currentUser(c)
	members, err := h.orgs.ChangeRole(c.Request().Context(), actor.ID, actor.OrgID, id, req.Role, req.AdditionRoles)
	if err != nil {
		return err
	}
	for _, m := range members {
		if m.ID == id {
			return c.JSON(http.StatusOK, h.present.User(m.User))
		}
	}
	return errNotFound
}

func (h *UserHandler) dismiss(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var in struct {
		Reason string `json:"reason"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	u, err := h.users.Dismiss(c.Request().Context(), currentUser(c), id, in.Reason)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, h.present.User(u))
}

func (h *UserHandler) setActive(active bool) echo.HandlerFunc {
	return func(c *echo.Context) error {
		id, err := pathID(c, "id")
		if err != nil {
			return err
		}
		u, err := h.users.SetActive(c.Request().Context(), currentUser(c), id, active)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, h.present.User(u))
	}
}
