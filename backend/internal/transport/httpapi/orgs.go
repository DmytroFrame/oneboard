package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/organization"
)

type OrgHandler struct {
	orgs    *organization.Service
	present *Presenter
}

func NewOrgHandler(orgs *organization.Service, present *Presenter) *OrgHandler {
	return &OrgHandler{orgs: orgs, present: present}
}

func (h *OrgHandler) Mount(_, private chi.Router) {
	private.Get("/organizations", h.list)
	private.Post("/organizations", h.create)
	private.Get("/organizations/{id}", h.get)
	private.Put("/organizations/{id}", h.rename)
	private.Delete("/organizations/{id}", h.delete)
	private.Get("/organizations/{id}/members", h.members)
	private.Put("/organizations/{id}/members/{userId}", h.changeRole)
	private.Delete("/organizations/{id}/members/{userId}", h.removeMember)
	private.Get("/organizations/{id}/invitations", h.invitations)
	private.Post("/organizations/{id}/invitations", h.invite)
	private.Delete("/organizations/{id}/invitations/{invitationId}", h.revokeInvitation)
}

func (h *OrgHandler) list(w http.ResponseWriter, r *http.Request) {
	orgs, err := h.orgs.List(r.Context(), currentUser(r).ID)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, mapSlice(orgs, toOrg))
}

type orgInput struct {
	Name string `json:"name"`
}

func (h *OrgHandler) create(w http.ResponseWriter, r *http.Request) {
	var in orgInput
	if !decode(w, r, &in) {
		return
	}
	o, err := h.orgs.Create(r.Context(), currentUser(r).ID, in.Name)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toOrg(o))
}

func (h *OrgHandler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	o, err := h.orgs.Get(r.Context(), currentUser(r).ID, id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toOrg(o))
}

func (h *OrgHandler) rename(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in orgInput
	if !decode(w, r, &in) {
		return
	}
	o, err := h.orgs.Rename(r.Context(), currentUser(r).ID, id, in.Name)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toOrg(o))
}

func (h *OrgHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := h.orgs.Delete(r.Context(), currentUser(r).ID, id); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *OrgHandler) members(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	ms, err := h.orgs.Members(r.Context(), currentUser(r).ID, id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, h.present.OrgMembers(ms))
}

func (h *OrgHandler) changeRole(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	target, ok := pathID(w, r, "userId")
	if !ok {
		return
	}
	var in struct {
		Role domain.OrgRole `json:"role"`
	}
	if !decode(w, r, &in) {
		return
	}
	ms, err := h.orgs.ChangeRole(r.Context(), currentUser(r).ID, id, target, in.Role)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, h.present.OrgMembers(ms))
}

// removeMember removes someone, or lets the caller leave when userId is their own id.
func (h *OrgHandler) removeMember(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	target, ok := pathID(w, r, "userId")
	if !ok {
		return
	}
	if err := h.orgs.RemoveMember(r.Context(), currentUser(r).ID, id, target); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *OrgHandler) invitations(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	invs, err := h.orgs.Invitations(r.Context(), currentUser(r).ID, id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, mapSlice(invs, toInvitation))
}

// invite adds a registered user right away, or keeps an invitation that is accepted on sign-up.
// Responds with {"status": "added" | "invited"}.
func (h *OrgHandler) invite(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in struct {
		Email string         `json:"email"`
		Role  domain.OrgRole `json:"role"`
	}
	if !decode(w, r, &in) {
		return
	}
	res, err := h.orgs.Invite(r.Context(), currentUser(r).ID, id, in.Email, in.Role)
	if err != nil {
		fail(w, r, err)
		return
	}
	status := "invited"
	if res.Added {
		status = "added"
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": status})
}

func (h *OrgHandler) revokeInvitation(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	invID, ok := pathID(w, r, "invitationId")
	if !ok {
		return
	}
	if err := h.orgs.RevokeInvitation(r.Context(), currentUser(r).ID, id, invID); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
