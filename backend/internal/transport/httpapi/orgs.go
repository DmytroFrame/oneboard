package httpapi

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/organization"
)

func init() { registerRoutes(NewOrgHandler) }

type OrgHandler struct {
	orgs    *organization.Service
	present *Presenter
}

func NewOrgHandler(orgs *organization.Service, present *Presenter) *OrgHandler {
	return &OrgHandler{orgs: orgs, present: present}
}

func (h *OrgHandler) Mount(_, private *echo.Group) {
	private.GET("/organizations", h.list)
	private.POST("/organizations", h.create)
	private.GET("/organizations/:id", h.get)
	private.PUT("/organizations/:id", h.rename)
	private.DELETE("/organizations/:id", h.delete)
	private.GET("/organizations/:id/members", h.members)
	private.PUT("/organizations/:id/members/:userId", h.changeRole)
	private.DELETE("/organizations/:id/members/:userId", h.removeMember)
	private.GET("/organizations/:id/invitations", h.invitations)
	private.POST("/organizations/:id/invitations", h.invite)
	private.DELETE("/organizations/:id/invitations/:invitationId", h.revokeInvitation)
}

func (h *OrgHandler) list(c *echo.Context) error {
	orgs, err := h.orgs.List(c.Request().Context(), currentUser(c).ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, mapSlice(orgs, toOrg))
}

type orgInput struct {
	Name string `json:"name"`
}

func (h *OrgHandler) create(c *echo.Context) error {
	var in orgInput
	if err := bind(c, &in); err != nil {
		return err
	}
	o, err := h.orgs.Create(c.Request().Context(), currentUser(c).ID, in.Name)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, toOrg(o))
}

func (h *OrgHandler) get(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	o, err := h.orgs.Get(c.Request().Context(), currentUser(c).ID, id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, toOrg(o))
}

func (h *OrgHandler) rename(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var in orgInput
	if err := bind(c, &in); err != nil {
		return err
	}
	o, err := h.orgs.Rename(c.Request().Context(), currentUser(c).ID, id, in.Name)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, toOrg(o))
}

func (h *OrgHandler) delete(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	if err := h.orgs.Delete(c.Request().Context(), currentUser(c).ID, id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *OrgHandler) members(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	ms, err := h.orgs.Members(c.Request().Context(), currentUser(c).ID, id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, h.present.OrgMembers(ms))
}

func (h *OrgHandler) changeRole(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	target, err := pathID(c, "userId")
	if err != nil {
		return err
	}
	var in struct {
		Role domain.OrgRole `json:"role"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	ms, err := h.orgs.ChangeRole(c.Request().Context(), currentUser(c).ID, id, target, in.Role)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, h.present.OrgMembers(ms))
}

// removeMember removes someone, or lets the caller leave when userId is their own id.
func (h *OrgHandler) removeMember(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	target, err := pathID(c, "userId")
	if err != nil {
		return err
	}
	if err := h.orgs.RemoveMember(c.Request().Context(), currentUser(c).ID, id, target); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *OrgHandler) invitations(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	invs, err := h.orgs.Invitations(c.Request().Context(), currentUser(c).ID, id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, mapSlice(invs, toInvitation))
}

// invite adds a registered user right away, or keeps an invitation that is accepted on sign-up.
// Responds with {"status": "added" | "invited"}.
func (h *OrgHandler) invite(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var in struct {
		Email string         `json:"email"`
		Role  domain.OrgRole `json:"role"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	res, err := h.orgs.Invite(c.Request().Context(), currentUser(c).ID, id, in.Email, in.Role)
	if err != nil {
		return err
	}
	status := "invited"
	if res.Added {
		status = "added"
	}
	return c.JSON(http.StatusCreated, map[string]string{"status": status})
}

func (h *OrgHandler) revokeInvitation(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	invID, err := pathID(c, "invitationId")
	if err != nil {
		return err
	}
	if err := h.orgs.RevokeInvitation(c.Request().Context(), currentUser(c).ID, id, invID); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
