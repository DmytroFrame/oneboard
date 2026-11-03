package httpapi

import (
	"fmt"
	"io"
	"net/http"
	"path"

	"github.com/labstack/echo/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/backwoods"
)

func init() { registerRoutes(NewBackwoodsHandler) }

// BackwoodsHandler serves the admin panel (OWNER and ADMIN).
type BackwoodsHandler struct {
	backwoods *backwoods.Service
}

func NewBackwoodsHandler(b *backwoods.Service) *BackwoodsHandler {
	return &BackwoodsHandler{backwoods: b}
}

func (h *BackwoodsHandler) Mount(_, private *echo.Group) {
	private.GET("/backwoods/backups", h.backups)
	private.POST("/backwoods/backups", h.backup)
	private.GET("/backwoods/backups/:id/download", h.download)
	private.GET("/backwoods/schema", h.schema)
	private.GET("/backwoods/versions", h.versions)
	private.GET("/backwoods/special-tasks", h.specialTasks)
	private.GET("/backwoods/project-comments", h.commentedProjects)
}

func (h *BackwoodsHandler) backups(c *echo.Context) error {
	st, err := h.backwoods.Backups(c.Request().Context(), currentUser(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, backupStateDTOOf(st))
}

func (h *BackwoodsHandler) backup(c *echo.Context) error {
	st, err := h.backwoods.Backup(c.Request().Context(), currentUser(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, backupStateDTOOf(st))
}

func (h *BackwoodsHandler) download(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	b, body, err := h.backwoods.Download(c.Request().Context(), currentUser(c), id)
	if err != nil {
		return err
	}
	defer body.Close()
	res := c.Response()
	res.Header().Set(echo.HeaderContentType, "application/gzip")
	res.Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`attachment; filename=%q`, "oneboard-"+path.Base(b.Key)))
	res.Header().Set(echo.HeaderContentLength, fmt.Sprint(b.Size))
	res.WriteHeader(http.StatusOK)
	_, err = io.Copy(res, body)
	return err
}

func (h *BackwoodsHandler) schema(c *echo.Context) error {
	s, err := h.backwoods.Schema(c.Request().Context(), currentUser(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, s) // domain.DBSchema carries its own JSON names: no values, nothing to hide
}

func (h *BackwoodsHandler) versions(c *echo.Context) error {
	vs, err := h.backwoods.Versions(c.Request().Context(), currentUser(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, mapSlice(vs, func(v domain.RuntimeVersion) map[string]string {
		return map[string]string{"name": v.Name, "version": v.Version}
	}))
}

func (h *BackwoodsHandler) specialTasks(c *echo.Context) error {
	fs, err := h.backwoods.SpecialTaskProjects(c.Request().Context(), currentUser(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, mapSlice(nonNil(fs), projectFindingDTOOf))
}

func (h *BackwoodsHandler) commentedProjects(c *echo.Context) error {
	fs, err := h.backwoods.CommentedProjects(c.Request().Context(), currentUser(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, mapSlice(nonNil(fs), projectFindingDTOOf))
}
