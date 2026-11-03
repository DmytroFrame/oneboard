// Package dashboard builds the signed-in user's home page overview.
package dashboard

import (
	"context"

	"go.uber.org/fx"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/modules"
)

var Module = modules.Register(modules.UseCases, fx.Module("dashboard", fx.Provide(NewService)))

const assignedLimit = 8

type Reader interface {
	// Stats and AssignedCards cover the projects of one organization.
	Stats(ctx context.Context, orgID, userID int64) (domain.DashboardStats, error)
	// AssignedCards returns the user's cards, most urgent and recently changed first.
	AssignedCards(ctx context.Context, orgID, userID int64, limit int) ([]domain.AssignedCard, error)
}

type Service struct {
	reader  Reader
	company CompanyReader
}

func NewService(r Reader, c CompanyReader) *Service { return &Service{reader: r, company: c} }

type Dashboard struct {
	Stats    domain.DashboardStats
	Assigned []domain.AssignedCard
}

func (s *Service) Get(ctx context.Context, viewer domain.User) (Dashboard, error) {
	stats, err := s.reader.Stats(ctx, viewer.OrgID, viewer.ID)
	if err != nil {
		return Dashboard{}, err
	}
	assigned, err := s.reader.AssignedCards(ctx, viewer.OrgID, viewer.ID, assignedLimit)
	return Dashboard{Stats: stats, Assigned: assigned}, err
}

// Company widgets (OmiTeam's dashboard).

type CompanyReader interface {
	// All of them are about one organization.
	Counters(ctx context.Context, orgID, userID int64) (domain.CompanyCounters, error)
	Birthdays(ctx context.Context, orgID int64, from domain.Date, days int) ([]domain.PersonDate, error)
	Anniversaries(ctx context.Context, orgID int64, from domain.Date, days int) ([]domain.PersonDate, error)
	OnTrial(ctx context.Context, orgID int64, today domain.Date) ([]domain.PersonDate, error)
	// WorkTime sums billable and other minutes per day on the organization's projects; userID 0 = everyone.
	WorkTime(ctx context.Context, orgID, userID int64, from, to domain.Date) ([]domain.DayWork, error)
}

// Upcoming is how far ahead birthdays and anniversaries are shown.
const Upcoming = 30

type Company struct {
	Counters       domain.CompanyCounters
	Birthdays      []domain.PersonDate // nil when hidden for the viewer
	Anniversaries  []domain.PersonDate
	OnTrial        []domain.PersonDate // nil when hidden
	ShowCandidates bool
}

// CompanyFor builds the widgets the viewer may see.
func (s *Service) CompanyFor(ctx context.Context, viewer domain.User) (Company, error) {
	var c Company
	var err error
	if c.Counters, err = s.company.Counters(ctx, viewer.OrgID, viewer.ID); err != nil {
		return c, err
	}
	c.ShowCandidates = viewer.Can(domain.PermViewCandidates)
	if !c.ShowCandidates {
		c.Counters.Candidates = 0
	}
	today := domain.Today()
	if viewer.Can(domain.PermShowBirthdays) {
		if c.Birthdays, err = s.company.Birthdays(ctx, viewer.OrgID, today, Upcoming); err != nil {
			return c, err
		}
	}
	if c.Anniversaries, err = s.company.Anniversaries(ctx, viewer.OrgID, today, Upcoming); err != nil {
		return c, err
	}
	if viewer.Can(domain.PermViewTrialUser) {
		if c.OnTrial, err = s.company.OnTrial(ctx, viewer.OrgID, today); err != nil {
			return c, err
		}
	}
	return c, nil
}

// WorkTime is the chart of a week: the viewer's own, or the team's for those who manage worklogs.
func (s *Service) WorkTime(ctx context.Context, viewer domain.User, team bool, anyDay domain.Date) ([]domain.DayWork, error) {
	userID := viewer.ID
	if team {
		if !viewer.Can(domain.PermEditWorklogs) {
			return nil, domain.ErrPermissionDenied
		}
		userID = 0
	}
	from := domain.MondayOf(anyDay)
	return s.company.WorkTime(ctx, viewer.OrgID, userID, from, from.AddDays(6))
}
