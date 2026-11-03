package postgres

import (
	"context"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

// CompanyRepo is the read model behind the dashboard's company widgets.
type CompanyRepo struct{ db *DB }

func NewCompanyRepo(db *DB) *CompanyRepo { return &CompanyRepo{db: db} }

func (r *CompanyRepo) Counters(ctx context.Context, orgID, userID int64) (domain.CompanyCounters, error) {
	var c domain.CompanyCounters
	err := r.db.q(ctx).QueryRow(ctx, `SELECT
		(SELECT COUNT(*) FROM organization_members WHERE organization_id = $2 AND active AND role NOT IN ('CONTRACTOR', 'CANDIDATE')),
		(SELECT COUNT(*) FROM organization_members WHERE organization_id = $2 AND active AND role = 'CONTRACTOR'),
		(SELECT COUNT(*) FROM project_members m JOIN projects p ON p.id = m.project_id
			WHERE m.user_id = $1 AND p.active AND p.organization_id = $2),
		(SELECT COUNT(*) FROM organization_members WHERE organization_id = $2 AND active AND role = 'CANDIDATE')`, userID, orgID).
		Scan(&c.Employees, &c.Contractors, &c.MyProjects, &c.Candidates)
	return c, err
}

// datesOf loads the organization's active members (not candidates) with the given date in their
// profile there (column: a fixed name, never input).
func (r *CompanyRepo) datesOf(ctx context.Context, orgID int64, column string) ([]domain.PersonDate, error) {
	rows, err := r.db.q(ctx).Query(ctx, `SELECT `+userColumns+`, p.`+column+`
		FROM users JOIN organization_members m ON m.user_id = users.id AND m.organization_id = $1
		JOIN profiles p ON p.user_id = users.id AND p.organization_id = $1
		WHERE m.active AND m.role <> 'CANDIDATE' AND p.`+column+` IS NOT NULL`, orgID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.PersonDate, error) {
		var pd domain.PersonDate
		var d time.Time
		err := row.Scan(append(userFields(&pd.User), &d)...)
		pd.Date = *toDate(&d)
		return pd, err
	})
}

// upcoming keeps people whose yearly date (birthday, start anniversary) falls in the next days,
// moved to that occurrence; Years is how many years it marks.
func upcoming(list []domain.PersonDate, from domain.Date, days int, anniversary bool) []domain.PersonDate {
	out := []domain.PersonDate{}
	end := from.AddDays(days)
	for _, pd := range list {
		next := domain.NewDate(from.Year(), pd.Date.Month(), pd.Date.Day()) // Feb 29 rolls to Mar 1 in other years
		if next.Before(from.Time) {
			next = domain.NewDate(from.Year()+1, pd.Date.Month(), pd.Date.Day())
		}
		years := next.Year() - pd.Date.Year()
		if next.After(end.Time) || (anniversary && years < 1) {
			continue
		}
		out = append(out, domain.PersonDate{User: pd.User, Date: next, Years: years})
	}
	slices.SortFunc(out, func(a, b domain.PersonDate) int { return a.Date.Compare(b.Date.Time) })
	return out
}

func (r *CompanyRepo) Birthdays(ctx context.Context, orgID int64, from domain.Date, days int) ([]domain.PersonDate, error) {
	list, err := r.datesOf(ctx, orgID, "birthday")
	if err != nil {
		return nil, err
	}
	out := upcoming(list, from, days, false)
	for i := range out {
		out[i].Years = 0 // ages are private
	}
	return out, nil
}

func (r *CompanyRepo) Anniversaries(ctx context.Context, orgID int64, from domain.Date, days int) ([]domain.PersonDate, error) {
	list, err := r.datesOf(ctx, orgID, "start_date")
	if err != nil {
		return nil, err
	}
	return upcoming(list, from, days, true), nil
}

func (r *CompanyRepo) OnTrial(ctx context.Context, orgID int64, today domain.Date) ([]domain.PersonDate, error) {
	list, err := r.datesOf(ctx, orgID, "trial_until")
	if err != nil {
		return nil, err
	}
	list = slices.DeleteFunc(list, func(pd domain.PersonDate) bool { return pd.Date.Before(today.Time) })
	slices.SortFunc(list, func(a, b domain.PersonDate) int { return a.Date.Compare(b.Date.Time) })
	return list, nil
}

func (r *CompanyRepo) WorkTime(ctx context.Context, orgID, userID int64, from, to domain.Date) ([]domain.DayWork, error) {
	rows, err := r.db.q(ctx).Query(ctx, `
		SELECT t.day,
			COALESCE(SUM(t.minutes) FILTER (WHERE p.billable AND c.billable), 0),
			COALESCE(SUM(t.minutes) FILTER (WHERE NOT (p.billable AND c.billable)), 0)
		FROM timelogs t JOIN projects p ON p.id = t.project_id JOIN cards c ON c.id = t.card_id
		WHERE p.organization_id = $4 AND t.day BETWEEN $1 AND $2 AND ($3 = 0 OR t.user_id = $3)
		GROUP BY t.day`, from.Time, to.Time, userID, orgID)
	if err != nil {
		return nil, err
	}
	type sum struct {
		day             time.Time
		billable, other int
	}
	sums, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (sum, error) {
		var s sum
		return s, row.Scan(&s.day, &s.billable, &s.other)
	})
	if err != nil {
		return nil, err
	}
	out := []domain.DayWork{}
	for d := from; !d.After(to.Time); d = d.AddDays(1) {
		w := domain.DayWork{Day: d}
		for _, s := range sums {
			if toDate(&s.day).Equal(d.Time) {
				w.BillableMinutes, w.OtherMinutes = s.billable, s.other
			}
		}
		out = append(out, w)
	}
	return out, nil
}
