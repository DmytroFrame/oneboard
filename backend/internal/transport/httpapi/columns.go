package httpapi

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"clodapp/backend/internal/domain"
	"clodapp/backend/internal/usecase/board"
)

type ColumnHandler struct {
	columns *board.ColumnService
}

func NewColumnHandler(columns *board.ColumnService) *ColumnHandler {
	return &ColumnHandler{columns: columns}
}

func (h *ColumnHandler) Mount(_, private chi.Router) {
	private.Get("/projects/{id}/columns", h.list)
	private.Post("/projects/{id}/columns", h.create)
	private.Put("/columns/{id}", h.rename)
	private.Post("/columns/{id}/move", h.move)
	private.Delete("/columns/{id}", h.delete)
}

type columnInput struct {
	Name string `json:"name"`
}

func (h *ColumnHandler) respond(w http.ResponseWriter, r *http.Request, status int, cols []domain.Column, err error) {
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, status, toColumns(cols))
}

func (h *ColumnHandler) list(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	cols, err := h.columns.List(r.Context(), currentUser(r).ID, id)
	h.respond(w, r, http.StatusOK, cols, err)
}

func (h *ColumnHandler) create(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in columnInput
	if !decode(w, r, &in) {
		return
	}
	cols, err := h.columns.Create(r.Context(), currentUser(r).ID, id, in.Name)
	h.respond(w, r, http.StatusCreated, cols, err)
}

func (h *ColumnHandler) rename(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in columnInput
	if !decode(w, r, &in) {
		return
	}
	cols, err := h.columns.Rename(r.Context(), currentUser(r).ID, id, in.Name)
	h.respond(w, r, http.StatusOK, cols, err)
}

func (h *ColumnHandler) move(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in struct {
		Index int `json:"index"`
	}
	if !decode(w, r, &in) {
		return
	}
	cols, err := h.columns.Move(r.Context(), currentUser(r).ID, id, in.Index)
	h.respond(w, r, http.StatusOK, cols, err)
}

// delete takes ?moveTo=<columnId>, required when the column still has cards.
func (h *ColumnHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var moveTo *int64
	if v, err := strconv.ParseInt(r.URL.Query().Get("moveTo"), 10, 64); err == nil {
		moveTo = &v
	}
	cols, err := h.columns.Delete(r.Context(), currentUser(r).ID, id, moveTo)
	h.respond(w, r, http.StatusOK, cols, err)
}
