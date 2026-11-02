package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/board"
)

func init() { registerRoutes(NewCardHandler) }

type CardHandler struct {
	cards   *board.CardService
	present *Presenter
}

func NewCardHandler(cards *board.CardService, present *Presenter) *CardHandler {
	return &CardHandler{cards: cards, present: present}
}

func (h *CardHandler) Mount(_, private *echo.Group) {
	private.GET("/projects/:id/cards", h.list)
	private.POST("/projects/:id/cards", h.create)
	private.GET("/cards/:id", h.get)
	private.PUT("/cards/:id", h.update)
	private.POST("/cards/:id/move", h.move)
	private.DELETE("/cards/:id", h.delete)
}

var (
	errColumnRequired = &apiError{http.StatusBadRequest, "column_required", "The column parameter is required"}
	errCursorInvalid  = &apiError{http.StatusBadRequest, "cursor_invalid", "Invalid cursor"}
)

// list returns one page of a column's cards, ordered by position.
//
//	?column=<id>       required
//	?limit=<n>         page size (default 20, max 100)
//	?cursor=<pos:id>   nextCursor from the previous page
//	?q=<text>          full-text search (OpenSearch) in key, title and description
//	?assignees=1,2     only these assignees
//	?unassigned=1      include unassigned cards (OR-ed with assignees)
func (h *CardHandler) list(c *echo.Context) error {
	projectID, err := pathID(c, "id")
	if err != nil {
		return err
	}
	columnID, err := strconv.ParseInt(c.QueryParam("column"), 10, 64)
	if err != nil {
		return errColumnRequired
	}
	query := board.ListQuery{
		ColumnID:   columnID,
		Search:     c.QueryParam("q"),
		Unassigned: c.QueryParam("unassigned") == "1",
	}
	query.Limit, _ = strconv.Atoi(c.QueryParam("limit"))
	for _, part := range strings.Split(c.QueryParam("assignees"), ",") {
		if id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64); err == nil {
			query.Assignees = append(query.Assignees, id)
		}
	}
	if cursor := c.QueryParam("cursor"); cursor != "" {
		var cur domain.CardCursor
		if _, err := fmt.Sscanf(cursor, "%d:%d", &cur.Position, &cur.ID); err != nil {
			return errCursorInvalid
		}
		query.Cursor = &cur
	}

	page, err := h.cards.List(c.Request().Context(), currentUser(c).ID, projectID, query)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, h.present.CardPage(page))
}

type cardInput struct {
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Priority    domain.Priority `json:"priority"`
	AssigneeID  *int64          `json:"assigneeId"`
	ColumnID    int64           `json:"columnId"` // create only
}

func (in cardInput) draft() domain.CardDraft {
	return domain.CardDraft{Title: in.Title, Description: in.Description, Priority: in.Priority, AssigneeID: in.AssigneeID}
}

func (h *CardHandler) create(c *echo.Context) error {
	projectID, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var in cardInput
	if err := bind(c, &in); err != nil {
		return err
	}
	card, err := h.cards.Create(c.Request().Context(), currentUser(c).ID, projectID, in.ColumnID, in.draft())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, h.present.Card(card))
}

func (h *CardHandler) get(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	card, err := h.cards.Get(c.Request().Context(), currentUser(c).ID, id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, h.present.Card(card))
}

func (h *CardHandler) update(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var in cardInput
	if err := bind(c, &in); err != nil {
		return err
	}
	card, err := h.cards.Update(c.Request().Context(), currentUser(c).ID, id, in.draft())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, h.present.Card(card))
}

func (h *CardHandler) move(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var in struct {
		ColumnID int64  `json:"columnId"`
		BeforeID *int64 `json:"beforeId"`
		AfterID  *int64 `json:"afterId"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	card, err := h.cards.Move(c.Request().Context(), currentUser(c).ID, id, in.ColumnID, board.Placement{BeforeID: in.BeforeID, AfterID: in.AfterID})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, h.present.Card(card))
}

func (h *CardHandler) delete(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	if err := h.cards.Delete(c.Request().Context(), currentUser(c).ID, id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
