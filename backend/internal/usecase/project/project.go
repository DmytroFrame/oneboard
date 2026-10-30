// Package project implements projects and their membership.
package project

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"strings"

	"go.uber.org/fx"

	"clodapp/backend/internal/domain"
	"clodapp/backend/internal/usecase"
)

var Module = fx.Module("project", fx.Provide(NewService))

var keyRe = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,9}$`)

var notFound = domain.ErrProjectNotFound

type Projects interface {
	ListForUser(ctx context.Context, userID int64) ([]domain.Project, error)
	GetForUser(ctx context.Context, projectID, userID int64) (domain.Project, error)
	Create(ctx context.Context, orgID int64, name, key string, ownerID int64) (int64, error) // domain.ErrDuplicate on a taken key
	Rename(ctx context.Context, projectID int64, name string) error
	Delete(ctx context.Context, projectID int64) error
}

type Members interface {
	usecase.Memberships
	List(ctx context.Context, projectID int64) ([]domain.Member, error)
	Add(ctx context.Context, projectID, userID int64, role domain.Role) error // domain.ErrDuplicate if already a member
	Remove(ctx context.Context, projectID, userID int64) error
}

type Columns interface {
	CreateMany(ctx context.Context, projectID int64, names []string) error
}

type Cards interface {
	UnassignUser(ctx context.Context, projectID, userID int64) error
}

// OrgRoles answers "what is this user's role in this organization".
type OrgRoles interface {
	Role(ctx context.Context, orgID, userID int64) (domain.OrgRole, error)
}

type AttachmentFiles interface {
	KeysForProjects(ctx context.Context, projectIDs []int64) ([]string, error)
}

type FileStorage interface {
	DeleteMany(ctx context.Context, keys []string) error
}

type SearchIndex interface {
	DeleteProject(ctx context.Context, projectID int64) error
}

type Service struct {
	projects Projects
	members  Members
	columns  Columns
	cards    Cards
	orgs     OrgRoles
	index    SearchIndex
	files    AttachmentFiles
	storage  FileStorage
	tx       usecase.Transactor
	log      *slog.Logger
}

type Deps struct {
	fx.In
	Projects Projects
	Members  Members
	Columns  Columns
	Cards    Cards
	Orgs     OrgRoles
	Index    SearchIndex
	Files    AttachmentFiles
	Storage  FileStorage
	Tx       usecase.Transactor
	Log      *slog.Logger
}

func NewService(d Deps) *Service {
	return &Service{d.Projects, d.Members, d.Columns, d.Cards, d.Orgs, d.Index, d.Files, d.Storage, d.Tx, d.Log}
}

func (s *Service) List(ctx context.Context, userID int64) ([]domain.Project, error) {
	return s.projects.ListForUser(ctx, userID)
}

func validName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 100 {
		return "", domain.ErrProjectName
	}
	return name, nil
}

// Create makes a project in an organization, with the default columns, owned by userID.
// Organization owners and admins can create projects.
func (s *Service) Create(ctx context.Context, userID, orgID int64, name, key string) (domain.Project, error) {
	orgRole, err := s.orgs.Role(ctx, orgID, userID)
	if err != nil {
		return domain.Project{}, err
	}
	if orgRole == domain.OrgRoleNone {
		return domain.Project{}, domain.ErrOrgNotFound
	}
	if !orgRole.CanCreateProjects() {
		return domain.Project{}, domain.ErrOrgAdminOnly
	}
	name, err = validName(name)
	if err != nil {
		return domain.Project{}, err
	}
	key = strings.ToUpper(strings.TrimSpace(key))
	if !keyRe.MatchString(key) {
		return domain.Project{}, domain.ErrProjectKey
	}

	var id int64
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if id, err = s.projects.Create(ctx, orgID, name, key, userID); err != nil {
			return err
		}
		if err := s.members.Add(ctx, id, userID, domain.RoleOwner); err != nil {
			return err
		}
		return s.columns.CreateMany(ctx, id, domain.DefaultColumns)
	})
	if errors.Is(err, domain.ErrDuplicate) {
		return domain.Project{}, domain.ErrProjectKeyTaken
	}
	if err != nil {
		return domain.Project{}, err
	}
	return s.projects.GetForUser(ctx, id, userID)
}

func (s *Service) Rename(ctx context.Context, userID, projectID int64, name string) (domain.Project, error) {
	if err := usecase.RequireOwner(ctx, s.members, projectID, userID, notFound); err != nil {
		return domain.Project{}, err
	}
	name, err := validName(name)
	if err != nil {
		return domain.Project{}, err
	}
	if err := s.projects.Rename(ctx, projectID, name); err != nil {
		return domain.Project{}, err
	}
	return s.projects.GetForUser(ctx, projectID, userID)
}

func (s *Service) Delete(ctx context.Context, userID, projectID int64) error {
	if err := usecase.RequireOwner(ctx, s.members, projectID, userID, notFound); err != nil {
		return err
	}
	keys, err := s.files.KeysForProjects(ctx, []int64{projectID})
	if err != nil {
		return err
	}
	if err := s.projects.Delete(ctx, projectID); err != nil {
		return err
	}
	if err := s.index.DeleteProject(ctx, projectID); err != nil {
		s.log.Error("search index: delete project", "project", projectID, "err", err)
	}
	if err := s.storage.DeleteMany(ctx, keys); err != nil {
		s.log.Error("storage: delete project files", "project", projectID, "err", err)
	}
	return nil
}

func (s *Service) Members(ctx context.Context, userID, projectID int64) ([]domain.Member, error) {
	if _, err := usecase.RequireMember(ctx, s.members, projectID, userID, notFound); err != nil {
		return nil, err
	}
	return s.members.List(ctx, projectID)
}

func (s *Service) AddMember(ctx context.Context, userID, projectID, targetID int64) ([]domain.Member, error) {
	if err := usecase.RequireOwner(ctx, s.members, projectID, userID, notFound); err != nil {
		return nil, err
	}
	p, err := s.projects.GetForUser(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}
	// Only people from the project's organization can join it.
	orgRole, err := s.orgs.Role(ctx, p.OrgID, targetID)
	if err != nil {
		return nil, err
	}
	if orgRole == domain.OrgRoleNone {
		return nil, domain.ErrNotInOrg
	}
	if err := s.members.Add(ctx, projectID, targetID, domain.RoleMember); err != nil {
		if errors.Is(err, domain.ErrDuplicate) {
			return nil, domain.ErrAlreadyMember
		}
		return nil, err
	}
	return s.members.List(ctx, projectID)
}

// RemoveMember lets the owner remove anyone but themselves, and lets any member leave.
// The removed user's cards in the project become unassigned.
func (s *Service) RemoveMember(ctx context.Context, userID, projectID, targetID int64) error {
	role, err := usecase.RequireMember(ctx, s.members, projectID, userID, notFound)
	if err != nil {
		return err
	}
	targetRole, err := s.members.Role(ctx, projectID, targetID)
	if err != nil {
		return err
	}
	switch {
	case targetRole == domain.RoleNone:
		return domain.ErrNotMember
	case targetRole == domain.RoleOwner:
		return domain.ErrCannotRemoveOwner
	case role != domain.RoleOwner && targetID != userID:
		return domain.ErrOwnerOnly
	}
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.members.Remove(ctx, projectID, targetID); err != nil {
			return err
		}
		return s.cards.UnassignUser(ctx, projectID, targetID)
	})
}
