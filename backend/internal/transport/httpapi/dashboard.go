package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"clodapp/backend/internal/usecase/dashboard"
)

type DashboardHandler struct {
	dashboard *dashboard.Service
	present   *Presenter
}

func NewDashboardHandler(d *dashboard.Service, present *Presenter) *DashboardHandler {
	return &DashboardHandler{dashboard: d, present: present}
}

func (h *DashboardHandler) Mount(_, private chi.Router) {
	private.Get("/dashboard", h.get)
}

func (h *DashboardHandler) get(w http.ResponseWriter, r *http.Request) {
	d, err := h.dashboard.Get(r.Context(), currentUser(r).ID)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, h.present.Dashboard(d))
}
