package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"clodapp/backend/internal/usecase/project"
)

type ProjectHandler struct {
	projects *project.Service
	present  *Presenter
}

func NewProjectHandler(projects *project.Service, present *Presenter) *ProjectHandler {
	return &ProjectHandler{projects: projects, present: present}
}

func (h *ProjectHandler) Mount(_, private chi.Router) {
	private.Get("/projects", h.list)
	private.Post("/projects", h.create)
	private.Put("/projects/{id}", h.rename)
	private.Delete("/projects/{id}", h.delete)
	private.Get("/projects/{id}/members", h.members)
	private.Post("/projects/{id}/members", h.addMember)
	private.Delete("/projects/{id}/members/{userId}", h.removeMember)
}

type projectInput struct {
	OrganizationID int64  `json:"organizationId"` // create only
	Name           string `json:"name"`
	Key            string `json:"key"`
}

func (h *ProjectHandler) list(w http.ResponseWriter, r *http.Request) {
	ps, err := h.projects.List(r.Context(), currentUser(r).ID)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, mapSlice(ps, toProject))
}

func (h *ProjectHandler) create(w http.ResponseWriter, r *http.Request) {
	var in projectInput
	if !decode(w, r, &in) {
		return
	}
	p, err := h.projects.Create(r.Context(), currentUser(r).ID, in.OrganizationID, in.Name, in.Key)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toProject(p))
}

func (h *ProjectHandler) rename(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in projectInput
	if !decode(w, r, &in) {
		return
	}
	p, err := h.projects.Rename(r.Context(), currentUser(r).ID, id, in.Name)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toProject(p))
}

func (h *ProjectHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := h.projects.Delete(r.Context(), currentUser(r).ID, id); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProjectHandler) members(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	ms, err := h.projects.Members(r.Context(), currentUser(r).ID, id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, h.present.Members(ms))
}

func (h *ProjectHandler) addMember(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in struct {
		UserID int64 `json:"userId"`
	}
	if !decode(w, r, &in) {
		return
	}
	ms, err := h.projects.AddMember(r.Context(), currentUser(r).ID, id, in.UserID)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, h.present.Members(ms))
}

func (h *ProjectHandler) removeMember(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	target, ok := pathID(w, r, "userId")
	if !ok {
		return
	}
	if err := h.projects.RemoveMember(r.Context(), currentUser(r).ID, id, target); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
