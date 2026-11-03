package httpapi

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/wiki"
)

func init() { registerRoutes(NewWikiHandler) }

// WikiHandler serves the wiki (articles, access, versions, groups) and project comments.
type WikiHandler struct {
	wiki     *wiki.Service
	comments *wiki.CommentService
	present  *Presenter
}

func NewWikiHandler(w *wiki.Service, c *wiki.CommentService, present *Presenter) *WikiHandler {
	return &WikiHandler{wiki: w, comments: c, present: present}
}

func (h *WikiHandler) Mount(_, private *echo.Group) {
	private.GET("/wiki/articles", h.list)
	private.GET("/wiki/home", h.home)
	private.GET("/wiki/search", h.search)
	private.POST("/wiki/articles", h.create)
	private.GET("/wiki/articles/:id", h.get)
	private.PUT("/wiki/articles/:id", h.update)
	private.PUT("/wiki/articles/:id/archived", h.archive)
	private.GET("/wiki/articles/:id/access", h.access)
	private.PUT("/wiki/articles/:id/access", h.setAccess)
	private.GET("/wiki/articles/:id/versions", h.versions)
	private.GET("/wiki/versions/:id", h.version)
	private.GET("/projects/:id/wiki", h.projectArticles)

	private.GET("/wiki/groups", h.groups)
	private.POST("/wiki/groups", h.createGroup)
	private.PUT("/wiki/groups/:id", h.updateGroup)
	private.GET("/wiki/groups/:id/usage", h.groupUsage)
	private.DELETE("/wiki/groups/:id", h.deleteGroup)

	private.GET("/projects/:id/comments", h.listComments)
	private.POST("/projects/:id/comments", h.createComment)
	private.PUT("/project-comments/:id", h.updateComment)
	private.DELETE("/project-comments/:id", h.deleteComment)
}

func (h *WikiHandler) items(c *echo.Context, items []wiki.Item, err error) error {
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, mapSlice(items, wikiItemDTOOf))
}

func (h *WikiHandler) list(c *echo.Context) error {
	items, err := h.wiki.List(c.Request().Context(), currentUser(c))
	return h.items(c, items, err)
}

func (h *WikiHandler) search(c *echo.Context) error {
	items, err := h.wiki.Search(c.Request().Context(), currentUser(c), c.QueryParam("q"))
	return h.items(c, items, err)
}

func (h *WikiHandler) projectArticles(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	items, err := h.wiki.ProjectArticles(c.Request().Context(), currentUser(c), id)
	return h.items(c, items, err)
}

func (h *WikiHandler) home(c *echo.Context) error {
	recent, changed, err := h.wiki.Home(c.Request().Context(), currentUser(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"recent": mapSlice(recent, wikiItemDTOOf), "changed": mapSlice(changed, wikiItemDTOOf)})
}

func (h *WikiHandler) page(c *echo.Context, p wiki.Page, err error, status int) error {
	if err != nil {
		return err
	}
	return c.JSON(status, wikiPageDTOOf(p))
}

func (h *WikiHandler) get(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	p, err := h.wiki.Get(c.Request().Context(), currentUser(c), id)
	return h.page(c, p, err, http.StatusOK)
}

func (h *WikiHandler) create(c *echo.Context) error {
	var in wikiDraftInput
	if err := bind(c, &in); err != nil {
		return err
	}
	p, err := h.wiki.Create(c.Request().Context(), currentUser(c), in.draft())
	return h.page(c, p, err, http.StatusCreated)
}

func (h *WikiHandler) update(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var in wikiDraftInput
	if err := bind(c, &in); err != nil {
		return err
	}
	p, err := h.wiki.Update(c.Request().Context(), currentUser(c), id, in.draft())
	return h.page(c, p, err, http.StatusOK)
}

func (h *WikiHandler) archive(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var in struct {
		Archived bool `json:"archived"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	p, err := h.wiki.SetArchived(c.Request().Context(), currentUser(c), id, in.Archived)
	return h.page(c, p, err, http.StatusOK)
}

func (h *WikiHandler) access(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	a, err := h.wiki.Access(c.Request().Context(), currentUser(c), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, wikiAccessDTOOf(a))
}

func (h *WikiHandler) setAccess(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var in wikiAccessDTO
	if err := bind(c, &in); err != nil {
		return err
	}
	p, err := h.wiki.SetAccess(c.Request().Context(), currentUser(c), id, in.access())
	return h.page(c, p, err, http.StatusOK)
}

func (h *WikiHandler) versions(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	vs, err := h.wiki.Versions(c.Request().Context(), currentUser(c), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, mapSlice(vs, wikiVersionDTOOf))
}

func (h *WikiHandler) version(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	v, err := h.wiki.Version(c.Request().Context(), currentUser(c), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, wikiVersionDTOOf(v))
}

func (h *WikiHandler) groups(c *echo.Context) error {
	gs, err := h.wiki.Groups(c.Request().Context(), currentUser(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, mapSlice(gs, wikiGroupDTOOf))
}

func (h *WikiHandler) createGroup(c *echo.Context) error {
	var in wikiGroupDTO
	if err := bind(c, &in); err != nil {
		return err
	}
	g, err := h.wiki.CreateGroup(c.Request().Context(), currentUser(c), in.group())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, wikiGroupDTOOf(g))
}

func (h *WikiHandler) updateGroup(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var in wikiGroupDTO
	if err := bind(c, &in); err != nil {
		return err
	}
	g, err := h.wiki.UpdateGroup(c.Request().Context(), currentUser(c), id, in.group())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, wikiGroupDTOOf(g))
}

func (h *WikiHandler) groupUsage(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	n, err := h.wiki.GroupUsage(c.Request().Context(), currentUser(c), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]int{"articles": n})
}

func (h *WikiHandler) deleteGroup(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	if err := h.wiki.DeleteGroup(c.Request().Context(), currentUser(c), id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *WikiHandler) listComments(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	list, err := h.comments.List(c.Request().Context(), currentUser(c), id, domain.ProjectCommentCategory(c.QueryParam("category")))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, mapSlice(list, projectCommentDTOOf))
}

func (h *WikiHandler) createComment(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var in projectCommentInput
	if err := bind(c, &in); err != nil {
		return err
	}
	pc, err := h.comments.Create(c.Request().Context(), currentUser(c), id, in.comment())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, projectCommentDTOOf(pc))
}

func (h *WikiHandler) updateComment(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var in projectCommentInput
	if err := bind(c, &in); err != nil {
		return err
	}
	pc, err := h.comments.Update(c.Request().Context(), currentUser(c), id, in.comment())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, projectCommentDTOOf(pc))
}

func (h *WikiHandler) deleteComment(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	if err := h.comments.Delete(c.Request().Context(), currentUser(c), id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
