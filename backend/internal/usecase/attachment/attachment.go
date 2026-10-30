// Package attachment implements files attached to cards.
package attachment

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"go.uber.org/fx"

	"clodapp/backend/internal/domain"
	"clodapp/backend/internal/usecase"
)

var Module = fx.Module("attachment", fx.Provide(NewService))

type Attachments interface {
	ListForCard(ctx context.Context, cardID int64) ([]domain.Attachment, error)
	Get(ctx context.Context, attachmentID int64) (domain.Attachment, error) // includes ProjectID
	CountForCard(ctx context.Context, cardID int64) (int, error)
	Create(ctx context.Context, a *domain.Attachment) error // fills ID and CreatedAt
	Delete(ctx context.Context, attachmentID int64) error
}

type Cards interface {
	Get(ctx context.Context, cardID int64) (domain.Card, error)
}

// Object is an opened stored file (or a byte range of it).
type Object struct {
	Body         io.ReadCloser
	ContentType  string
	Size         int64  // bytes in Body
	ContentRange string // set when a range was requested, e.g. "bytes 0-99/1000"
}

// ErrInvalidRange is returned by Storage.Open when the requested byte range can't be satisfied.
var ErrInvalidRange = errors.New("invalid range")

type Storage interface {
	Put(ctx context.Context, key, contentType string, body io.ReadSeeker, size int64) error
	Open(ctx context.Context, key, byteRange string) (Object, error) // domain.ErrNoRows if missing
	DeleteMany(ctx context.Context, keys []string) error
}

// Thumbnailer makes a small JPEG preview of an image.
type Thumbnailer interface {
	Thumbnail(r io.Reader) ([]byte, error)
}

type Service struct {
	attachments Attachments
	cards       Cards
	members     usecase.Memberships
	storage     Storage
	thumbs      Thumbnailer
	log         *slog.Logger
}

func NewService(a Attachments, c Cards, m usecase.Memberships, s Storage, t Thumbnailer, log *slog.Logger) *Service {
	return &Service{attachments: a, cards: c, members: m, storage: s, thumbs: t, log: log}
}

// card loads a card and checks the user's access to its project.
func (s *Service) card(ctx context.Context, userID, cardID int64) (domain.Card, domain.Role, error) {
	c, err := s.cards.Get(ctx, cardID)
	if err != nil {
		return c, "", usecase.NotFoundAs(err, domain.ErrCardNotFound)
	}
	role, err := usecase.RequireMember(ctx, s.members, c.ProjectID, userID, domain.ErrCardNotFound)
	return c, role, err
}

func (s *Service) List(ctx context.Context, userID, cardID int64) ([]domain.Attachment, error) {
	if _, _, err := s.card(ctx, userID, cardID); err != nil {
		return nil, err
	}
	return s.attachments.ListForCard(ctx, cardID)
}

type Upload struct {
	FileName string
	Size     int64
	Body     io.ReadSeeker
}

func cleanFileName(name string) string {
	name = strings.TrimSpace(filepath.Base(strings.ReplaceAll(name, "\\", "/")))
	if name == "" || name == "." || name == "/" || !utf8.ValidString(name) {
		return "file"
	}
	if len(name) > 200 {
		ext := filepath.Ext(name)
		if len(ext) > 20 {
			ext = ""
		}
		name = strings.ToValidUTF8(name[:200-len(ext)], "") + ext
	}
	return name
}

// Upload stores a file on the card. The type is sniffed from the content, never taken from the client.
func (s *Service) Upload(ctx context.Context, userID, cardID int64, up Upload) (domain.Attachment, error) {
	if _, _, err := s.card(ctx, userID, cardID); err != nil {
		return domain.Attachment{}, err
	}
	switch {
	case up.Size <= 0:
		return domain.Attachment{}, domain.ErrAttachmentEmpty
	case up.Size > domain.MaxAttachmentBytes:
		return domain.Attachment{}, domain.ErrAttachmentTooLarge
	}
	n, err := s.attachments.CountForCard(ctx, cardID)
	if err != nil {
		return domain.Attachment{}, err
	}
	if n >= domain.MaxAttachmentsPerCard {
		return domain.Attachment{}, domain.ErrAttachmentLimit
	}

	head := make([]byte, 512)
	read, err := io.ReadFull(up.Body, head)
	if err != nil && err != io.ErrUnexpectedEOF {
		return domain.Attachment{}, err
	}
	contentType := http.DetectContentType(head[:read])
	if i := strings.IndexByte(contentType, ';'); i >= 0 {
		contentType = contentType[:i] // drop "; charset=…"
	}

	suffix := make([]byte, 12)
	if _, err := rand.Read(suffix); err != nil {
		return domain.Attachment{}, err
	}
	a := domain.Attachment{
		CardID:      cardID,
		UploaderID:  &userID,
		FileName:    cleanFileName(up.FileName),
		ContentType: contentType,
		Size:        up.Size,
		Kind:        domain.KindOf(contentType),
		StorageKey:  fmt.Sprintf("attachments/%d/%s", cardID, hex.EncodeToString(suffix)),
	}

	if _, err := up.Body.Seek(0, io.SeekStart); err != nil {
		return domain.Attachment{}, err
	}
	if err := s.storage.Put(ctx, a.StorageKey, contentType, up.Body, up.Size); err != nil {
		return domain.Attachment{}, fmt.Errorf("store attachment: %w", err)
	}
	if a.Kind == domain.AttachmentImage {
		a.ThumbKey = s.storeThumbnail(ctx, a.StorageKey, up.Body)
	}

	if err := s.attachments.Create(ctx, &a); err != nil {
		s.deleteObjects(ctx, a)
		return domain.Attachment{}, err
	}
	return a, nil
}

// storeThumbnail is best-effort: without a thumbnail the UI falls back to the original image.
func (s *Service) storeThumbnail(ctx context.Context, key string, body io.ReadSeeker) string {
	if _, err := body.Seek(0, io.SeekStart); err != nil {
		return ""
	}
	thumb, err := s.thumbs.Thumbnail(body)
	if err != nil {
		s.log.Warn("attachment thumbnail", "key", key, "err", err)
		return ""
	}
	thumbKey := key + ".thumb.jpg"
	if err := s.storage.Put(ctx, thumbKey, "image/jpeg", bytes.NewReader(thumb), int64(len(thumb))); err != nil {
		s.log.Warn("store attachment thumbnail", "key", thumbKey, "err", err)
		return ""
	}
	return thumbKey
}

func (s *Service) deleteObjects(ctx context.Context, a domain.Attachment) {
	keys := []string{a.StorageKey}
	if a.HasThumb() {
		keys = append(keys, a.ThumbKey)
	}
	if err := s.storage.DeleteMany(ctx, keys); err != nil {
		s.log.Warn("delete attachment objects", "attachment", a.ID, "err", err)
	}
}

// Delete removes an attachment. Allowed for the uploader and the project owner.
func (s *Service) Delete(ctx context.Context, userID, attachmentID int64) (domain.Attachment, error) {
	a, err := s.attachments.Get(ctx, attachmentID)
	if err != nil {
		return a, usecase.NotFoundAs(err, domain.ErrAttachmentNotFound)
	}
	role, err := usecase.RequireMember(ctx, s.members, a.ProjectID, userID, domain.ErrAttachmentNotFound)
	if err != nil {
		return a, err
	}
	if role != domain.RoleOwner && (a.UploaderID == nil || *a.UploaderID != userID) {
		return a, domain.ErrAttachmentDeleteDenied
	}
	if err := s.attachments.Delete(ctx, a.ID); err != nil {
		return a, err
	}
	s.deleteObjects(ctx, a)
	return a, nil
}

type Variant string

const (
	VariantFile  Variant = "file"
	VariantThumb Variant = "thumb"
)

// Open reads a stored attachment. Authorisation happens before: the caller presents a signed URL.
func (s *Service) Open(ctx context.Context, attachmentID int64, variant Variant, byteRange string) (domain.Attachment, Object, error) {
	a, err := s.attachments.Get(ctx, attachmentID)
	if err != nil {
		return a, Object{}, usecase.NotFoundAs(err, domain.ErrAttachmentNotFound)
	}
	key := a.StorageKey
	if variant == VariantThumb {
		if !a.HasThumb() {
			return a, Object{}, domain.ErrAttachmentNotFound
		}
		key = a.ThumbKey
	}
	obj, err := s.storage.Open(ctx, key, byteRange)
	if err != nil {
		return a, Object{}, usecase.NotFoundAs(err, domain.ErrAttachmentNotFound)
	}
	return a, obj, nil
}
