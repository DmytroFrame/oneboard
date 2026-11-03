// Package hr implements OmiTeam's recruiting: candidates moving through the hiring funnel, hiring and
// rejecting them, and the HR statistics of hires and dismissals.
package hr

import (
	"context"
	"slices"
	"time"

	"go.uber.org/fx"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/modules"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase"
)

var Module = modules.Register(modules.UseCases, fx.Module("hr", fx.Provide(NewService)))

type Candidates interface {
	List(ctx context.Context, orgID int64, f domain.CandidateFilter) (domain.CandidatePage, error)
	// Get returns the data of a CANDIDATE member of the organization (defaults when nothing was saved
	// yet); domain.ErrNoRows otherwise.
	Get(ctx context.Context, orgID, userID int64) (domain.Candidate, error)
	Save(ctx context.Context, orgID int64, c domain.Candidate) error
}

// Log is the record the HR statistics are built from.
type Log interface {
	Record(ctx context.Context, e domain.HREvent) error
	Between(ctx context.Context, orgID int64, from, to domain.Date) ([]domain.HREvent, error)
	Delete(ctx context.Context, orgID, id int64) error
	// DeleteLatest removes a person's most recent event of a kind (a rejection taken back).
	DeleteLatest(ctx context.Context, orgID, userID int64, kind domain.HREventKind) error
}

type Accounts interface {
	InOrg(ctx context.Context, orgID, userID int64) (domain.User, error)
	SetRole(ctx context.Context, orgID, userID int64, role domain.CompanyRole, additions []domain.AdditionRole) error
	// StartIfUnset sets the start date of the organization's profile of the person unless it has one.
	StartIfUnset(ctx context.Context, orgID, userID int64, day domain.Date) error
}

type Service struct {
	candidates Candidates
	log        Log
	accounts   Accounts
	tx         usecase.Transactor
	events     usecase.UserEvents
}

func NewService(c Candidates, l Log, a Accounts, tx usecase.Transactor, events usecase.UserEvents) *Service {
	return &Service{c, l, a, tx, events}
}

// canSee: the candidates list is gated by viewCandidates (docs/omiteam-integration.md, decision 6);
// changing a candidate also needs editCandidate.
func canSee(viewer domain.User) bool { return viewer.Can(domain.PermViewCandidates) }

func (s *Service) List(ctx context.Context, viewer domain.User, f domain.CandidateFilter) (domain.CandidatePage, error) {
	if !canSee(viewer) {
		return domain.CandidatePage{}, domain.ErrPermissionDenied
	}
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 30
	}
	page, err := s.candidates.List(ctx, viewer.OrgID, f)
	if err == nil && !viewer.Can(domain.PermViewSalary) {
		for i := range page.Candidates {
			page.Candidates[i].RedactMoney()
		}
	}
	return page, err
}

func (s *Service) get(ctx context.Context, viewer domain.User, id int64) (domain.Candidate, error) {
	if !canSee(viewer) {
		return domain.Candidate{}, domain.ErrCandidateNotFound
	}
	c, err := s.candidates.Get(ctx, viewer.OrgID, id)
	return c, usecase.NotFoundAs(err, domain.ErrCandidateNotFound)
}

func (s *Service) Get(ctx context.Context, viewer domain.User, id int64) (domain.Candidate, error) {
	c, err := s.get(ctx, viewer, id)
	if err == nil && !viewer.Can(domain.PermViewSalary) {
		c.RedactMoney()
	}
	return c, err
}

// Update saves the recruiting data; money fields are kept as they were for those who can't see them.
func (s *Service) Update(ctx context.Context, viewer domain.User, id int64, in domain.Candidate) (domain.Candidate, error) {
	if !viewer.Can(domain.PermEditCandidate) {
		return domain.Candidate{}, domain.ErrPermissionDenied
	}
	current, err := s.get(ctx, viewer, id)
	if err != nil {
		return current, err
	}
	next := current
	next.Status, next.Attributes, next.Channel, next.TrelloLink = in.Status, in.Attributes, in.Channel, in.TrelloLink
	if viewer.Can(domain.PermViewSalary) {
		next.MinSalary, next.MaxSalary, next.MinRate, next.MaxRate = in.MinSalary, in.MaxSalary, in.MinRate, in.MaxRate
		next.HiringCommission, next.CommissionReceiver = in.HiringCommission, in.CommissionReceiver
	}
	next.Recruiter = nil
	if in.Recruiter != nil && in.Recruiter.ID != 0 {
		r, err := s.accounts.InOrg(ctx, viewer.OrgID, in.Recruiter.ID)
		if err != nil || r.Role != domain.CompanyRecruiter || !r.Active {
			return current, domain.ErrCandidateInvalid
		}
		next.Recruiter = &r
	}
	if err := next.Normalize(); err != nil {
		return current, err
	}
	if err := s.candidates.Save(ctx, viewer.OrgID, next); err != nil {
		return current, err
	}
	return s.Get(ctx, viewer, id)
}

// SetStatus moves a candidate along the funnel (the quick action in the list).
func (s *Service) SetStatus(ctx context.Context, viewer domain.User, id int64, status domain.CandidateStatus) (domain.Candidate, error) {
	if !viewer.Can(domain.PermEditCandidate) {
		return domain.Candidate{}, domain.ErrPermissionDenied
	}
	c, err := s.get(ctx, viewer, id)
	if err != nil {
		return c, err
	}
	c.Status = status
	if err := c.Normalize(); err != nil {
		return c, err
	}
	if err := s.candidates.Save(ctx, viewer.OrgID, c); err != nil {
		return c, err
	}
	return s.Get(ctx, viewer, id)
}

// Reject closes a candidate unsuccessfully with a reason from the list; an empty reason reopens them.
func (s *Service) Reject(ctx context.Context, viewer domain.User, id int64, reason string) (domain.Candidate, error) {
	if !viewer.Can(domain.PermEditCandidate) {
		return domain.Candidate{}, domain.ErrPermissionDenied
	}
	if reason != "" && !slices.Contains(domain.RejectionReasons, reason) {
		return domain.Candidate{}, domain.ErrRejectionReason
	}
	c, err := s.get(ctx, viewer, id)
	if err != nil {
		return c, err
	}
	was := c.RejectionReason
	c.RejectionReason = reason
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.candidates.Save(ctx, viewer.OrgID, c); err != nil {
			return err
		}
		switch {
		case was == "" && reason != "":
			return s.log.Record(ctx, domain.HREvent{OrgID: viewer.OrgID, User: c.User, Kind: domain.HRRejected, Day: domain.Today(), Reason: reason, ActorID: &viewer.ID})
		case was != "" && reason == "":
			return s.log.DeleteLatest(ctx, viewer.OrgID, id, domain.HRRejected)
		}
		return nil
	})
	if err != nil {
		return c, err
	}
	return s.Get(ctx, viewer, id)
}

// Hire turns a candidate into a member of the team with the given role: the funnel ends "in team",
// the start date is set if empty and the hire shows in the HR statistics.
func (s *Service) Hire(ctx context.Context, viewer domain.User, id int64, role domain.CompanyRole) (domain.User, error) {
	if !viewer.Can(domain.PermAddUser) || !slices.Contains(viewer.AssignableRoles(), role) {
		return domain.User{}, domain.ErrRoleChangeForbidden
	}
	c, err := s.get(ctx, viewer, id)
	if err != nil {
		return domain.User{}, err
	}
	today := domain.Today()
	c.Status, c.RejectionReason = domain.CandidateInTeam, ""
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.candidates.Save(ctx, viewer.OrgID, c); err != nil {
			return err
		}
		if err := s.accounts.SetRole(ctx, viewer.OrgID, id, role, c.User.AdditionRoles); err != nil {
			return err
		}
		if err := s.accounts.StartIfUnset(ctx, viewer.OrgID, id, today); err != nil {
			return err
		}
		return s.log.Record(ctx, domain.HREvent{OrgID: viewer.OrgID, User: c.User, Kind: domain.HRHired, Day: today, FromCandidate: true, ActorID: &viewer.ID})
	})
	if err != nil {
		return domain.User{}, err
	}
	s.events.PublishUser(ctx, domain.UserEvent{Type: domain.UserAccountChanged, UserID: id, OrgID: viewer.OrgID, ActorID: viewer.ID})
	u := c.User
	u.Role = role
	return u, nil
}

// Stats: hires and dismissals per month of a year (months from..to), and the candidates closed.
func (s *Service) Stats(ctx context.Context, viewer domain.User, year, fromMonth, toMonth int) (domain.HRStats, error) {
	var st domain.HRStats
	if !viewer.Can(domain.PermAccessStatistics) { // decision 6
		return st, domain.ErrPermissionDenied
	}
	fromMonth, toMonth = min(max(fromMonth, 1), 12), min(max(toMonth, 1), 12)
	if toMonth < fromMonth || year < 2000 || year > 3000 {
		return st, domain.ErrReportPeriod
	}
	from := domain.NewDate(year, time.Month(fromMonth), 1)
	to := domain.NewDate(year, time.Month(toMonth)+1, 1).AddDays(-1)
	events, err := s.log.Between(ctx, viewer.OrgID, from, to)
	if err != nil {
		return st, err
	}
	for m := fromMonth; m <= toMonth; m++ {
		st.Months = append(st.Months, domain.HRMonth{Month: m})
	}
	st.Hired, st.Dismissed = []domain.HREvent{}, []domain.HREvent{}
	for _, e := range events {
		i := int(e.Day.Month()) - fromMonth
		switch e.Kind {
		case domain.HRHired:
			st.Months[i].Hired++
			st.Hired = append(st.Hired, e)
			if e.FromCandidate {
				st.ClosedSuccess++
			}
		case domain.HRDismissed:
			st.Months[i].Dismissed++
			st.Dismissed = append(st.Dismissed, e)
		case domain.HRRejected:
			st.ClosedFail++
		}
	}
	return st, nil
}

// DeleteEvent removes a record from the statistics (the person stays as they are).
func (s *Service) DeleteEvent(ctx context.Context, viewer domain.User, id int64) error {
	if !viewer.Can(domain.PermDeleteStatisticsItems) {
		return domain.ErrPermissionDenied
	}
	return usecase.NotFoundAs(s.log.Delete(ctx, viewer.OrgID, id), domain.ErrHREventNotFound)
}
