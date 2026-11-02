package httpapi

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/board"
)

func init() { registerRoutes(NewColumnHandler) }

type ColumnHandler struct {
	columns *board.ColumnService
}

func NewColumnHandler(columns *board.ColumnService) *ColumnHandler {
	return &ColumnHandler{columns: columns}
}

func (h *ColumnHandler) Mount(_, private *echo.Group) {
	private.GET("/projects/:id/columns", h.list)
	private.POST("/projects/:id/columns", h.create)
	private.PUT("/columns/:id", h.rename)
	private.POST("/columns/:id/move", h.move)
	private.DELETE("/columns/:id", h.delete)
}

type columnInput struct {
	Name string `json:"name"`
}

func respondColumns(c *echo.Context, status int, cols []domain.Column, err error) error {
	if err != nil {
		return err
	}
	return c.JSON(status, toColumns(cols))
}

func (h *ColumnHandler) list(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	cols, err := h.columns.List(c.Request().Context(), currentUser(c).ID, id)
	return respondColumns(c, http.StatusOK, cols, err)
}

func (h *ColumnHandler) create(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var in columnInput
	if err := bind(c, &in); err != nil {
		return err
	}
	cols, err := h.columns.Create(c.Request().Context(), currentUser(c).ID, id, in.Name)
	return respondColumns(c, http.StatusCreated, cols, err)
}

func (h *ColumnHandler) rename(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var in columnInput
	if err := bind(c, &in); err != nil {
		return err
	}
	cols, err := h.columns.Rename(c.Request().Context(), currentUser(c).ID, id, in.Name)
	return respondColumns(c, http.StatusOK, cols, err)
}

func (h *ColumnHandler) move(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var in struct {
		Index int `json:"index"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	cols, err := h.columns.Move(c.Request().Context(), currentUser(c).ID, id, in.Index)
	return respondColumns(c, http.StatusOK, cols, err)
}

// delete takes ?moveTo=<columnId>, required when the column still has cards.
func (h *ColumnHandler) delete(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var moveTo *int64
	if v, err := strconv.ParseInt(c.QueryParam("moveTo"), 10, 64); err == nil {
		moveTo = &v
	}
	cols, err := h.columns.Delete(c.Request().Context(), currentUser(c).ID, id, moveTo)
	return respondColumns(c, http.StatusOK, cols, err)
}
