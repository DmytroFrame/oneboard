package httpapi

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"

	"github.com/DmytroFrame/oneboard/backend/internal/usecase/project"
)

func init() { registerRoutes(NewProjectHandler) }

type ProjectHandler struct {
	projects *project.Service
	present  *Presenter
}

func NewProjectHandler(projects *project.Service, present *Presenter) *ProjectHandler {
	return &ProjectHandler{projects: projects, present: present}
}

func (h *ProjectHandler) Mount(_, private *echo.Group) {
	private.GET("/projects", h.list)
	private.POST("/projects", h.create)
	private.PUT("/projects/:id", h.rename)
	private.DELETE("/projects/:id", h.delete)
	private.GET("/projects/:id/members", h.members)
	private.POST("/projects/:id/members", h.addMember)
	private.DELETE("/projects/:id/members/:userId", h.removeMember)
	private.PUT("/projects/:id/members/:userId", h.setMemberRole)
	private.PUT("/projects/:id/settings", h.updateSettings)
	private.PUT("/projects/:id/pin", h.pin)
}

func (h *ProjectHandler) updateSettings(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var in projectSettingsDTO
	if err := bind(c, &in); err != nil {
		return err
	}
	p, err := h.projects.UpdateSettings(c.Request().Context(), currentUser(c), id, domain.ProjectSettings(in))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, toProject(p))
}

func (h *ProjectHandler) pin(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var in struct {
		Pinned bool `json:"pinned"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	if err := h.projects.SetPinned(c.Request().Context(), currentUser(c), id, in.Pinned); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *ProjectHandler) setMemberRole(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	userID, err := pathID(c, "userId")
	if err != nil {
		return err
	}
	var in struct {
		Role domain.Role `json:"role"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	ms, err := h.projects.SetMemberRole(c.Request().Context(), currentUser(c), id, userID, in.Role)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, h.present.Members(ms))
}

type projectInput struct {
	OrganizationID int64  `json:"organizationId"` // create only
	Name           string `json:"name"`
	Key            string `json:"key"`
}

// list: the user's projects in the current organization; ?all=1 adds every other project there for
// those with viewAllProjects.
func (h *ProjectHandler) list(c *echo.Context) error {
	if c.QueryParam("all") == "1" {
		ps, err := h.projects.ListAll(c.Request().Context(), currentUser(c))
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, mapSlice(ps, toProject))
	}
	ps, err := h.projects.List(c.Request().Context(), currentUser(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, mapSlice(ps, toProject))
}

func (h *ProjectHandler) create(c *echo.Context) error {
	var in projectInput
	if err := bind(c, &in); err != nil {
		return err
	}
	p, err := h.projects.Create(c.Request().Context(), currentUser(c).ID, in.OrganizationID, in.Name, in.Key)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, toProject(p))
}

func (h *ProjectHandler) rename(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var in projectInput
	if err := bind(c, &in); err != nil {
		return err
	}
	p, err := h.projects.Rename(c.Request().Context(), currentUser(c).ID, id, in.Name)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, toProject(p))
}

func (h *ProjectHandler) delete(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	if err := h.projects.Delete(c.Request().Context(), currentUser(c).ID, id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *ProjectHandler) members(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	ms, err := h.projects.Members(c.Request().Context(), currentUser(c).ID, id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, h.present.Members(ms))
}

func (h *ProjectHandler) addMember(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var in struct {
		UserID int64 `json:"userId"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	ms, err := h.projects.AddMember(c.Request().Context(), currentUser(c).ID, id, in.UserID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, h.present.Members(ms))
}

func (h *ProjectHandler) removeMember(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	target, err := pathID(c, "userId")
	if err != nil {
		return err
	}
	if err := h.projects.RemoveMember(c.Request().Context(), currentUser(c).ID, id, target); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
