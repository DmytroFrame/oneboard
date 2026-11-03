package people

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase"
)

// A person's documents (contracts, CVs…) and links. Private ones are visible to the person, the
// uploader and those with viewPrivateDocs.

func (s *Service) canSeeDocs(viewer domain.User, userID int64) bool {
	return viewer.ID == userID || viewer.Can(domain.PermViewCrewBlock) || viewer.Can(domain.PermAddDocs)
}

func canSeeDoc(viewer domain.User, d domain.Document) bool {
	return !d.Private || viewer.ID == d.UserID || (d.UploaderID != nil && *d.UploaderID == viewer.ID) || viewer.Can(domain.PermViewPrivateDocs)
}

func canChangeDoc(viewer domain.User, d domain.Document) bool {
	return (d.UploaderID != nil && *d.UploaderID == viewer.ID) || viewer.Can(domain.PermEditDocs)
}

func (s *Service) Documents(ctx context.Context, viewer domain.User, userID int64) ([]domain.Document, error) {
	if !s.canSeeDocs(viewer, userID) {
		return nil, domain.ErrPermissionDenied
	}
	all, err := s.docs.List(ctx, viewer.OrgID, userID)
	return slices.DeleteFunc(all, func(d domain.Document) bool { return !canSeeDoc(viewer, d) }), err
}

// NewDocument is a file (Body) or a link.
type NewDocument struct {
	Name, Description, Type, Link string
	Private                       bool
	FileName                      string
	Body                          io.Reader
}

func validDoc(name, typ, link string) (string, string, error) {
	name, link = strings.TrimSpace(name), strings.TrimSpace(link)
	if name == "" || len(name) > 200 || !slices.Contains(domain.DocumentTypes, typ) {
		return "", "", domain.ErrDocumentInvalid
	}
	if link != "" {
		if u, err := url.Parse(link); err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return "", "", domain.ErrDocumentInvalid
		}
	}
	return name, link, nil
}

func (s *Service) AddDocument(ctx context.Context, viewer domain.User, userID int64, in NewDocument) (domain.Document, error) {
	own := viewer.ID == userID
	if !(own && viewer.Can(domain.PermAddOwnDocs)) && !viewer.Can(domain.PermAddDocs) {
		return domain.Document{}, domain.ErrPermissionDenied
	}
	if _, err := s.users.InOrg(ctx, viewer.OrgID, userID); err != nil {
		return domain.Document{}, usecase.NotFoundAs(err, domain.ErrUserNotFound)
	}
	name, link, err := validDoc(in.Name, in.Type, in.Link)
	if err != nil {
		return domain.Document{}, err
	}
	uploader := viewer.ID
	d := domain.Document{UserID: userID, UploaderID: &uploader, Name: name, Description: strings.TrimSpace(in.Description),
		Type: in.Type, Private: in.Private, Link: link}
	if in.Body != nil {
		data, err := io.ReadAll(io.LimitReader(in.Body, domain.MaxDocumentBytes+1))
		if err != nil {
			return domain.Document{}, err
		}
		if len(data) == 0 {
			return domain.Document{}, domain.ErrDocumentInvalid
		}
		if len(data) > domain.MaxDocumentBytes {
			return domain.Document{}, domain.ErrDocumentTooLarge
		}
		b := make([]byte, 12)
		_, _ = rand.Read(b)
		d.StorageKey = fmt.Sprintf("documents/%d/%s", userID, hex.EncodeToString(b))
		d.FileName, d.ContentType, d.Size = sanitizeFileName(in.FileName), http.DetectContentType(data), int64(len(data))
		if err := s.files.Put(ctx, d.StorageKey, d.ContentType, bytes.NewReader(data), d.Size); err != nil {
			return domain.Document{}, err
		}
		d.Link = ""
	} else if link == "" {
		return domain.Document{}, domain.ErrDocumentInvalid
	}
	if err := s.docs.Create(ctx, viewer.OrgID, &d); err != nil {
		if d.StorageKey != "" {
			_ = s.files.Delete(context.WithoutCancel(ctx), d.StorageKey)
		}
		return domain.Document{}, err
	}
	return d, nil
}

func sanitizeFileName(name string) string {
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == '"' || r == '\\' || r == '/' {
			return '_'
		}
		return r
	}, strings.TrimSpace(name))
	if name == "" {
		return "document"
	}
	if len(name) > 200 {
		name = name[:200]
	}
	return name
}

func (s *Service) document(ctx context.Context, viewer domain.User, id int64) (domain.Document, error) {
	d, err := s.docs.Get(ctx, viewer.OrgID, id)
	if err != nil {
		return d, usecase.NotFoundAs(err, domain.ErrDocumentNotFound)
	}
	if !s.canSeeDocs(viewer, d.UserID) || !canSeeDoc(viewer, d) {
		return d, domain.ErrDocumentNotFound
	}
	return d, nil
}

func (s *Service) UpdateDocument(ctx context.Context, viewer domain.User, id int64, name, description, typ, link string, private bool) (domain.Document, error) {
	d, err := s.document(ctx, viewer, id)
	if err != nil {
		return d, err
	}
	if !canChangeDoc(viewer, d) {
		return d, domain.ErrPermissionDenied
	}
	name, link, err = validDoc(name, typ, link)
	if err != nil {
		return d, err
	}
	d.Name, d.Description, d.Type, d.Private = name, strings.TrimSpace(description), typ, private
	if d.StorageKey == "" {
		if link == "" {
			return d, domain.ErrDocumentInvalid
		}
		d.Link = link
	}
	return d, s.docs.Update(ctx, d)
}

func (s *Service) DeleteDocument(ctx context.Context, viewer domain.User, id int64) error {
	d, err := s.document(ctx, viewer, id)
	if err != nil {
		return err
	}
	if (d.UploaderID == nil || *d.UploaderID != viewer.ID) && !viewer.Can(domain.PermDeleteDocs) {
		return domain.ErrPermissionDenied
	}
	if err := s.docs.Delete(ctx, id); err != nil {
		return err
	}
	if d.StorageKey != "" {
		_ = s.files.Delete(context.WithoutCancel(ctx), d.StorageKey)
	}
	return nil
}

// OpenDocument streams a document's file to someone allowed to see it.
func (s *Service) OpenDocument(ctx context.Context, viewer domain.User, id int64) (domain.Document, io.ReadCloser, error) {
	d, err := s.document(ctx, viewer, id)
	if err != nil {
		return d, nil, err
	}
	if d.StorageKey == "" {
		return d, nil, domain.ErrDocumentNotFound
	}
	body, err := s.files.Read(ctx, d.StorageKey)
	return d, body, usecase.NotFoundAs(err, domain.ErrDocumentNotFound)
}
