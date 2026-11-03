// Package reports implements OmiTeam's reports: the time report (sums of time logs and their export),
// billability and bench by person, and a project's task statistics.
package reports

import (
	"context"
	"slices"
	"time"

	"go.uber.org/fx"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/modules"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase"
)

var Module = modules.Register(modules.UseCases, fx.Module("reports", fx.Provide(NewService)))

type Reader interface {
	TimeSums(ctx context.Context, f domain.TimeReportFilter, group domain.ReportGroup) ([]domain.TimeSum, error)
	TimeLogs(ctx context.Context, f domain.TimeReportFilter) ([]domain.Timelog, error)
	// PeopleTime sums everyone's logs on the organization's projects in the period, per person and project.
	PeopleTime(ctx context.Context, orgID int64, from, to domain.Date) ([]domain.PersonProjectTime, error)
	// ReportPeople lists the organization's active members with the given roles and their daily hours.
	ReportPeople(ctx context.Context, orgID int64, roles []domain.CompanyRole) ([]domain.User, map[int64]int, error)
	ColumnMoves(ctx context.Context, projectID int64) ([]domain.ColumnMove, error)
	// ProjectOrg is the organization a project belongs to (domain.ErrNoRows for an unknown project).
	ProjectOrg(ctx context.Context, projectID int64) (int64, error)
	CardStarts(ctx context.Context, projectID int64) ([]domain.CardStart, error)
}

type Calendar interface {
	Holidays(ctx context.Context, orgID int64, from, to domain.Date) ([]domain.Holiday, error)
	Overlapping(ctx context.Context, orgID int64, from, to domain.Date) ([]domain.Vacation, error)
}

type Columns interface {
	List(ctx context.Context, projectID int64) ([]domain.Column, error)
}

type Service struct {
	reader   Reader
	calendar Calendar
	columns  Columns
	members  usecase.Memberships
}

func NewService(r Reader, c Calendar, cols Columns, m usecase.Memberships) *Service {
	return &Service{r, c, cols, m}
}

// scope narrows a time report to what the viewer may see: contractors only their own logs, people
// without viewAllProjects only projects they are in.
func scope(viewer domain.User, f *domain.TimeReportFilter) error {
	if viewer.Role == domain.CompanyCandidate {
		return domain.ErrPermissionDenied
	}
	if err := domain.ValidPeriod(f.From, f.To); err != nil {
		return err
	}
	f.OrgID = viewer.OrgID
	if viewer.Role == domain.CompanyContractor {
		f.OnlyUser = viewer.ID
	}
	if !viewer.Can(domain.PermViewAllProjects) {
		f.VisibleTo = viewer.ID
	}
	return nil
}

func (s *Service) Time(ctx context.Context, viewer domain.User, f domain.TimeReportFilter, group domain.ReportGroup) ([]domain.TimeSum, error) {
	if !group.Valid() {
		group = domain.GroupProject
	}
	if err := scope(viewer, &f); err != nil {
		return nil, err
	}
	return s.reader.TimeSums(ctx, f, group)
}

// TimeLogs are the logs behind the time report, for the export.
func (s *Service) TimeLogs(ctx context.Context, viewer domain.User, f domain.TimeReportFilter) ([]domain.Timelog, error) {
	if err := scope(viewer, &f); err != nil {
		return nil, err
	}
	return s.reader.TimeLogs(ctx, f)
}

// People builds the billability and bench reports (OmiTeam's billability-report-statistics and
// bench-report-statistics share their data): everyone's norm, absences and logged time.
func (s *Service) People(ctx context.Context, viewer domain.User, from, to domain.Date, roles []domain.CompanyRole) ([]domain.PersonReport, error) {
	if !viewer.Can(domain.PermViewCrewBlock) {
		return nil, domain.ErrPermissionDenied
	}
	if err := domain.ValidPeriod(from, to); err != nil {
		return nil, err
	}
	roles = slices.DeleteFunc(slices.Clone(roles), func(r domain.CompanyRole) bool { return !r.Valid() || r == domain.CompanyCandidate })
	if len(roles) == 0 {
		roles = slices.DeleteFunc(slices.Clone(domain.CompanyRoles), func(r domain.CompanyRole) bool { return r == domain.CompanyCandidate })
	}
	people, hours, err := s.reader.ReportPeople(ctx, viewer.OrgID, roles)
	if err != nil {
		return nil, err
	}
	hs, err := s.calendar.Holidays(ctx, viewer.OrgID, from, to)
	if err != nil {
		return nil, err
	}
	holidays := map[string]bool{}
	for _, h := range hs {
		holidays[h.Day.String()] = true
	}
	vacations, err := s.calendar.Overlapping(ctx, viewer.OrgID, from, to)
	if err != nil {
		return nil, err
	}
	times, err := s.reader.PeopleTime(ctx, viewer.OrgID, from, to)
	if err != nil {
		return nil, err
	}

	today := domain.Today()
	out := make([]domain.PersonReport, 0, len(people))
	for _, u := range people {
		perDay := hours[u.ID] * 60
		absent := map[string]domain.VacationType{}
		for _, v := range vacations {
			if v.UserID == u.ID && v.Status == domain.VacationApproved {
				for d := v.Start; !d.After(v.End.Time); d = d.AddDays(1) {
					absent[d.String()] = v.Type
				}
			}
		}
		r := domain.PersonReport{User: u, Absence: map[domain.VacationType]int{}, Projects: []domain.PersonProjectTime{}}
		for d := from; !d.After(to.Time); d = d.AddDays(1) {
			if d.Weekend() || holidays[d.String()] {
				continue
			}
			r.NormMinutes += perDay
			if t, ok := absent[d.String()]; ok {
				r.Absence[t] += perDay
			} else if !d.After(today.Time) {
				r.PastNormMinutes += perDay
			}
		}
		for _, t := range times {
			if t.UserID == u.ID {
				r.Minutes += t.Minutes
				r.BillableMinutes += t.BillableMinutes
				r.BenchMinutes += t.BenchMinutes
				r.Projects = append(r.Projects, t)
			}
		}
		out = append(out, r)
	}
	return out, nil
}

// statsBuckets is how many periods the task statistics show.
var statsBuckets = map[domain.StatsPeriod]int{domain.StatsWeek: 12, domain.StatsMonth: 12, domain.StatsYear: 5}

func periodStart(d domain.Date, p domain.StatsPeriod) domain.Date {
	switch p {
	case domain.StatsWeek:
		return domain.MondayOf(d)
	case domain.StatsYear:
		return domain.NewDate(d.Year(), time.January, 1)
	}
	return domain.NewDate(d.Year(), d.Month(), 1)
}

func nextPeriod(d domain.Date, p domain.StatsPeriod) domain.Date {
	switch p {
	case domain.StatsWeek:
		return d.AddDays(7)
	case domain.StatsYear:
		return domain.NewDate(d.Year()+1, time.January, 1)
	}
	return domain.NewDate(d.Year(), d.Month()+1, 1)
}

// TaskStats: per period, tasks created, tasks finished (moved into a "done" column) and how long
// tasks stayed in the chosen columns (all columns but "done" ones when none are chosen).
func (s *Service) TaskStats(ctx context.Context, viewer domain.User, projectID int64, period domain.StatsPeriod, columnIDs []int64) (domain.TaskStats, error) {
	var stats domain.TaskStats
	// Members see their project; viewAllProjects covers the other projects of the viewer's organization.
	if _, err := usecase.RequireMember(ctx, s.members, projectID, viewer.ID, domain.ErrProjectNotFound); err != nil {
		org, oerr := s.reader.ProjectOrg(ctx, projectID)
		if oerr != nil || org != viewer.OrgID || !viewer.Can(domain.PermViewAllProjects) {
			return stats, err
		}
	}
	if _, ok := statsBuckets[period]; !ok {
		period = domain.StatsWeek
	}
	cols, err := s.columns.List(ctx, projectID)
	if err != nil {
		return stats, err
	}
	done := map[int64]bool{}
	chosen := map[int64]bool{}
	for _, c := range cols {
		done[c.ID] = c.Status == domain.StatusDone
		if slices.Contains(columnIDs, c.ID) {
			chosen[c.ID] = true
		}
	}
	if len(chosen) == 0 {
		for _, c := range cols {
			chosen[c.ID] = !done[c.ID]
		}
	}
	starts, err := s.reader.CardStarts(ctx, projectID)
	if err != nil {
		return stats, err
	}
	moves, err := s.reader.ColumnMoves(ctx, projectID)
	if err != nil {
		return stats, err
	}

	// Buckets from the oldest shown period up to the current one.
	n := statsBuckets[period]
	first := periodStart(domain.Today(), period)
	for range n - 1 {
		first = periodStart(first.AddDays(-1), period)
	}
	var bounds []domain.Date
	for d := first; len(bounds) < n; d = nextPeriod(d, period) {
		bounds = append(bounds, d)
		stats.Buckets = append(stats.Buckets, domain.StatsBucket{Start: d})
	}
	bucket := func(t time.Time) int {
		day := domain.DateOf(t)
		i := -1
		for j, b := range bounds {
			if !day.Before(b.Time) {
				i = j
			}
		}
		return i
	}
	stayMinutes := make([]int, n)
	var totalMinutes int

	byCard := map[int64][]domain.ColumnMove{}
	for _, m := range moves {
		byCard[m.CardID] = append(byCard[m.CardID], m)
	}
	for _, c := range starts {
		if i := bucket(c.CreatedAt); i >= 0 {
			stats.Buckets[i].Created++
		}
		ms := byCard[c.CardID]
		col := c.ColumnID
		if len(ms) > 0 {
			col = ms[0].From
		}
		since := c.CreatedAt
		var stay time.Duration
		for _, m := range ms {
			if chosen[col] {
				stay += m.At.Sub(since)
			}
			if chosen[col] && !chosen[m.To] && stay > 0 { // left the chosen columns
				if i := bucket(m.At); i >= 0 {
					minutes := int(stay.Minutes())
					stats.Buckets[i].Stays++
					stayMinutes[i] += minutes
					stats.Stays++
					totalMinutes += minutes
				}
				stay = 0
			}
			if done[m.To] && !done[col] {
				if i := bucket(m.At); i >= 0 {
					stats.Buckets[i].Done++
				}
			}
			col, since = m.To, m.At
		}
	}
	for i := range stats.Buckets {
		if stats.Buckets[i].Stays > 0 {
			stats.Buckets[i].AvgMinutes = stayMinutes[i] / stats.Buckets[i].Stays
		}
	}
	if stats.Stays > 0 {
		stats.AvgMinutes = totalMinutes / stats.Stays
	}
	return stats, nil
}
