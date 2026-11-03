// Package absence implements OmiTeam's absence requests (vacation, sick day, day off) with reviewers,
// and the company's public holidays.
package absence

import (
	"context"
	"slices"
	"strings"
	"time"

	"go.uber.org/fx"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/modules"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase"
)

var Module = modules.Register(modules.UseCases, fx.Module("absence", fx.Provide(NewService)))

// Holidays and requests belong to an organization (the actor's OrgID); ids from other organizations
// are "not found".

type Holidays interface {
	List(ctx context.Context, orgID int64, from, to domain.Date) ([]domain.Holiday, error)
	Save(ctx context.Context, orgID int64, h *domain.Holiday) error // insert when ID is 0; domain.ErrDuplicate on the same day and name
	Delete(ctx context.Context, orgID, id int64) error
}

type Vacations interface {
	Get(ctx context.Context, orgID, id int64) (domain.Vacation, error)
	List(ctx context.Context, orgID int64, f domain.VacationFilter) ([]domain.Vacation, error)
	ToReview(ctx context.Context, orgID, reviewerID int64) ([]domain.Vacation, error)
	Overlapping(ctx context.Context, orgID int64, from, to domain.Date) ([]domain.Vacation, error)
	Create(ctx context.Context, orgID int64, v *domain.Vacation) error
	Update(ctx context.Context, v domain.Vacation) error
	SetReviewers(ctx context.Context, id int64, reviewers []int64) error
	Decide(ctx context.Context, id int64, status domain.VacationStatus, by int64, at time.Time) error
	Delete(ctx context.Context, id int64) error
	// DaysTaken sums the working days of a user's approved requests per type in a year.
	DaysTaken(ctx context.Context, orgID, userID int64, year int) (map[domain.VacationType]int, error)
}

type Users interface {
	// Reviewers are the organization's active people who may approve absences.
	Reviewers(ctx context.Context, orgID int64) ([]domain.User, error)
	InOrg(ctx context.Context, orgID, userID int64) (domain.User, error)
}

type Service struct {
	holidays  Holidays
	vacations Vacations
	users     Users
	events    usecase.UserEvents
}

func NewService(h Holidays, v Vacations, u Users, events usecase.UserEvents) *Service {
	return &Service{holidays: h, vacations: v, users: u, events: events}
}

// --- holidays ---

func yearRange(year int) (domain.Date, domain.Date) {
	return domain.NewDate(year, time.January, 1), domain.NewDate(year, time.December, 31)
}

func (s *Service) Holidays(ctx context.Context, actor domain.User, year int) ([]domain.Holiday, error) {
	from, to := yearRange(year)
	return s.holidays.List(ctx, actor.OrgID, from, to)
}

func (s *Service) SaveHoliday(ctx context.Context, actor domain.User, h domain.Holiday) (domain.Holiday, error) {
	if !actor.Can(domain.PermEditHolidays) {
		return h, domain.ErrPermissionDenied
	}
	h.NameLatin, h.NameCyrillic = strings.TrimSpace(h.NameLatin), strings.TrimSpace(h.NameCyrillic)
	if h.Day.IsZero() || h.NameLatin == "" || len([]rune(h.NameLatin)) > 100 || len([]rune(h.NameCyrillic)) > 100 {
		return h, domain.ErrHolidayInvalid
	}
	if err := s.holidays.Save(ctx, actor.OrgID, &h); err != nil {
		return h, usecase.NotFoundAs(err, domain.ErrHolidayNotFound)
	}
	return h, nil
}

func (s *Service) DeleteHoliday(ctx context.Context, actor domain.User, id int64) error {
	if !actor.Can(domain.PermEditHolidays) {
		return domain.ErrPermissionDenied
	}
	return usecase.NotFoundAs(s.holidays.Delete(ctx, actor.OrgID, id), domain.ErrHolidayNotFound)
}

func (s *Service) holidaySet(ctx context.Context, orgID int64, from, to domain.Date) (map[string]bool, error) {
	hs, err := s.holidays.List(ctx, orgID, from, to)
	set := map[string]bool{}
	for _, h := range hs {
		set[h.Day.String()] = true
	}
	return set, err
}

// --- requests ---

// seesAll: who can open everyone's requests (OmiTeam: seeVacationsRequests, and HR).
func seesAll(actor domain.User) bool {
	return actor.Can(domain.PermSeeVacationsRequests) || actor.Role == domain.CompanyHR
}

// manages: may change requests that aren't theirs (canViewVacationMenu).
func manages(actor domain.User) bool {
	switch actor.Role {
	case domain.CompanyOwner, domain.CompanyAdmin, domain.CompanyManager:
		return true
	}
	return actor.IsLead()
}

func (s *Service) Reviewers(ctx context.Context, actor domain.User) ([]domain.User, error) {
	return s.users.Reviewers(ctx, actor.OrgID)
}

func (s *Service) Mine(ctx context.Context, actor domain.User, year int, typ domain.VacationType) ([]domain.Vacation, error) {
	return s.vacations.List(ctx, actor.OrgID, domain.VacationFilter{UserID: actor.ID, Year: year, Type: typ})
}

func (s *Service) All(ctx context.Context, actor domain.User, f domain.VacationFilter) ([]domain.Vacation, error) {
	if !seesAll(actor) {
		return nil, domain.ErrPermissionDenied
	}
	return s.vacations.List(ctx, actor.OrgID, f)
}

// ToReview: pending requests where the actor is a reviewer (the red dot in the menu).
func (s *Service) ToReview(ctx context.Context, actor domain.User) ([]domain.Vacation, error) {
	return s.vacations.ToReview(ctx, actor.OrgID, actor.ID)
}

// ThisWeek: everyone's absences overlapping the current week, pending first (dashboard widget).
func (s *Service) ThisWeek(ctx context.Context, actor domain.User) ([]domain.Vacation, error) {
	from := domain.MondayOf(domain.Today())
	list, err := s.vacations.Overlapping(ctx, actor.OrgID, from, from.AddDays(6))
	order := map[domain.VacationStatus]int{domain.VacationPending: 0, domain.VacationApproved: 1, domain.VacationDeclined: 2}
	slices.SortStableFunc(list, func(a, b domain.Vacation) int { return order[a.Status] - order[b.Status] })
	return list, err
}

// RequestInput is a new or edited request.
type RequestInput struct {
	Type        domain.VacationType
	Start, End  domain.Date
	Comment     string
	ReviewerIDs []int64
}

func (s *Service) validate(ctx context.Context, actor domain.User, in *RequestInput) (int, error) {
	in.Comment = strings.TrimSpace(in.Comment)
	if !in.Type.Valid() || in.Start.IsZero() || in.End.IsZero() || in.End.Before(in.Start.Time) || len(in.Comment) > 2000 ||
		in.End.Sub(in.Start.Time) > 366*24*time.Hour {
		return 0, domain.ErrVacationInvalid
	}
	if err := s.checkReviewers(ctx, actor, in.ReviewerIDs); err != nil {
		return 0, err
	}
	holidays, err := s.holidaySet(ctx, actor.OrgID, in.Start, in.End)
	if err != nil {
		return 0, err
	}
	days := domain.WorkingDays(in.Start, in.End, holidays)
	if days == 0 {
		return 0, domain.ErrVacationNoDays
	}
	return days, nil
}

// checkReviewers: at least one, all real reviewers, never the requester.
func (s *Service) checkReviewers(ctx context.Context, actor domain.User, ids []int64) error {
	if len(ids) == 0 || len(ids) > 10 {
		return domain.ErrVacationReviewers
	}
	allowed, err := s.users.Reviewers(ctx, actor.OrgID)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if id == actor.ID || !slices.ContainsFunc(allowed, func(u domain.User) bool { return u.ID == id }) {
			return domain.ErrVacationReviewers
		}
	}
	return nil
}

func (s *Service) Create(ctx context.Context, actor domain.User, in RequestInput) (domain.Vacation, error) {
	days, err := s.validate(ctx, actor, &in)
	if err != nil {
		return domain.Vacation{}, err
	}
	v := domain.Vacation{UserID: actor.ID, UserName: actor.Name, Type: in.Type, Start: in.Start, End: in.End, Days: days,
		Comment: in.Comment, Status: domain.VacationPending, ReviewerIDs: in.ReviewerIDs}
	if err := s.vacations.Create(ctx, actor.OrgID, &v); err != nil {
		return v, err
	}
	s.notify(ctx, actor.ID, v)
	return v, nil
}

// request loads a request the actor may change: their own while pending, or any for managers.
func (s *Service) request(ctx context.Context, actor domain.User, id int64) (domain.Vacation, error) {
	v, err := s.vacations.Get(ctx, actor.OrgID, id)
	if err != nil {
		return v, usecase.NotFoundAs(err, domain.ErrVacationNotFound)
	}
	if manages(actor) {
		return v, nil
	}
	if v.UserID != actor.ID {
		return v, domain.ErrVacationNotFound
	}
	if v.Status != domain.VacationPending {
		return v, domain.ErrVacationDecided
	}
	return v, nil
}

func (s *Service) Update(ctx context.Context, actor domain.User, id int64, in RequestInput) (domain.Vacation, error) {
	v, err := s.request(ctx, actor, id)
	if err != nil {
		return v, err
	}
	requester := domain.User{ID: v.UserID}
	days, err := s.validate(ctx, requester, &in)
	if err != nil {
		return v, err
	}
	v.Type, v.Start, v.End, v.Days, v.Comment, v.ReviewerIDs = in.Type, in.Start, in.End, days, in.Comment, in.ReviewerIDs
	if err := s.vacations.Update(ctx, v); err != nil {
		return v, err
	}
	if err := s.vacations.SetReviewers(ctx, id, in.ReviewerIDs); err != nil {
		return v, err
	}
	s.notify(ctx, actor.ID, v)
	return s.vacations.Get(ctx, actor.OrgID, id)
}

func (s *Service) SetReviewers(ctx context.Context, actor domain.User, id int64, reviewers []int64) (domain.Vacation, error) {
	v, err := s.request(ctx, actor, id)
	if err != nil {
		return v, err
	}
	if err := s.checkReviewers(ctx, domain.User{ID: v.UserID}, reviewers); err != nil {
		return v, err
	}
	if err := s.vacations.SetReviewers(ctx, id, reviewers); err != nil {
		return v, err
	}
	v.ReviewerIDs = reviewers
	s.notify(ctx, actor.ID, v)
	return v, nil
}

// Decide approves or declines a pending request: its reviewers may, and company owners and admins.
func (s *Service) Decide(ctx context.Context, actor domain.User, id int64, status domain.VacationStatus) (domain.Vacation, error) {
	if status != domain.VacationApproved && status != domain.VacationDeclined {
		return domain.Vacation{}, domain.ErrVacationInvalid
	}
	v, err := s.vacations.Get(ctx, actor.OrgID, id)
	if err != nil {
		return v, usecase.NotFoundAs(err, domain.ErrVacationNotFound)
	}
	admin := actor.Role == domain.CompanyOwner || actor.Role == domain.CompanyAdmin
	if !admin && !slices.Contains(v.ReviewerIDs, actor.ID) {
		return v, domain.ErrNotReviewer
	}
	if v.Status != domain.VacationPending && !admin {
		return v, domain.ErrVacationDecided
	}
	now := time.Now()
	if err := s.vacations.Decide(ctx, id, status, actor.ID, now); err != nil {
		return v, err
	}
	v.Status, v.DecidedBy, v.DecidedAt = status, &actor.ID, &now
	s.notify(ctx, actor.ID, v)
	return v, nil
}

func (s *Service) Delete(ctx context.Context, actor domain.User, id int64) error {
	v, err := s.request(ctx, actor, id)
	if err != nil {
		return err
	}
	if err := s.vacations.Delete(ctx, id); err != nil {
		return err
	}
	s.notify(ctx, actor.ID, v)
	return nil
}

// Stats: days taken and left per type. Someone else's take seeFilterInVacationStatistics.
func (s *Service) Stats(ctx context.Context, actor domain.User, userID int64, year int) (domain.VacationStats, error) {
	if userID == 0 {
		userID = actor.ID
	}
	if userID != actor.ID && !actor.Can(domain.PermVacationStatsFilter) {
		return domain.VacationStats{}, domain.ErrPermissionDenied
	}
	if _, err := s.users.InOrg(ctx, actor.OrgID, userID); err != nil {
		return domain.VacationStats{}, usecase.NotFoundAs(err, domain.ErrUserNotFound)
	}
	taken, err := s.vacations.DaysTaken(ctx, actor.OrgID, userID, year)
	if err != nil {
		return domain.VacationStats{}, err
	}
	st := domain.VacationStats{Year: year, Taken: map[domain.VacationType]int{}, Left: map[domain.VacationType]int{}}
	for typ, allowance := range domain.VacationAllowance {
		st.Taken[typ] = taken[typ]
		st.Left[typ] = max(0, allowance-taken[typ])
	}
	return st, nil
}

// notify tells the requester's and the reviewers' tabs that requests changed (lists, the menu dot).
func (s *Service) notify(ctx context.Context, actorID int64, v domain.Vacation) {
	for _, id := range append([]int64{v.UserID}, v.ReviewerIDs...) {
		s.events.PublishUser(ctx, domain.UserEvent{Type: domain.UserVacationsChanged, UserID: id, ActorID: actorID})
	}
}
