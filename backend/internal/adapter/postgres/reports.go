package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

// ReportRepo reads the data behind the reports.
type ReportRepo struct{ db *DB }

func NewReportRepo(db *DB) *ReportRepo { return &ReportRepo{db: db} }

// timeWhere turns a time report filter into a WHERE clause over timelogs t / cards c / projects p.
func timeWhere(f domain.TimeReportFilter) (string, []any) {
	var args []any
	arg := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	where := []string{"p.organization_id = " + arg(f.OrgID), "t.day BETWEEN " + arg(f.From.Time) + " AND " + arg(f.To.Time)}
	if len(f.ProjectIDs) > 0 {
		where = append(where, "t.project_id = ANY("+arg(f.ProjectIDs)+")")
	}
	if len(f.UserIDs) > 0 {
		where = append(where, "t.user_id = ANY("+arg(f.UserIDs)+")")
	}
	if f.MilestoneID != 0 {
		where = append(where, "c.milestone_id = "+arg(f.MilestoneID))
	}
	if f.Billable != nil {
		where = append(where, "(p.billable AND c.billable) = "+arg(*f.Billable))
	}
	if f.OnlyUser != 0 {
		where = append(where, "t.user_id = "+arg(f.OnlyUser))
	}
	if f.VisibleTo != 0 {
		where = append(where, "EXISTS (SELECT 1 FROM project_members pm WHERE pm.project_id = t.project_id AND pm.user_id = "+arg(f.VisibleTo)+")")
	}
	return " WHERE " + strings.Join(where, " AND "), args
}

func (r *ReportRepo) TimeSums(ctx context.Context, f domain.TimeReportFilter, group domain.ReportGroup) ([]domain.TimeSum, error) {
	// key and label per grouping: fixed SQL, never input
	cols := map[domain.ReportGroup][2]string{
		domain.GroupProject:   {"p.id", "p.name"},
		domain.GroupUser:      {"u.id", "u.name"},
		domain.GroupMilestone: {"COALESCE(m.id, 0)", "COALESCE(m.name, '')"},
		domain.GroupTask:      {"c.id", "p.key || '-' || c.number || ' ' || c.title"},
		domain.GroupDay:       {"0", "to_char(t.day, 'YYYY-MM-DD')"},
	}[group]
	order := "3 DESC, 2"
	if group == domain.GroupDay {
		order = "2"
	}
	where, args := timeWhere(f)
	rows, err := r.db.q(ctx).Query(ctx, `SELECT `+cols[0]+`, `+cols[1]+`, SUM(t.minutes),
			COALESCE(SUM(t.minutes) FILTER (WHERE p.billable AND c.billable), 0), COUNT(*)
		FROM timelogs t JOIN users u ON u.id = t.user_id JOIN cards c ON c.id = t.card_id JOIN projects p ON p.id = t.project_id
			LEFT JOIN milestones m ON m.id = c.milestone_id`+where+`
		GROUP BY 1, 2 ORDER BY `+order, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.TimeSum, error) {
		var s domain.TimeSum
		err := row.Scan(&s.ID, &s.Name, &s.Minutes, &s.BillableMinutes, &s.Count)
		if err == nil && group == domain.GroupDay {
			d, perr := domain.ParseDate(s.Name)
			s.Day, err = &d, perr
		}
		return s, err
	})
}

func (r *ReportRepo) TimeLogs(ctx context.Context, f domain.TimeReportFilter) ([]domain.Timelog, error) {
	where, args := timeWhere(f)
	rows, err := r.db.q(ctx).Query(ctx, timelogSelect+where+` ORDER BY t.day, u.name, t.id LIMIT 50000`, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Timelog, error) { return scanTimelog(row) })
}

func (r *ReportRepo) PeopleTime(ctx context.Context, orgID int64, from, to domain.Date) ([]domain.PersonProjectTime, error) {
	rows, err := r.db.q(ctx).Query(ctx, `SELECT t.user_id, p.id, p.name, p.billable, SUM(t.minutes),
			COALESCE(SUM(t.minutes) FILTER (WHERE p.billable AND c.billable), 0),
			COALESCE(SUM(t.minutes) FILTER (WHERE c.special_type = 'bench'), 0)
		FROM timelogs t JOIN cards c ON c.id = t.card_id JOIN projects p ON p.id = t.project_id
		WHERE p.organization_id = $1 AND t.day BETWEEN $2 AND $3
		GROUP BY t.user_id, p.id ORDER BY p.name`, orgID, from.Time, to.Time)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.PersonProjectTime, error) {
		var t domain.PersonProjectTime
		return t, row.Scan(&t.UserID, &t.ProjectID, &t.ProjectName, &t.ProjectBillable, &t.Minutes, &t.BillableMinutes, &t.BenchMinutes)
	})
}

func (r *ReportRepo) ReportPeople(ctx context.Context, orgID int64, roles []domain.CompanyRole) ([]domain.User, map[int64]int, error) {
	rows, err := r.db.q(ctx).Query(ctx, `SELECT `+userColumns+`, `+memberColumns+`, COALESCE(p.working_hours, 8)
		FROM users JOIN organization_members m ON m.user_id = users.id AND m.organization_id = $1
		LEFT JOIN profiles p ON p.user_id = users.id AND p.organization_id = $1
		WHERE m.active AND m.role = ANY($2) ORDER BY users.name`, orgID, roles)
	if err != nil {
		return nil, nil, err
	}
	hours := map[int64]int{}
	users, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.User, error) {
		var u domain.User
		var m domain.Membership
		var h int
		err := row.Scan(append(memberFields(&u, &m), &h)...)
		hours[u.ID] = h
		return u.In(m), err
	})
	return users, hours, err
}

func (r *ReportRepo) ColumnMoves(ctx context.Context, projectID int64) ([]domain.ColumnMove, error) {
	// history stores column ids as text; entries with empty values (none expected) are skipped
	rows, err := r.db.q(ctx).Query(ctx, `SELECT h.card_id, h.old_value::bigint, h.new_value::bigint, h.created_at
		FROM card_history h JOIN cards c ON c.id = h.card_id
		WHERE c.project_id = $1 AND h.field = 'column' AND h.old_value ~ '^[0-9]+$' AND h.new_value ~ '^[0-9]+$'
		ORDER BY h.card_id, h.created_at, h.id`, projectID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.ColumnMove, error) {
		var m domain.ColumnMove
		return m, row.Scan(&m.CardID, &m.From, &m.To, &m.At)
	})
}

func (r *ReportRepo) ProjectOrg(ctx context.Context, projectID int64) (int64, error) {
	var org int64
	return org, mapErr(r.db.q(ctx).QueryRow(ctx, `SELECT organization_id FROM projects WHERE id = $1`, projectID).Scan(&org))
}

func (r *ReportRepo) CardStarts(ctx context.Context, projectID int64) ([]domain.CardStart, error) {
	rows, err := r.db.q(ctx).Query(ctx, `SELECT id, column_id, created_at FROM cards WHERE project_id = $1 AND NOT is_template`, projectID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.CardStart, error) {
		var c domain.CardStart
		return c, row.Scan(&c.CardID, &c.ColumnID, &c.CreatedAt)
	})
}
