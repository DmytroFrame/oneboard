package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"clodapp/backend/internal/domain"
	"clodapp/backend/internal/usecase/board"
)

type CardHandler struct {
	cards   *board.CardService
	present *Presenter
}

func NewCardHandler(cards *board.CardService, present *Presenter) *CardHandler {
	return &CardHandler{cards: cards, present: present}
}

func (h *CardHandler) Mount(_, private chi.Router) {
	private.Get("/projects/{id}/cards", h.list)
	private.Post("/projects/{id}/cards", h.create)
	private.Put("/cards/{id}", h.update)
	private.Post("/cards/{id}/move", h.move)
	private.Delete("/cards/{id}", h.delete)
}

// list returns one page of a column's cards, ordered by position.
//
//	?column=<id>       required
//	?limit=<n>         page size (default 20, max 100)
//	?cursor=<pos:id>   nextCursor from the previous page
//	?q=<text>          full-text search (OpenSearch) in key, title and description
//	?assignees=1,2     only these assignees
//	?unassigned=1      include unassigned cards (OR-ed with assignees)
func (h *CardHandler) list(w http.ResponseWriter, r *http.Request) {
	projectID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	q := r.URL.Query()
	columnID, err := strconv.ParseInt(q.Get("column"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "column_required", "The column parameter is required")
		return
	}
	query := board.ListQuery{
		ColumnID:   columnID,
		Search:     q.Get("q"),
		Unassigned: q.Get("unassigned") == "1",
	}
	query.Limit, _ = strconv.Atoi(q.Get("limit"))
	for _, part := range strings.Split(q.Get("assignees"), ",") {
		if id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64); err == nil {
			query.Assignees = append(query.Assignees, id)
		}
	}
	if c := q.Get("cursor"); c != "" {
		var cur domain.CardCursor
		if _, err := fmt.Sscanf(c, "%d:%d", &cur.Position, &cur.ID); err != nil {
			writeError(w, http.StatusBadRequest, "cursor_invalid", "Invalid cursor")
			return
		}
		query.Cursor = &cur
	}

	page, err := h.cards.List(r.Context(), currentUser(r).ID, projectID, query)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, h.present.CardPage(page))
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

func (h *CardHandler) create(w http.ResponseWriter, r *http.Request) {
	projectID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in cardInput
	if !decode(w, r, &in) {
		return
	}
	c, err := h.cards.Create(r.Context(), currentUser(r).ID, projectID, in.ColumnID, in.draft())
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, h.present.Card(c))
}

func (h *CardHandler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in cardInput
	if !decode(w, r, &in) {
		return
	}
	c, err := h.cards.Update(r.Context(), currentUser(r).ID, id, in.draft())
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, h.present.Card(c))
}

func (h *CardHandler) move(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in struct {
		ColumnID int64  `json:"columnId"`
		BeforeID *int64 `json:"beforeId"`
		AfterID  *int64 `json:"afterId"`
	}
	if !decode(w, r, &in) {
		return
	}
	c, err := h.cards.Move(r.Context(), currentUser(r).ID, id, in.ColumnID, board.Placement{BeforeID: in.BeforeID, AfterID: in.AfterID})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, h.present.Card(c))
}

func (h *CardHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := h.cards.Delete(r.Context(), currentUser(r).ID, id); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
