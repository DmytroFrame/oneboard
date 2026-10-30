// Package organization implements organizations: their members, roles and invitations.
// Projects belong to an organization; only its members can be added to them.
package organization

import (
	"context"
	"errors"
	"log/slog"
	"net/mail"
	"strings"

	"go.uber.org/fx"

	"clodapp/backend/internal/domain"
	"clodapp/backend/internal/usecase"
)

var Module = fx.Module("organization", fx.Provide(NewService))

type Organizations interface {
	ListForUser(ctx context.Context, userID int64) ([]domain.Organization, error)
	GetForUser(ctx context.Context, orgID, userID int64) (domain.Organization, error) // ErrNoRows for non-members
	Create(ctx context.Context, name string, createdBy int64) (int64, error)
	Rename(ctx context.Context, orgID int64, name string) error
	Delete(ctx context.Context, orgID int64) error
	ProjectIDs(ctx context.Context, orgID int64) ([]int64, error)
}

type Members interface {
	Role(ctx context.Context, orgID, userID int64) (domain.OrgRole, error) // OrgRoleNone for non-members
	List(ctx context.Context, orgID int64) ([]domain.OrgMember, error)
	Add(ctx context.Context, orgID, userID int64, role domain.OrgRole) error // ErrDuplicate if already a member
	SetRole(ctx context.Context, orgID, userID int64, role domain.OrgRole) error
	Remove(ctx context.Context, orgID, userID int64) error
	CountOwners(ctx context.Context, orgID int64) (int, error)
	// FirstOwnerExcept returns the longest-standing owner other than userID.
	FirstOwnerExcept(ctx context.Context, orgID, userID int64) (int64, error)
}

type Invitations interface {
	List(ctx context.Context, orgID int64) ([]domain.Invitation, error)
	Create(ctx context.Context, orgID int64, email string, role domain.OrgRole, invitedBy int64) error // ErrDuplicate
	Delete(ctx context.Context, orgID, invitationID int64) error                                       // ErrNoRows
}

type Users interface {
	ByEmail(ctx context.Context, email string) (domain.User, error)
}

// Projects is what leaving an organization does to its projects.
type Projects interface {
	// TransferOwnership hands every project in the org owned by from over to to.
	TransferOwnership(ctx context.Context, orgID, from, to int64) error
	// RemoveFromOrgProjects drops the user from the org's projects and unassigns their cards there.
	RemoveFromOrgProjects(ctx context.Context, orgID, userID int64) error
}

type SearchIndex interface {
	DeleteProject(ctx context.Context, projectID int64) error
}

type AttachmentFiles interface {
	KeysForProjects(ctx context.Context, projectIDs []int64) ([]string, error)
}

type FileStorage interface {
	DeleteMany(ctx context.Context, keys []string) error
}

type Service struct {
	orgs        Organizations
	members     Members
	invitations Invitations
	users       Users
	projects    Projects
	index       SearchIndex
	files       AttachmentFiles
	storage     FileStorage
	tx          usecase.Transactor
	log         *slog.Logger
}

type Deps struct {
	fx.In
	Orgs        Organizations
	Members     Members
	Invitations Invitations
	Users       Users
	Projects    Projects
	Index       SearchIndex
	Files       AttachmentFiles
	Storage     FileStorage
	Tx          usecase.Transactor
	Log         *slog.Logger
}

func NewService(d Deps) *Service {
	return &Service{d.Orgs, d.Members, d.Invitations, d.Users, d.Projects, d.Index, d.Files, d.Storage, d.Tx, d.Log}
}

// role returns the user's role, or ErrOrgNotFound for outsiders (who can't tell it exists).
func (s *Service) role(ctx context.Context, orgID, userID int64) (domain.OrgRole, error) {
	role, err := s.members.Role(ctx, orgID, userID)
	if err != nil {
		return "", err
	}
	if role == domain.OrgRoleNone {
		return "", domain.ErrOrgNotFound
	}
	return role, nil
}

func (s *Service) List(ctx context.Context, userID int64) ([]domain.Organization, error) {
	return s.orgs.ListForUser(ctx, userID)
}

func (s *Service) Get(ctx context.Context, userID, orgID int64) (domain.Organization, error) {
	o, err := s.orgs.GetForUser(ctx, orgID, userID)
	return o, usecase.NotFoundAs(err, domain.ErrOrgNotFound)
}

// Create makes an organization owned by userID. Anyone can create one.
func (s *Service) Create(ctx context.Context, userID int64, name string) (domain.Organization, error) {
	name, err := domain.NormalizeOrgName(name)
	if err != nil {
		return domain.Organization{}, err
	}
	var id int64
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if id, err = s.orgs.Create(ctx, name, userID); err != nil {
			return err
		}
		return s.members.Add(ctx, id, userID, domain.OrgRoleOwner)
	})
	if err != nil {
		return domain.Organization{}, err
	}
	return s.orgs.GetForUser(ctx, id, userID)
}

func (s *Service) Rename(ctx context.Context, userID, orgID int64, name string) (domain.Organization, error) {
	role, err := s.role(ctx, orgID, userID)
	if err != nil {
		return domain.Organization{}, err
	}
	if !role.CanRename() {
		return domain.Organization{}, domain.ErrOrgAdminOnly
	}
	if name, err = domain.NormalizeOrgName(name); err != nil {
		return domain.Organization{}, err
	}
	if err := s.orgs.Rename(ctx, orgID, name); err != nil {
		return domain.Organization{}, err
	}
	return s.orgs.GetForUser(ctx, orgID, userID)
}

// Delete removes the organization with all its projects, cards and files.
func (s *Service) Delete(ctx context.Context, userID, orgID int64) error {
	role, err := s.role(ctx, orgID, userID)
	if err != nil {
		return err
	}
	if !role.CanDelete() {
		return domain.ErrOrgOwnerOnly
	}
	projectIDs, err := s.orgs.ProjectIDs(ctx, orgID)
	if err != nil {
		return err
	}
	keys, err := s.files.KeysForProjects(ctx, projectIDs)
	if err != nil {
		return err
	}
	if err := s.orgs.Delete(ctx, orgID); err != nil {
		return err
	}
	// The database is the source of truth; leftovers in the index or storage are only logged.
	for _, id := range projectIDs {
		if err := s.index.DeleteProject(ctx, id); err != nil {
			s.log.Error("search index: delete project", "project", id, "err", err)
		}
	}
	if err := s.storage.DeleteMany(ctx, keys); err != nil {
		s.log.Error("storage: delete organization files", "org", orgID, "err", err)
	}
	return nil
}

func (s *Service) Members(ctx context.Context, userID, orgID int64) ([]domain.OrgMember, error) {
	if _, err := s.role(ctx, orgID, userID); err != nil {
		return nil, err
	}
	return s.members.List(ctx, orgID)
}

// InviteResult says whether the person joined right away (they had an account) or was invited by email.
type InviteResult struct {
	Added bool
}

func (s *Service) Invite(ctx context.Context, userID, orgID int64, email string, role domain.OrgRole) (InviteResult, error) {
	actor, err := s.role(ctx, orgID, userID)
	if err != nil {
		return InviteResult{}, err
	}
	if !actor.CanInvite() {
		return InviteResult{}, domain.ErrOrgAdminOnly
	}
	if !role.Valid() {
		return InviteResult{}, domain.ErrOrgRoleInvalid
	}
	if !actor.CanGrant(role) {
		return InviteResult{}, domain.ErrOrgRoleForbidden
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if _, err := mail.ParseAddress(email); err != nil {
		return InviteResult{}, domain.ErrEmailInvalid
	}

	u, err := s.users.ByEmail(ctx, email)
	switch {
	case err == nil:
		if err := s.members.Add(ctx, orgID, u.ID, role); err != nil {
			if errors.Is(err, domain.ErrDuplicate) {
				return InviteResult{}, domain.ErrAlreadyOrgMember
			}
			return InviteResult{}, err
		}
		return InviteResult{Added: true}, nil
	case errors.Is(err, domain.ErrNoRows):
		if err := s.invitations.Create(ctx, orgID, email, role, userID); err != nil {
			if errors.Is(err, domain.ErrDuplicate) {
				return InviteResult{}, domain.ErrInvitationExists
			}
			return InviteResult{}, err
		}
		return InviteResult{}, nil
	default:
		return InviteResult{}, err
	}
}

func (s *Service) Invitations(ctx context.Context, userID, orgID int64) ([]domain.Invitation, error) {
	role, err := s.role(ctx, orgID, userID)
	if err != nil {
		return nil, err
	}
	if !role.CanInvite() {
		return nil, domain.ErrOrgAdminOnly
	}
	return s.invitations.List(ctx, orgID)
}

func (s *Service) RevokeInvitation(ctx context.Context, userID, orgID, invitationID int64) error {
	role, err := s.role(ctx, orgID, userID)
	if err != nil {
		return err
	}
	if !role.CanInvite() {
		return domain.ErrOrgAdminOnly
	}
	return usecase.NotFoundAs(s.invitations.Delete(ctx, orgID, invitationID), domain.ErrInvitationNotFound)
}

func (s *Service) ChangeRole(ctx context.Context, userID, orgID, targetID int64, role domain.OrgRole) ([]domain.OrgMember, error) {
	actor, err := s.role(ctx, orgID, userID)
	if err != nil {
		return nil, err
	}
	if !actor.CanManageRoles() {
		return nil, domain.ErrOrgOwnerOnly
	}
	if !role.Valid() {
		return nil, domain.ErrOrgRoleInvalid
	}
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		current, err := s.members.Role(ctx, orgID, targetID)
		if err != nil {
			return err
		}
		if current == domain.OrgRoleNone {
			return domain.ErrOrgMemberNotFound
		}
		if current == domain.OrgRoleOwner && role != domain.OrgRoleOwner {
			if err := s.ensureAnotherOwner(ctx, orgID); err != nil {
				return err
			}
		}
		return s.members.SetRole(ctx, orgID, targetID, role)
	})
	if err != nil {
		return nil, err
	}
	return s.members.List(ctx, orgID)
}

func (s *Service) ensureAnotherOwner(ctx context.Context, orgID int64) error {
	owners, err := s.members.CountOwners(ctx, orgID)
	if err != nil {
		return err
	}
	if owners <= 1 {
		return domain.ErrLastOwner
	}
	return nil
}

// RemoveMember removes targetID from the organization (or lets a member leave when targetID == userID).
// They lose access to all of its projects; projects they owned go to whoever stays responsible.
func (s *Service) RemoveMember(ctx context.Context, userID, orgID, targetID int64) error {
	actor, err := s.role(ctx, orgID, userID)
	if err != nil {
		return err
	}
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		target, err := s.members.Role(ctx, orgID, targetID)
		if err != nil {
			return err
		}
		if target == domain.OrgRoleNone {
			return domain.ErrOrgMemberNotFound
		}
		if targetID != userID && !actor.CanRemove(target) {
			return domain.ErrRemoveForbidden
		}
		if target == domain.OrgRoleOwner {
			if err := s.ensureAnotherOwner(ctx, orgID); err != nil {
				return err
			}
		}

		successor := userID // the remover stays in the organization
		if targetID == userID {
			if successor, err = s.members.FirstOwnerExcept(ctx, orgID, userID); err != nil {
				return err
			}
		}
		if err := s.projects.TransferOwnership(ctx, orgID, targetID, successor); err != nil {
			return err
		}
		if err := s.projects.RemoveFromOrgProjects(ctx, orgID, targetID); err != nil {
			return err
		}
		return s.members.Remove(ctx, orgID, targetID)
	})
}
