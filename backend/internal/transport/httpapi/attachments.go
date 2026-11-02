package httpapi

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/attachment"
)

func init() { registerRoutes(NewAttachmentHandler) }

type AttachmentHandler struct {
	attachments *attachment.Service
	present     *Presenter
}

func NewAttachmentHandler(a *attachment.Service, present *Presenter) *AttachmentHandler {
	return &AttachmentHandler{attachments: a, present: present}
}

func (h *AttachmentHandler) Mount(public, private *echo.Group) {
	private.GET("/cards/:id/attachments", h.list)
	private.POST("/cards/:id/attachments", h.upload)
	private.DELETE("/attachments/:id", h.delete)
	// Authorised by the signed, expiring URL — see Presenter. :variant is "file" or "thumb".
	public.GET("/attachments/:id/:variant", h.serve)
}

func (h *AttachmentHandler) list(c *echo.Context) error {
	cardID, err := pathID(c, "id")
	if err != nil {
		return err
	}
	as, err := h.attachments.List(c.Request().Context(), currentUser(c).ID, cardID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, mapSlice(as, h.present.Attachment))
}

// upload takes multipart/form-data with the file in the "file" field.
func (h *AttachmentHandler) upload(c *echo.Context) error {
	cardID, err := pathID(c, "id")
	if err != nil {
		return err
	}
	r := c.Request()
	r.Body = http.MaxBytesReader(c.Response(), r.Body, domain.MaxAttachmentBytes+1<<20) // room for multipart headers
	// Up to 8 MB stays in memory; larger files are spooled to a temp file by the multipart parser.
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return domain.ErrAttachmentTooLarge
		}
		return domain.ErrAttachmentMissing
	}
	defer r.MultipartForm.RemoveAll()
	file, header, err := r.FormFile("file")
	if err != nil {
		return domain.ErrAttachmentMissing
	}
	defer file.Close()

	a, err := h.attachments.Upload(r.Context(), currentUser(c).ID, cardID, attachment.Upload{
		FileName: header.Filename,
		Size:     header.Size,
		Body:     file,
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, h.present.Attachment(a))
}

func (h *AttachmentHandler) delete(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	if _, err := h.attachments.Delete(c.Request().Context(), currentUser(c).ID, id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// serve streams a file. Images and videos are shown inline (with Range support for seeking);
// anything else is forced to download so an uploaded HTML/SVG can never run on our origin.
func (h *AttachmentHandler) serve(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	variant := c.Param("variant")
	if variant != "file" && variant != "thumb" {
		return errNotFound
	}
	if !h.present.validAttachmentSig(id, variant, c.QueryParam("exp"), c.QueryParam("sig")) {
		return domain.ErrAttachmentNotFound
	}

	a, obj, err := h.attachments.Open(c.Request().Context(), id, attachment.Variant(variant), c.Request().Header.Get("Range"))
	if errors.Is(err, attachment.ErrInvalidRange) {
		return c.NoContent(http.StatusRequestedRangeNotSatisfiable)
	}
	if err != nil {
		return err
	}
	defer obj.Body.Close()

	contentType, disposition := "application/octet-stream", "attachment"
	switch {
	case variant == "thumb":
		contentType, disposition = "image/jpeg", "inline"
	case a.Kind != domain.AttachmentFile:
		contentType, disposition = a.ContentType, "inline"
	}
	if c.QueryParam("download") == "1" {
		disposition = "attachment"
	}

	w := c.Response()
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
	return nil
}
