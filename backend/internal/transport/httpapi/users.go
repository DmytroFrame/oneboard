package httpapi

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"clodapp/backend/internal/domain"
	"clodapp/backend/internal/usecase/user"
)

type UserHandler struct {
	users *user.Service
}

func NewUserHandler(users *user.Service) *UserHandler {
	return &UserHandler{users: users}
}

func (h *UserHandler) Mount(_, private chi.Router) {
	private.Get("/users", h.list)
	private.Put("/users/me/settings", h.updateSettings)
	private.Put("/users/me/avatar", h.uploadAvatar)
	private.Delete("/users/me/avatar", h.removeAvatar)
	private.Get("/users/{id}/avatar", h.avatar)
}

// list returns every registered user; used to pick people to add to a project.
func (h *UserHandler) list(w http.ResponseWriter, r *http.Request) {
	users, err := h.users.List(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, mapSlice(users, toUser))
}

// uploadAvatar accepts multipart/form-data with the image in the "avatar" field.
func (h *UserHandler) uploadAvatar(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, user.MaxAvatarBytes+64<<10) // room for multipart headers
	file, _, err := r.FormFile("avatar")
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			fail(w, r, domain.ErrAvatarTooLarge)
			return
		}
		fail(w, r, domain.ErrAvatarMissing)
		return
	}
	defer file.Close()

	u, err := h.users.UploadAvatar(r.Context(), currentUser(r).ID, file)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toMe(u))
}

func (h *UserHandler) removeAvatar(w http.ResponseWriter, r *http.Request) {
	u, err := h.users.RemoveAvatar(r.Context(), currentUser(r).ID)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toMe(u))
}

func (h *UserHandler) updateSettings(w http.ResponseWriter, r *http.Request) {
	var in settingsDTO
	if !decode(w, r, &in) {
		return
	}
	u, err := h.users.UpdateSettings(r.Context(), currentUser(r).ID, domain.Settings{Language: in.Language, Theme: in.Theme})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toMe(u))
}

// avatar streams the image from object storage. URLs carry ?v=<version>, so it is cached for good.
func (h *UserHandler) avatar(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	body, info, err := h.users.Avatar(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	defer body.Close()

	w.Header().Set("Content-Type", info.ContentType)
	if info.Size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
	}
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = io.Copy(w, body)
}
