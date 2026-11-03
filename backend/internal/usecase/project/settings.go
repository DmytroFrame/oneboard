package project

import (
	"context"
	"errors"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase"
)

// OmiTeam project management on top of OneBoard's projects: company roles (MANAGER, LEAD…) can
// manage projects they belong to, and some see every project.

// ListAll is every project of the organization, for those with viewAllProjects; others get their own.
func (s *Service) ListAll(ctx context.Context, actor domain.User) ([]domain.Project, error) {
	if !actor.Can(domain.PermViewAllProjects) {
		return s.projects.ListInOrg(ctx, actor.OrgID, actor.ID)
	}
	return s.projects.ListAll(ctx, actor.OrgID, actor.ID)
}

// canManage: the project's owner, or a company manager (editProject) who is a member or sees all projects.
func canManage(actor domain.User, p domain.Project) bool {
	return p.Role == domain.RoleOwner ||
		(actor.Can(domain.PermEditProject) && (p.Role != domain.RoleNone || actor.Can(domain.PermViewAllProjects)))
}

func (s *Service) visible(ctx context.Context, actor domain.User, projectID int64) (domain.Project, error) {
	p, err := s.projects.GetAny(ctx, projectID, actor.ID)
	if err != nil {
		return p, usecase.NotFoundAs(err, notFound)
	}
	// Outsiders only see projects of the organization they work in, and only with viewAllProjects.
	if p.Role == domain.RoleNone && (p.OrgID != actor.OrgID || !actor.Can(domain.PermViewAllProjects)) {
		return p, notFound
	}
	return p, nil
}

// UpdateSettings changes the OmiTeam fields. The budget takes editBudget; marking the main project
// is for owners and admins of the company.
func (s *Service) UpdateSettings(ctx context.Context, actor domain.User, projectID int64, in domain.ProjectSettings) (domain.Project, error) {
	p, err := s.visible(ctx, actor, projectID)
	if err != nil {
		return p, err
	}
	if !canManage(actor, p) {
		return p, domain.ErrPermissionDenied
	}
	if err := in.Normalize(); err != nil {
		return p, err
	}
	if !actor.Can(domain.PermEditBudget) {
		in.BudgetHours = p.BudgetHours
	}
	if !actor.Can(domain.PermOpenBackwoods) {
		in.IsMain = p.IsMain
	}
	if err := s.projects.UpdateSettings(ctx, projectID, in); err != nil {
		return p, err
	}
	s.tellMembersOf(ctx, projectID, actor.ID, domain.AccessRenamed, p)
	return s.projects.GetAny(ctx, projectID, actor.ID)
}

func (s *Service) SetPinned(ctx context.Context, actor domain.User, projectID int64, pinned bool) error {
	if _, err := s.visible(ctx, actor, projectID); err != nil {
		return err
	}
	return s.projects.SetPinned(ctx, actor.ID, projectID, pinned)
}

// SetMemberRole makes a member a project admin or back. The owner stays the owner.
func (s *Service) SetMemberRole(ctx context.Context, actor domain.User, projectID, targetID int64, role domain.Role) ([]domain.Member, error) {
	if role != domain.RoleAdmin && role != domain.RoleMember {
		return nil, domain.ErrProjectRoleInvalid
	}
	p, err := s.visible(ctx, actor, projectID)
	if err != nil {
		return nil, err
	}
	if p.Role != domain.RoleOwner && !actor.Can(domain.PermAddProjectMembers) {
		return nil, domain.ErrPermissionDenied
	}
	if err := s.members.SetRole(ctx, projectID, targetID, role); errors.Is(err, domain.ErrNoRows) {
		return nil, domain.ErrNotMember
	} else if err != nil {
		return nil, err
	}
	s.events.Publish(ctx, domain.BoardEvent{Type: domain.EventMembersChanged, ProjectID: projectID, ActorID: actor.ID})
	return s.members.List(ctx, projectID)
}

// tellMembersOf refreshes everyone's sidebar after the project's details changed.
func (s *Service) tellMembersOf(ctx context.Context, projectID, actorID int64, reason domain.AccessReason, p domain.Project) {
	members, err := s.members.List(ctx, projectID)
	if err != nil {
		s.log.Warn("project members", "err", err)
		return
	}
	s.tellMembers(ctx, members, actorID, reason, p)
}
