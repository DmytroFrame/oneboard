package wiki

import (
	"context"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase"
)

// Project comments sit next to the project wiki they are being moved into.

type ProjectComments interface {
	List(ctx context.Context, projectID int64, category domain.ProjectCommentCategory) ([]domain.ProjectComment, error)
	Get(ctx context.Context, id int64) (domain.ProjectComment, error)
	Create(ctx context.Context, c *domain.ProjectComment) error
	Update(ctx context.Context, c domain.ProjectComment) error
	Delete(ctx context.Context, id int64) error
	// ProjectOrg is the organization a project belongs to.
	ProjectOrg(ctx context.Context, projectID int64) (int64, error)
}

// CommentService: project members read and add comments; only company owners and admins
// (editProjectComment) edit or delete them.
type CommentService struct {
	comments ProjectComments
	members  usecase.Memberships
}

func NewCommentService(c ProjectComments, m usecase.Memberships) *CommentService {
	return &CommentService{c, m}
}

// member: project members, or people with viewAllProjects for projects of their organization.
func (s *CommentService) member(ctx context.Context, u domain.User, projectID int64) error {
	_, err := usecase.RequireMember(ctx, s.members, projectID, u.ID, domain.ErrProjectNotFound)
	if err != nil && u.Can(domain.PermViewAllProjects) {
		if org, oerr := s.comments.ProjectOrg(ctx, projectID); oerr == nil && org == u.OrgID {
			return nil
		}
	}
	return err
}

func (s *CommentService) List(ctx context.Context, u domain.User, projectID int64, category domain.ProjectCommentCategory) ([]domain.ProjectComment, error) {
	if err := s.member(ctx, u, projectID); err != nil {
		return nil, err
	}
	return s.comments.List(ctx, projectID, category)
}

func (s *CommentService) Create(ctx context.Context, u domain.User, projectID int64, c domain.ProjectComment) (domain.ProjectComment, error) {
	if err := s.member(ctx, u, projectID); err != nil {
		return c, err
	}
	c.ProjectID, c.AuthorID, c.AuthorName = projectID, &u.ID, u.Name
	if err := c.Normalize(); err != nil {
		return c, err
	}
	return c, s.comments.Create(ctx, &c)
}

func (s *CommentService) editable(ctx context.Context, u domain.User, id int64) (domain.ProjectComment, error) {
	c, err := s.comments.Get(ctx, id)
	if err != nil {
		return c, usecase.NotFoundAs(err, domain.ErrProjectCommentNotFound)
	}
	if err := s.member(ctx, u, c.ProjectID); err != nil {
		return c, domain.ErrProjectCommentNotFound
	}
	if !u.Can(domain.PermEditProjectComment) {
		return c, domain.ErrPermissionDenied
	}
	return c, nil
}

func (s *CommentService) Update(ctx context.Context, u domain.User, id int64, in domain.ProjectComment) (domain.ProjectComment, error) {
	c, err := s.editable(ctx, u, id)
	if err != nil {
		return c, err
	}
	c.Category, c.Title, c.Body = in.Category, in.Title, in.Body
	if err := c.Normalize(); err != nil {
		return c, err
	}
	return c, s.comments.Update(ctx, c)
}

func (s *CommentService) Delete(ctx context.Context, u domain.User, id int64) error {
	if _, err := s.editable(ctx, u, id); err != nil {
		return err
	}
	return s.comments.Delete(ctx, id)
}
