package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

// --- time logs ---

type TimelogRepo struct{ db *DB }

func NewTimelogRepo(db *DB) *TimelogRepo { return &TimelogRepo{db: db} }

const timelogSelect = `SELECT t.id, t.user_id, t.card_id, t.project_id, t.day, t.minutes, t.comment, t.created_by, t.created_at,
	u.name, p.key || '-' || c.number, c.title, p.name, p.billable AND c.billable
	FROM timelogs t JOIN users u ON u.id = t.user_id JOIN cards c ON c.id = t.card_id JOIN projects p ON p.id = t.project_id`

func scanTimelog(row pgx.Row) (domain.Timelog, error) {
	var t domain.Timelog
	var day time.Time
	err := row.Scan(&t.ID, &t.UserID, &t.CardID, &t.ProjectID, &day, &t.Minutes, &t.Comment, &t.CreatedBy, &t.CreatedAt,
		&t.UserName, &t.CardKey, &t.CardTitle, &t.ProjectName, &t.Billable)
	t.Day = *toDate(&day)
	return t, mapErr(err)
}

func (r *TimelogRepo) collect(ctx context.Context, sql string, args ...any) ([]domain.Timelog, error) {
	rows, err := r.db.q(ctx).Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Timelog, error) { return scanTimelog(row) })
}

func (r *TimelogRepo) ForUser(ctx context.Context, orgID, userID int64, from, to domain.Date) ([]domain.Timelog, error) {
	return r.collect(ctx, timelogSelect+` WHERE p.organization_id = $1 AND t.user_id = $2 AND t.day BETWEEN $3 AND $4 ORDER BY t.day, t.id`,
		orgID, userID, from.Time, to.Time)
}

func (r *TimelogRepo) ForCard(ctx context.Context, cardID int64, offset, limit int) ([]domain.Timelog, error) {
	return r.collect(ctx, timelogSelect+` WHERE t.card_id = $1 ORDER BY t.day DESC, t.id DESC OFFSET $2 LIMIT $3`, cardID, offset, limit)
}

func (r *TimelogRepo) Get(ctx context.Context, id int64) (domain.Timelog, error) {
	return scanTimelog(r.db.q(ctx).QueryRow(ctx, timelogSelect+` WHERE t.id = $1`, id))
}

func (r *TimelogRepo) Create(ctx context.Context, t *domain.Timelog) error {
	return mapErr(r.db.q(ctx).QueryRow(ctx, `INSERT INTO timelogs (user_id, card_id, project_id, day, minutes, comment, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id, created_at`,
		t.UserID, t.CardID, t.ProjectID, t.Day.Time, t.Minutes, t.Comment, t.CreatedBy).Scan(&t.ID, &t.CreatedAt))
}

func (r *TimelogRepo) Update(ctx context.Context, t domain.Timelog) error {
	return exec(ctx, r.db, `UPDATE timelogs SET card_id = $2, project_id = $3, day = $4, minutes = $5, comment = $6, updated_at = now()
		WHERE id = $1`, t.ID, t.CardID, t.ProjectID, t.Day.Time, t.Minutes, t.Comment)
}

func (r *TimelogRepo) Delete(ctx context.Context, id int64) error {
	return execOne(ctx, r.db, `DELETE FROM timelogs WHERE id = $1`, id)
}

// --- holidays ---

type HolidayRepo struct{ db *DB }

func NewHolidayRepo(db *DB) *HolidayRepo { return &HolidayRepo{db: db} }

func (r *HolidayRepo) List(ctx context.Context, orgID int64, from, to domain.Date) ([]domain.Holiday, error) {
	rows, err := r.db.q(ctx).Query(ctx, `SELECT id, day, name_latin, name_cyrillic FROM holidays
		WHERE organization_id = $1 AND day BETWEEN $2 AND $3 ORDER BY day`, orgID, from.Time, to.Time)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Holiday, error) {
		var h domain.Holiday
		var day time.Time
		err := row.Scan(&h.ID, &day, &h.NameLatin, &h.NameCyrillic)
		h.Day = *toDate(&day)
		return h, err
	})
}

// Holidays implements timetracking.Calendar.
func (r *HolidayRepo) Holidays(ctx context.Context, orgID int64, from, to domain.Date) ([]domain.Holiday, error) {
	return r.List(ctx, orgID, from, to)
}

func (r *HolidayRepo) Save(ctx context.Context, orgID int64, h *domain.Holiday) error {
	if h.ID == 0 {
		return mapErr(r.db.q(ctx).QueryRow(ctx, `INSERT INTO holidays (organization_id, day, name_latin, name_cyrillic) VALUES ($1, $2, $3, $4) RETURNING id`,
			orgID, h.Day.Time, h.NameLatin, h.NameCyrillic).Scan(&h.ID))
	}
	return execOne(ctx, r.db, `UPDATE holidays SET day = $3, name_latin = $4, name_cyrillic = $5 WHERE organization_id = $1 AND id = $2`,
		orgID, h.ID, h.Day.Time, h.NameLatin, h.NameCyrillic)
}

func (r *HolidayRepo) Delete(ctx context.Context, orgID, id int64) error {
	return execOne(ctx, r.db, `DELETE FROM holidays WHERE organization_id = $1 AND id = $2`, orgID, id)
}

// --- vacations ---

type VacationRepo struct{ db *DB }

func NewVacationRepo(db *DB) *VacationRepo { return &VacationRepo{db: db} }

const vacationSelect = `SELECT v.id, v.user_id, u.name, v.type, v.start_day, v.end_day, v.days, v.comment, v.status, v.decided_by,
	v.decided_at, v.created_at,
	COALESCE((SELECT array_agg(r.user_id ORDER BY r.user_id) FROM vacation_reviewers r WHERE r.vacation_id = v.id), '{}')
	FROM vacations v JOIN users u ON u.id = v.user_id`

func scanVacation(row pgx.Row) (domain.Vacation, error) {
	var v domain.Vacation
	var start, end time.Time
	err := row.Scan(&v.ID, &v.UserID, &v.UserName, &v.Type, &start, &end, &v.Days, &v.Comment, &v.Status, &v.DecidedBy,
		&v.DecidedAt, &v.CreatedAt, &v.ReviewerIDs)
	v.Start, v.End = *toDate(&start), *toDate(&end)
	return v, mapErr(err)
}

func (r *VacationRepo) collect(ctx context.Context, sql string, args ...any) ([]domain.Vacation, error) {
	rows, err := r.db.q(ctx).Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Vacation, error) { return scanVacation(row) })
}

func (r *VacationRepo) Get(ctx context.Context, orgID, id int64) (domain.Vacation, error) {
	return scanVacation(r.db.q(ctx).QueryRow(ctx, vacationSelect+` WHERE v.organization_id = $1 AND v.id = $2`, orgID, id))
}

func (r *VacationRepo) List(ctx context.Context, orgID int64, f domain.VacationFilter) ([]domain.Vacation, error) {
	where := []string{"v.organization_id = $1"}
	args := []any{orgID}
	arg := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	if f.UserID != 0 {
		where = append(where, "v.user_id = "+arg(f.UserID))
	}
	if f.Year != 0 {
		from, to := domain.NewDate(f.Year, time.January, 1), domain.NewDate(f.Year, time.December, 31)
		if f.Month >= 1 && f.Month <= 12 {
			from = domain.NewDate(f.Year, time.Month(f.Month), 1)
			to = from.AddDays(32)
			to = domain.NewDate(to.Year(), to.Month(), 1).AddDays(-1)
		}
		where = append(where, "v.start_day <= "+arg(to.Time)+" AND v.end_day >= "+arg(from.Time))
	}
	if f.Status != "" {
		where = append(where, "v.status = "+arg(f.Status))
	}
	if f.Type != "" {
		where = append(where, "v.type = "+arg(f.Type))
	}
	if q := strings.TrimSpace(f.RequesterName); q != "" {
		where = append(where, "u.name ILIKE "+arg("%"+escapeLike(q)+"%"))
	}
	cond := " WHERE " + strings.Join(where, " AND ")
	return r.collect(ctx, vacationSelect+cond+` ORDER BY v.start_day DESC, v.id DESC LIMIT 500`, args...)
}

func (r *VacationRepo) ToReview(ctx context.Context, orgID, reviewerID int64) ([]domain.Vacation, error) {
	return r.collect(ctx, vacationSelect+` WHERE v.organization_id = $1 AND v.status = 'pending'
		AND EXISTS (SELECT 1 FROM vacation_reviewers r WHERE r.vacation_id = v.id AND r.user_id = $2) ORDER BY v.start_day`, orgID, reviewerID)
}

// Overlapping: requests of the organization's active members that touch the period.
func (r *VacationRepo) Overlapping(ctx context.Context, orgID int64, from, to domain.Date) ([]domain.Vacation, error) {
	return r.collect(ctx, vacationSelect+` WHERE v.organization_id = $1 AND v.start_day <= $3 AND v.end_day >= $2
		AND EXISTS (SELECT 1 FROM organization_members m WHERE m.organization_id = $1 AND m.user_id = v.user_id AND m.active)
		ORDER BY v.start_day`, orgID, from.Time, to.Time)
}

// ApprovedAbsences implements timetracking.Calendar.
func (r *VacationRepo) ApprovedAbsences(ctx context.Context, orgID, userID int64, from, to domain.Date) ([]domain.Vacation, error) {
	return r.collect(ctx, vacationSelect+` WHERE v.organization_id = $1 AND v.user_id = $2 AND v.status = 'approved'
		AND v.start_day <= $4 AND v.end_day >= $3`, orgID, userID, from.Time, to.Time)
}

func (r *VacationRepo) Create(ctx context.Context, orgID int64, v *domain.Vacation) error {
	if err := r.db.q(ctx).QueryRow(ctx, `INSERT INTO vacations (organization_id, user_id, type, start_day, end_day, days, comment)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id, created_at`, orgID, v.UserID, v.Type, v.Start.Time, v.End.Time, v.Days, v.Comment).
		Scan(&v.ID, &v.CreatedAt); err != nil {
		return mapErr(err)
	}
	return r.SetReviewers(ctx, v.ID, v.ReviewerIDs)
}

func (r *VacationRepo) Update(ctx context.Context, v domain.Vacation) error {
	return exec(ctx, r.db, `UPDATE vacations SET type = $2, start_day = $3, end_day = $4, days = $5, comment = $6 WHERE id = $1`,
		v.ID, v.Type, v.Start.Time, v.End.Time, v.Days, v.Comment)
}

func (r *VacationRepo) SetReviewers(ctx context.Context, id int64, reviewers []int64) error {
	if err := exec(ctx, r.db, `DELETE FROM vacation_reviewers WHERE vacation_id = $1`, id); err != nil {
		return err
	}
	return exec(ctx, r.db, `INSERT INTO vacation_reviewers (vacation_id, user_id) SELECT $1, unnest($2::bigint[]) ON CONFLICT DO NOTHING`, id, reviewers)
}

func (r *VacationRepo) Decide(ctx context.Context, id int64, status domain.VacationStatus, by int64, at time.Time) error {
	return exec(ctx, r.db, `UPDATE vacations SET status = $2, decided_by = $3, decided_at = $4 WHERE id = $1`, id, status, by, at)
}

func (r *VacationRepo) Delete(ctx context.Context, id int64) error {
	return execOne(ctx, r.db, `DELETE FROM vacations WHERE id = $1`, id)
}

// DaysTaken counts approved working days that fall in the year (a request across New Year is split).
func (r *VacationRepo) DaysTaken(ctx context.Context, orgID, userID int64, year int) (map[domain.VacationType]int, error) {
	from, to := domain.NewDate(year, time.January, 1), domain.NewDate(year, time.December, 31)
	list, err := r.ApprovedAbsences(ctx, orgID, userID, from, to)
	if err != nil {
		return nil, err
	}
	holidays := map[string]bool{}
	rows, err := r.db.q(ctx).Query(ctx, `SELECT day FROM holidays WHERE organization_id = $1 AND day BETWEEN $2 AND $3`, orgID, from.Time, to.Time)
	if err != nil {
		return nil, err
	}
	days, err := pgx.CollectRows(rows, pgx.RowTo[time.Time])
	if err != nil {
		return nil, err
	}
	for _, d := range days {
		holidays[toDate(&d).String()] = true
	}
	out := map[domain.VacationType]int{}
	for _, v := range list {
		start, end := v.Start, v.End
		if start.Before(from.Time) {
			start = from
		}
		if end.After(to.Time) {
			end = to
		}
		out[v.Type] += domain.WorkingDays(start, end, holidays)
	}
	return out, nil
}

// Reviewers: the organization's active members who may approve absences (owners, admins, managers, leads).
func (r *UserRepo) Reviewers(ctx context.Context, orgID int64) ([]domain.User, error) {
	rows, err := r.db.q(ctx).Query(ctx, usersInOrg+`
		WHERE m.active AND (m.role IN ('OWNER', 'ADMIN', 'MANAGER') OR 'LEAD' = ANY(m.addition_roles)) ORDER BY lower(users.name)`, orgID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.User, error) { return scanMember(row) })
}
