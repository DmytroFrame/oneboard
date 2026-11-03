// Package organization implements organizations: their members, roles and invitations.
// Projects belong to an organization; only its members can be added to them.
package organization

import (
	"context"
	"errors"
	"log/slog"
	"net/mail"
	"slices"
	"strings"

	"go.uber.org/fx"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/modules"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase"
)

var Module = modules.Register(modules.UseCases, fx.Module("organization", fx.Provide(NewService)))

type Organizations interface {
	ListForUser(ctx context.Context, userID int64) ([]domain.Organization, error)
	GetForUser(ctx context.Context, orgID, userID int64) (domain.Organization, error) // ErrNoRows for non-members
	// Found creates an organization owned by its creator, with the default reference lists.
	Found(ctx context.Context, name string, ownerID int64) (int64, error)
	Rename(ctx context.Context, orgID int64, name string) error
	Delete(ctx context.Context, orgID int64) error
	ProjectIDs(ctx context.Context, orgID int64) ([]int64, error)
}

type Members interface {
	// Membership is the user's place in the organization; domain.ErrNoRows for outsiders.
	Membership(ctx context.Context, orgID, userID int64) (domain.Membership, error)
	List(ctx context.Context, orgID int64) ([]domain.OrgMember, error)
	Add(ctx context.Context, orgID, userID int64, role domain.CompanyRole) error // ErrDuplicate if already a member
	SetRole(ctx context.Context, orgID, userID int64, role domain.CompanyRole, additions []domain.AdditionRole) error
	Remove(ctx context.Context, orgID, userID int64) error
	CountOwners(ctx context.Context, orgID int64) (int, error)
	// FirstOwnerExcept returns the longest-standing owner other than userID.
	FirstOwnerExcept(ctx context.Context, orgID, userID int64) (int64, error)
}

type Invitations interface {
	List(ctx context.Context, orgID int64) ([]domain.Invitation, error)
	Create(ctx context.Context, orgID int64, email string, role domain.CompanyRole, invitedBy int64) error // ErrDuplicate
	Delete(ctx context.Context, orgID, invitationID int64) error                                           // ErrNoRows
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
	boardEvents usecase.BoardEvents
	userEvents  usecase.UserEvents
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
	BoardEvents usecase.BoardEvents
	UserEvents  usecase.UserEvents
	Tx          usecase.Transactor
	Log         *slog.Logger
}

func NewService(d Deps) *Service {
	return &Service{d.Orgs, d.Members, d.Invitations, d.Users, d.Projects, d.Index, d.Files, d.Storage, d.BoardEvents, d.UserEvents, d.Tx, d.Log}
}

// ---------- real-time notifications (best-effort, after the change is committed) ----------

// tellUser lets a user know their access to the organization changed.
func (s *Service) tellUser(ctx context.Context, userID, actorID int64, reason domain.AccessReason, org domain.Organization) {
	s.userEvents.PublishUser(ctx, domain.UserEvent{
		Type: domain.UserAccessChanged, UserID: userID, Reason: reason, OrgID: org.ID, Name: org.Name, ActorID: actorID,
	})
}

// tellMembers refreshes the member list for everyone in the organization.
func (s *Service) tellMembers(ctx context.Context, orgID, actorID int64) {
	members, err := s.members.List(ctx, orgID)
	if err != nil {
		s.log.Warn("notify organization members", "org", orgID, "err", err)
		return
	}
	for _, m := range members {
		s.userEvents.PublishUser(ctx, domain.UserEvent{Type: domain.UserOrgMembersChanged, UserID: m.ID, OrgID: orgID, ActorID: actorID})
	}
}

// member returns the user as a member of the organization, or ErrOrgNotFound for outsiders (who
// can't tell it exists) and people whose membership is inactive.
func (s *Service) member(ctx context.Context, orgID, userID int64) (domain.User, error) {
	m, err := s.members.Membership(ctx, orgID, userID)
	if err != nil {
		return domain.User{}, usecase.NotFoundAs(err, domain.ErrOrgNotFound)
	}
	if !m.Active {
		return domain.User{}, domain.ErrOrgNotFound
	}
	return domain.User{ID: userID}.In(m), nil
}

func (s *Service) List(ctx context.Context, userID int64) ([]domain.Organization, error) {
	return s.orgs.ListForUser(ctx, userID)
}

func (s *Service) Get(ctx context.Context, userID, orgID int64) (domain.Organization, error) {
	o, err := s.orgs.GetForUser(ctx, orgID, userID)
	return o, usecase.NotFoundAs(err, domain.ErrOrgNotFound)
}

// Create makes an organization owned by userID, isolated from every other one. Anyone signed in can.
func (s *Service) Create(ctx context.Context, userID int64, name string) (domain.Organization, error) {
	name, err := domain.NormalizeOrgName(name)
	if err != nil {
		return domain.Organization{}, err
	}
	var id int64
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		id, err = s.orgs.Found(ctx, name, userID)
		return err
	})
	if err != nil {
		return domain.Organization{}, err
	}
	return s.orgs.GetForUser(ctx, id, userID)
}

func (s *Service) Rename(ctx context.Context, userID, orgID int64, name string) (domain.Organization, error) {
	actor, err := s.member(ctx, orgID, userID)
	if err != nil {
		return domain.Organization{}, err
	}
	if !actor.ManagesOrg() {
		return domain.Organization{}, domain.ErrOrgAdminOnly
	}
	if name, err = domain.NormalizeOrgName(name); err != nil {
		return domain.Organization{}, err
	}
	if err := s.orgs.Rename(ctx, orgID, name); err != nil {
		return domain.Organization{}, err
	}
	org, err := s.orgs.GetForUser(ctx, orgID, userID)
	if err == nil {
		if members, lerr := s.members.List(ctx, orgID); lerr == nil {
			for _, m := range members {
				s.tellUser(ctx, m.ID, userID, domain.AccessRenamed, org)
			}
		}
	}
	return org, err
}

// Delete removes the organization with all its projects, cards and files.
func (s *Service) Delete(ctx context.Context, userID, orgID int64) error {
	actor, err := s.member(ctx, orgID, userID)
	if err != nil {
		return err
	}
	if actor.Role != domain.CompanyOwner {
		return domain.ErrOrgOwnerOnly
	}
	org, err := s.orgs.GetForUser(ctx, orgID, userID)
	if err != nil {
		return err
	}
	members, err := s.members.List(ctx, orgID)
	if err != nil {
		return err
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
	for _, id := range projectIDs {
		s.boardEvents.Publish(ctx, domain.BoardEvent{Type: domain.EventProjectDeleted, ProjectID: id, ActorID: userID})
	}
	for _, m := range members {
		s.tellUser(ctx, m.ID, userID, domain.AccessDeleted, org)
	}
	return nil
}

func (s *Service) Members(ctx context.Context, userID, orgID int64) ([]domain.OrgMember, error) {
	if _, err := s.member(ctx, orgID, userID); err != nil {
		return nil, err
	}
	return s.members.List(ctx, orgID)
}

// InviteResult says whether the person joined right away (they had an account) or was invited by email.
type InviteResult struct {
	Added bool
}

func (s *Service) Invite(ctx context.Context, userID, orgID int64, email string, role domain.CompanyRole) (InviteResult, error) {
	actor, err := s.member(ctx, orgID, userID)
	if err != nil {
		return InviteResult{}, err
	}
	if !actor.Can(domain.PermAddUser) {
		return InviteResult{}, domain.ErrOrgAdminOnly
	}
	if !role.Valid() {
		return InviteResult{}, domain.ErrOrgRoleInvalid
	}
	if !actor.CanInviteAs(role) {
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
		if org, err := s.orgs.GetForUser(ctx, orgID, userID); err == nil {
			s.tellUser(ctx, u.ID, userID, domain.AccessAdded, org)
		}
		s.tellMembers(ctx, orgID, userID)
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
	actor, err := s.member(ctx, orgID, userID)
	if err != nil {
		return nil, err
	}
	if !actor.Can(domain.PermAddUser) {
		return nil, domain.ErrOrgAdminOnly
	}
	return s.invitations.List(ctx, orgID)
}

func (s *Service) RevokeInvitation(ctx context.Context, userID, orgID, invitationID int64) error {
	actor, err := s.member(ctx, orgID, userID)
	if err != nil {
		return err
	}
	if !actor.Can(domain.PermAddUser) {
		return domain.ErrOrgAdminOnly
	}
	return usecase.NotFoundAs(s.invitations.Delete(ctx, orgID, invitationID), domain.ErrInvitationNotFound)
}

// ChangeRole sets a member's company role in the organization and, when additions is not nil, their
// addition roles (domain.CheckRoleChange); the organization always keeps an owner.
func (s *Service) ChangeRole(ctx context.Context, userID, orgID, targetID int64, role domain.CompanyRole, additions []domain.AdditionRole) ([]domain.OrgMember, error) {
	actor, err := s.member(ctx, orgID, userID)
	if err != nil {
		return nil, err
	}
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		m, err := s.members.Membership(ctx, orgID, targetID)
		if err != nil {
			return usecase.NotFoundAs(err, domain.ErrOrgMemberNotFound)
		}
		target := domain.User{ID: targetID}.In(m)
		target.Role, target.AdditionRoles = m.Role, m.AdditionRoles // judge by the role even if inactive
		if err := domain.CheckRoleChange(actor, target, role, additions); err != nil {
			return err
		}
		if m.Role == domain.CompanyOwner && role != domain.CompanyOwner {
			if err := s.ensureAnotherOwner(ctx, orgID); err != nil {
				return err
			}
		}
		if additions == nil {
			additions = m.AdditionRoles
		}
		return s.members.SetRole(ctx, orgID, targetID, role, additions)
	})
	if err != nil {
		return nil, err
	}
	if org, err := s.orgs.GetForUser(ctx, orgID, targetID); err == nil {
		s.tellUser(ctx, targetID, userID, domain.AccessRole, org)
	}
	s.userEvents.PublishUser(ctx, domain.UserEvent{Type: domain.UserAccountChanged, UserID: targetID, OrgID: orgID, ActorID: userID})
	s.tellMembers(ctx, orgID, userID)
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
	actor, err := s.member(ctx, orgID, userID)
	if err != nil {
		return err
	}
	org, err := s.orgs.GetForUser(ctx, orgID, userID)
	if err != nil {
		return err
	}
	projectIDs, err := s.orgs.ProjectIDs(ctx, orgID)
	if err != nil {
		return err
	}
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		m, err := s.members.Membership(ctx, orgID, targetID)
		if err != nil {
			return usecase.NotFoundAs(err, domain.ErrOrgMemberNotFound)
		}
		// Removing someone else takes fireUser and the right to have given them their role.
		if targetID != userID && (!actor.Can(domain.PermFireUser) || (m.Role != domain.CompanyCandidate && !slices.Contains(actor.AssignableRoles(), m.Role))) {
			return domain.ErrRemoveForbidden
		}
		if m.Role == domain.CompanyOwner {
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
	if err != nil {
		return err
	}
	// The removed user loses every board of the organization; the remaining people see the change.
	for _, id := range projectIDs {
		s.boardEvents.Publish(ctx, domain.BoardEvent{Type: domain.EventMembersChanged, ProjectID: id, ActorID: userID, RemovedUserID: targetID})
	}
	s.tellUser(ctx, targetID, userID, domain.AccessRemoved, org)
	s.tellMembers(ctx, orgID, userID)
	return nil
}
