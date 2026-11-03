// Package timetracking implements OmiTeam's time tracking: logging minutes on tasks, the weekly view
// and statistics, the task work log, and the "log your previous working days" warning.
package timetracking

import (
	"context"
	"errors"
	"log/slog"

	"go.uber.org/fx"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/modules"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase"
)

var Module = modules.Register(modules.UseCases, fx.Module("timetracking", fx.Provide(NewService)))

type Logs interface {
	// ForUser: someone's logs on projects of one organization.
	ForUser(ctx context.Context, orgID, userID int64, from, to domain.Date) ([]domain.Timelog, error)
	ForCard(ctx context.Context, cardID int64, offset, limit int) ([]domain.Timelog, error)
	Get(ctx context.Context, id int64) (domain.Timelog, error)
	Create(ctx context.Context, t *domain.Timelog) error
	Update(ctx context.Context, t domain.Timelog) error
	Delete(ctx context.Context, id int64) error
}

type Cards interface {
	Get(ctx context.Context, cardID int64) (domain.Card, error)
}

type Projects interface {
	GetAny(ctx context.Context, projectID, userID int64) (domain.Project, error)
}

type Profiles interface {
	// Get is the organization's profile of a member; domain.ErrNoRows for people outside it.
	Get(ctx context.Context, orgID, userID int64) (domain.Profile, error)
}

// Calendar: what is not a working day for someone.
type Calendar interface {
	Holidays(ctx context.Context, orgID int64, from, to domain.Date) ([]domain.Holiday, error)
	ApprovedAbsences(ctx context.Context, orgID, userID int64, from, to domain.Date) ([]domain.Vacation, error)
}

type Service struct {
	logs     Logs
	cards    Cards
	projects Projects
	profiles Profiles
	calendar Calendar
	members  usecase.Memberships
	events   usecase.BoardEvents
	log      *slog.Logger
}

type Deps struct {
	fx.In
	Logs     Logs
	Cards    Cards
	Projects Projects
	Profiles Profiles
	Calendar Calendar
	Members  usecase.Memberships
	Events   usecase.BoardEvents
	Log      *slog.Logger
}

func NewService(d Deps) *Service {
	return &Service{d.Logs, d.Cards, d.Projects, d.Profiles, d.Calendar, d.Members, d.Events, d.Log}
}

// canTrackFor: OmiTeam's canTrackTimeForUser — company owners, admins and managers, or a LEAD who
// administers the project.
func canTrackFor(actor domain.User, projectRole domain.Role) bool {
	switch actor.Role {
	case domain.CompanyOwner, domain.CompanyAdmin, domain.CompanyManager:
		return true
	}
	return actor.IsLead() && (projectRole == domain.RoleAdmin || projectRole == domain.RoleOwner)
}

// canSeeOthers: whose week can the actor open (their own always).
func canSeeOthers(actor domain.User) bool { return actor.Can(domain.PermEditWorklogs) }

// Week returns someone's logs and statistics for the week starting on Monday weekStart.
func (s *Service) Week(ctx context.Context, actor domain.User, userID int64, anyDay domain.Date) ([]domain.Timelog, domain.WeekStats, error) {
	if userID == 0 {
		userID = actor.ID
	}
	if userID != actor.ID && !canSeeOthers(actor) {
		return nil, domain.WeekStats{}, domain.ErrPermissionDenied
	}
	if _, err := s.profiles.Get(ctx, actor.OrgID, userID); err != nil {
		return nil, domain.WeekStats{}, usecase.NotFoundAs(err, domain.ErrUserNotFound)
	}
	from := domain.MondayOf(anyDay)
	to := from.AddDays(6)
	logs, err := s.logs.ForUser(ctx, actor.OrgID, userID, from, to)
	if err != nil {
		return nil, domain.WeekStats{}, err
	}
	stats, err := s.weekStats(ctx, actor.OrgID, userID, from, logs)
	return logs, stats, err
}

func (s *Service) weekStats(ctx context.Context, orgID, userID int64, from domain.Date, logs []domain.Timelog) (domain.WeekStats, error) {
	to := from.AddDays(6)
	cal, err := s.dayInfo(ctx, orgID, userID, from, to)
	if err != nil {
		return domain.WeekStats{}, err
	}
	hours := 8
	if p, err := s.profiles.Get(ctx, orgID, userID); err == nil {
		hours = p.WorkingHours
	}
	var w domain.WeekStats
	for d := from; !d.After(to.Time); d = d.AddDays(1) {
		day := domain.DayStats{Day: d, Holiday: cal.holidays[d.String()], Absence: cal.absences[d.String()]}
		if !d.Weekend() && day.Holiday == "" && day.Absence == "" {
			day.NormMinutes = hours * 60
		}
		for _, l := range logs {
			if l.Day.Equal(d.Time) {
				day.Minutes += l.Minutes
				if l.Billable {
					day.BillableMinutes += l.Minutes
				}
			}
		}
		w.Days = append(w.Days, day)
		w.Minutes += day.Minutes
		w.BillableMinutes += day.BillableMinutes
		w.NormMinutes += day.NormMinutes
		w.OvertimeMinutes += max(0, day.Minutes-day.NormMinutes)
	}
	return w, nil
}

type calendarInfo struct {
	holidays map[string]string              // date → name
	absences map[string]domain.VacationType // date → approved absence
}

func (s *Service) dayInfo(ctx context.Context, orgID, userID int64, from, to domain.Date) (calendarInfo, error) {
	info := calendarInfo{holidays: map[string]string{}, absences: map[string]domain.VacationType{}}
	hs, err := s.calendar.Holidays(ctx, orgID, from, to)
	if err != nil {
		return info, err
	}
	for _, h := range hs {
		info.holidays[h.Day.String()] = h.NameLatin
	}
	vs, err := s.calendar.ApprovedAbsences(ctx, orgID, userID, from, to)
	if err != nil {
		return info, err
	}
	for _, v := range vs {
		for d := v.Start; !d.After(v.End.Time); d = d.AddDays(1) {
			info.absences[d.String()] = v.Type
		}
	}
	return info, nil
}

// target resolves who a log is for and checks the actor may log in that card's project for them.
func (s *Service) target(ctx context.Context, actor domain.User, userID, cardID int64) (domain.Card, int64, error) {
	if userID == 0 {
		userID = actor.ID
	}
	card, err := s.cards.Get(ctx, cardID)
	if err != nil {
		return card, 0, usecase.NotFoundAs(err, domain.ErrCardNotFound)
	}
	actorRole, err := s.members.Role(ctx, card.ProjectID, actor.ID)
	if err != nil {
		return card, 0, err
	}
	if userID != actor.ID && !canTrackFor(actor, actorRole) {
		return card, 0, domain.ErrTrackForOthers
	}
	if actorRole == domain.RoleNone && !actor.Can(domain.PermViewAllProjects) {
		return card, 0, domain.ErrCardNotFound
	}
	role, err := s.members.Role(ctx, card.ProjectID, userID)
	if err != nil {
		return card, 0, err
	}
	if role == domain.RoleNone {
		return card, 0, domain.ErrNotProjectMember
	}
	p, err := s.projects.GetAny(ctx, card.ProjectID, actor.ID)
	if err != nil {
		return card, 0, err
	}
	if p.OrgID != actor.OrgID { // time is logged in the organization the user works in
		return card, 0, domain.ErrCardNotFound
	}
	if !p.Active {
		return card, 0, domain.ErrProjectInactive
	}
	return card, userID, nil
}

func (s *Service) Create(ctx context.Context, actor domain.User, in domain.TimelogInput) (domain.Timelog, error) {
	if err := in.Validate(); err != nil {
		return domain.Timelog{}, err
	}
	card, userID, err := s.target(ctx, actor, in.UserID, in.CardID)
	if err != nil {
		return domain.Timelog{}, err
	}
	creator := actor.ID
	t := domain.Timelog{UserID: userID, CardID: card.ID, ProjectID: card.ProjectID, Day: in.Day, Minutes: in.Minutes, Comment: in.Comment, CreatedBy: &creator}
	if err := s.logs.Create(ctx, &t); err != nil {
		return t, err
	}
	s.cardChanged(ctx, actor.ID, card.ID)
	return s.logs.Get(ctx, t.ID)
}

// owned loads a log the actor may change: their own, or anyone's where they may log for others.
func (s *Service) owned(ctx context.Context, actor domain.User, id int64) (domain.Timelog, error) {
	t, err := s.logs.Get(ctx, id)
	if err != nil {
		return t, usecase.NotFoundAs(err, domain.ErrTimelogNotFound)
	}
	p, err := s.projects.GetAny(ctx, t.ProjectID, actor.ID)
	if err != nil {
		return t, err
	}
	if p.OrgID != actor.OrgID {
		return t, domain.ErrTimelogNotFound
	}
	if t.UserID == actor.ID {
		return t, nil
	}
	role, err := s.members.Role(ctx, t.ProjectID, actor.ID)
	if err != nil {
		return t, err
	}
	if !canTrackFor(actor, role) {
		return t, domain.ErrTimelogNotFound
	}
	return t, nil
}

func (s *Service) Update(ctx context.Context, actor domain.User, id int64, in domain.TimelogInput) (domain.Timelog, error) {
	if err := in.Validate(); err != nil {
		return domain.Timelog{}, err
	}
	t, err := s.owned(ctx, actor, id)
	if err != nil {
		return t, err
	}
	oldCard := t.CardID
	card, _, err := s.target(ctx, actor, t.UserID, in.CardID)
	if err != nil {
		return t, err
	}
	t.CardID, t.ProjectID, t.Day, t.Minutes, t.Comment = card.ID, card.ProjectID, in.Day, in.Minutes, in.Comment
	if err := s.logs.Update(ctx, t); err != nil {
		return t, err
	}
	s.cardChanged(ctx, actor.ID, card.ID)
	if oldCard != card.ID {
		s.cardChanged(ctx, actor.ID, oldCard)
	}
	return s.logs.Get(ctx, id)
}

func (s *Service) Delete(ctx context.Context, actor domain.User, id int64) error {
	t, err := s.owned(ctx, actor, id)
	if err != nil {
		return err
	}
	if err := s.logs.Delete(ctx, id); err != nil {
		return err
	}
	s.cardChanged(ctx, actor.ID, t.CardID)
	return nil
}

// CardLogs is a task's work log, newest first.
func (s *Service) CardLogs(ctx context.Context, actor domain.User, cardID int64, offset int) ([]domain.Timelog, error) {
	card, err := s.cards.Get(ctx, cardID)
	if err != nil {
		return nil, usecase.NotFoundAs(err, domain.ErrCardNotFound)
	}
	if _, err := usecase.RequireMember(ctx, s.members, card.ProjectID, actor.ID, domain.ErrCardNotFound); err != nil {
		return nil, err
	}
	return s.logs.ForCard(ctx, cardID, max(0, offset), 30)
}

// cardChanged re-sends a card so its tracked time updates on everyone's board.
func (s *Service) cardChanged(ctx context.Context, actorID, cardID int64) {
	c, err := s.cards.Get(ctx, cardID)
	if err != nil {
		s.log.Warn("card after time log", "card", cardID, "err", err)
		return
	}
	s.events.Publish(ctx, domain.BoardEvent{Type: domain.EventCardUpdated, ProjectID: c.ProjectID, ActorID: actorID, Card: &c})
}

// WarningLookback: how far back missing logs are reported.
const WarningLookback = 14

// MissingDays lists the user's recent working days (before today) with nothing logged — the
// "log your previous working days" warning. Days before they started, weekends, holidays and
// approved absences don't count.
func (s *Service) MissingDays(ctx context.Context, actor domain.User) ([]domain.Date, error) {
	today := domain.Today()
	from, to := today.AddDays(-WarningLookback), today.AddDays(-1)
	if p, err := s.profiles.Get(ctx, actor.OrgID, actor.ID); err == nil {
		if p.StartDate != nil && p.StartDate.After(from.Time) {
			from = *p.StartDate
		}
		if p.WorkingHours == 0 {
			return []domain.Date{}, nil
		}
	} else if !errors.Is(err, domain.ErrNoRows) {
		return nil, err
	}
	if actor.OrgID == 0 || actor.Role == domain.CompanyCandidate || from.After(to.Time) {
		return []domain.Date{}, nil
	}
	logs, err := s.logs.ForUser(ctx, actor.OrgID, actor.ID, from, to)
	if err != nil {
		return nil, err
	}
	logged := map[string]bool{}
	for _, l := range logs {
		logged[l.Day.String()] = true
	}
	cal, err := s.dayInfo(ctx, actor.OrgID, actor.ID, from, to)
	if err != nil {
		return nil, err
	}
	missing := []domain.Date{}
	for d := from; !d.After(to.Time); d = d.AddDays(1) {
		k := d.String()
		if !d.Weekend() && cal.holidays[k] == "" && cal.absences[k] == "" && !logged[k] {
			missing = append(missing, d)
		}
	}
	return missing, nil
}
