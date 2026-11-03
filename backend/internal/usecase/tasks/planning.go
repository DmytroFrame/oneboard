package tasks

import (
	"context"
	"strings"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase"
)

// Milestones, sprints and versions of a project. Members see them; managers change them.

func validPlanning(name *string, start, end *domain.Date) error {
	*name = strings.TrimSpace(*name)
	if *name == "" || len([]rune(*name)) > 100 {
		return domain.ErrPlanningInvalid
	}
	if start != nil && end != nil && end.Before(start.Time) {
		return domain.ErrPlanningInvalid
	}
	return nil
}

func (s *Service) Milestones(ctx context.Context, actor domain.User, projectID int64) ([]domain.Milestone, error) {
	if _, err := s.member(ctx, actor, projectID); err != nil {
		return nil, err
	}
	return s.planning.Milestones(ctx, projectID)
}

func (s *Service) Sprints(ctx context.Context, actor domain.User, projectID int64) ([]domain.Sprint, error) {
	if _, err := s.member(ctx, actor, projectID); err != nil {
		return nil, err
	}
	return s.planning.Sprints(ctx, projectID)
}

func (s *Service) Versions(ctx context.Context, actor domain.User, projectID int64) ([]domain.Version, error) {
	if _, err := s.member(ctx, actor, projectID); err != nil {
		return nil, err
	}
	return s.planning.Versions(ctx, projectID)
}

// checkItem ensures an existing item (ID != 0) belongs to the project it is saved in.
func (s *Service) checkItem(ctx context.Context, table string, id, projectID int64) error {
	if id == 0 {
		return nil
	}
	ok, err := s.planning.InProject(ctx, table, id, projectID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrPlanningNotFound
	}
	return nil
}

func (s *Service) SaveMilestone(ctx context.Context, actor domain.User, m domain.Milestone) (domain.Milestone, error) {
	if err := s.manager(ctx, actor, m.ProjectID); err != nil {
		return m, err
	}
	if err := validPlanning(&m.Name, m.Start, m.End); err != nil {
		return m, err
	}
	if err := s.checkItem(ctx, "milestones", m.ID, m.ProjectID); err != nil {
		return m, err
	}
	return m, s.planning.SaveMilestone(ctx, &m)
}

func (s *Service) SaveSprint(ctx context.Context, actor domain.User, sp domain.Sprint) (domain.Sprint, error) {
	if err := s.manager(ctx, actor, sp.ProjectID); err != nil {
		return sp, err
	}
	if err := validPlanning(&sp.Name, sp.Start, sp.End); err != nil {
		return sp, err
	}
	switch sp.Status {
	case domain.SprintNew, domain.SprintCurrent, domain.SprintPast:
	default:
		return sp, domain.ErrPlanningInvalid
	}
	if err := s.checkItem(ctx, "sprints", sp.ID, sp.ProjectID); err != nil {
		return sp, err
	}
	return sp, s.planning.SaveSprint(ctx, &sp)
}

func (s *Service) SaveVersion(ctx context.Context, actor domain.User, v domain.Version) (domain.Version, error) {
	if err := s.manager(ctx, actor, v.ProjectID); err != nil {
		return v, err
	}
	if err := validPlanning(&v.Name, nil, nil); err != nil {
		return v, err
	}
	switch v.Status {
	case domain.VersionInDevelopment, domain.VersionUpcoming, domain.VersionReleased, domain.VersionArchived:
	default:
		return v, domain.ErrPlanningInvalid
	}
	if err := s.checkItem(ctx, "versions", v.ID, v.ProjectID); err != nil {
		return v, err
	}
	return v, s.planning.SaveVersion(ctx, &v)
}

// DeletePlanning removes a milestone, sprint or version; its tasks just lose the link.
func (s *Service) DeletePlanning(ctx context.Context, actor domain.User, table string, id int64) error {
	projectID, err := s.planning.ProjectOf(ctx, table, id)
	if err != nil {
		return usecase.NotFoundAs(err, domain.ErrPlanningNotFound)
	}
	if err := s.manager(ctx, actor, projectID); err != nil {
		return err
	}
	if err := s.planning.Delete(ctx, table, id); err != nil {
		return err
	}
	s.events.Publish(ctx, domain.BoardEvent{Type: domain.EventColumnsChanged, ProjectID: projectID, ActorID: actor.ID})
	return nil
}
