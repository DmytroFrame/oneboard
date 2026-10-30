package httpapi

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"clodapp/backend/internal/domain"
	"clodapp/backend/internal/usecase/attachment"
)

type AttachmentHandler struct {
	attachments *attachment.Service
	present     *Presenter
}

func NewAttachmentHandler(a *attachment.Service, present *Presenter) *AttachmentHandler {
	return &AttachmentHandler{attachments: a, present: present}
}

func (h *AttachmentHandler) Mount(public, private chi.Router) {
	private.Get("/cards/{id}/attachments", h.list)
	private.Post("/cards/{id}/attachments", h.upload)
	private.Delete("/attachments/{id}", h.delete)
	// Authorised by the signed, expiring URL — see Presenter.
	public.Get("/attachments/{id}/{variant:file|thumb}", h.serve)
}

func (h *AttachmentHandler) list(w http.ResponseWriter, r *http.Request) {
	cardID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	as, err := h.attachments.List(r.Context(), currentUser(r).ID, cardID)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, mapSlice(as, h.present.Attachment))
}

// upload takes multipart/form-data with the file in the "file" field.
func (h *AttachmentHandler) upload(w http.ResponseWriter, r *http.Request) {
	cardID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, domain.MaxAttachmentBytes+1<<20) // room for multipart headers
	// Up to 8 MB stays in memory; larger files are spooled to a temp file by the multipart parser.
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			fail(w, r, domain.ErrAttachmentTooLarge)
			return
		}
		fail(w, r, domain.ErrAttachmentMissing)
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, header, err := r.FormFile("file")
	if err != nil {
		fail(w, r, domain.ErrAttachmentMissing)
		return
	}
	defer file.Close()

	a, err := h.attachments.Upload(r.Context(), currentUser(r).ID, cardID, attachment.Upload{
		FileName: header.Filename,
		Size:     header.Size,
		Body:     file,
	})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, h.present.Attachment(a))
}

func (h *AttachmentHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if _, err := h.attachments.Delete(r.Context(), currentUser(r).ID, id); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// serve streams a file. Images and videos are shown inline (with Range support for seeking);
// anything else is forced to download so an uploaded HTML/SVG can never run on our origin.
func (h *AttachmentHandler) serve(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	variant := chi.URLParam(r, "variant")
	q := r.URL.Query()
	if !h.present.validAttachmentSig(id, variant, q.Get("exp"), q.Get("sig")) {
		fail(w, r, domain.ErrAttachmentNotFound)
		return
	}

	a, obj, err := h.attachments.Open(r.Context(), id, attachment.Variant(variant), r.Header.Get("Range"))
	if errors.Is(err, attachment.ErrInvalidRange) {
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	defer obj.Body.Close()

	contentType, disposition := "application/octet-stream", "attachment"
	switch {
	case variant == "thumb":
		contentType, disposition = "image/jpeg", "inline"
	case a.Kind != domain.AttachmentFile:
		contentType, disposition = a.ContentType, "inline"
	}
	if q.Get("download") == "1" {
		disposition = "attachment"
	}

	hdr := w.Header()
	hdr.Set("Content-Type", contentType)
	hdr.Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": a.FileName}))
	hdr.Set("X-Content-Type-Options", "nosniff")
	hdr.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	hdr.Set("Cache-Control", "private, max-age=86400")
	hdr.Set("Accept-Ranges", "bytes")
	if obj.Size > 0 {
		hdr.Set("Content-Length", strconv.FormatInt(obj.Size, 10))
	}
	status := http.StatusOK
	if obj.ContentRange != "" {
		hdr.Set("Content-Range", obj.ContentRange)
		status = http.StatusPartialContent
	}
	w.WriteHeader(status)
	_, _ = io.Copy(w, obj.Body)
}
