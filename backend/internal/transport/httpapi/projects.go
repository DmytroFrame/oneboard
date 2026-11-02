package httpapi

import (
	"net/http"

	"github.com/labstack/echo/v5"

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
}

type projectInput struct {
	OrganizationID int64  `json:"organizationId"` // create only
	Name           string `json:"name"`
	Key            string `json:"key"`
}

func (h *ProjectHandler) list(c *echo.Context) error {
	ps, err := h.projects.List(c.Request().Context(), currentUser(c).ID)
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
