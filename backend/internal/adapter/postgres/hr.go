package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

// --- candidates ---

type CandidateRepo struct{ db *DB }

func NewCandidateRepo(db *DB) *CandidateRepo { return &CandidateRepo{db: db} }

// candidateSelect: every CANDIDATE account, with defaults where nothing was saved yet.
// candidateSelect: the CANDIDATE members of the organization bound to $1.
const candidateSelect = `SELECT u.id, u.name, u.email, u.color, COALESCE(u.avatar_key, ''), u.avatar_updated_at, om.role, om.addition_roles, om.active,
	COALESCE(p.details->>'specialization', ''), COALESCE(p.details->>'grade', ''),
	COALESCE(c.status, 'new'), COALESCE(c.attributes, '{}'), COALESCE(c.channel, ''),
	c.min_salary::float8, c.max_salary::float8, c.min_rate::float8, c.max_rate::float8, c.hiring_commission::float8,
	COALESCE(c.commission_receiver, ''), COALESCE(c.trello_link, ''), COALESCE(c.rejection_reason, ''),
	r.id, r.name, r.email, r.color, COALESCE(r.avatar_key, ''), r.avatar_updated_at
	FROM users u JOIN organization_members om ON om.user_id = u.id AND om.organization_id = $1 AND om.role = 'CANDIDATE'
	LEFT JOIN candidates c ON c.user_id = u.id AND c.organization_id = $1
	LEFT JOIN profiles p ON p.user_id = u.id AND p.organization_id = $1
	LEFT JOIN users r ON r.id = c.recruiter_id`

func scanCandidate(row pgx.Row) (domain.Candidate, error) {
	var c domain.Candidate
	u := &c.User
	var rID *int64
	var rName, rEmail, rColor *string
	var rKey string
	var rAt *time.Time
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.Color, &u.AvatarKey, &u.AvatarUpdatedAt, &u.Role, &u.AdditionRoles, &u.Active,
		&c.Specialization, &c.Grade, &c.Status, &c.Attributes, &c.Channel,
		&c.MinSalary, &c.MaxSalary, &c.MinRate, &c.MaxRate, &c.HiringCommission,
		&c.CommissionReceiver, &c.TrelloLink, &c.RejectionReason,
		&rID, &rName, &rEmail, &rColor, &rKey, &rAt)
	if err == nil && rID != nil {
		c.Recruiter = &domain.User{ID: *rID, Name: *rName, Email: *rEmail, Color: *rColor, AvatarKey: rKey, AvatarUpdatedAt: rAt}
	}
	return c, mapErr(err)
}

func (r *CandidateRepo) List(ctx context.Context, orgID int64, f domain.CandidateFilter) (domain.CandidatePage, error) {
	args := []any{orgID}
	arg := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	where := []string{"TRUE"}
	if q := strings.TrimSpace(f.Q); q != "" {
		like := arg("%" + escapeLike(q) + "%")
		where = append(where, "(u.name ILIKE "+like+" OR p.name_cyrillic ILIKE "+like+")")
	}
	if f.Status != "" {
		where = append(where, "COALESCE(c.status, 'new') = "+arg(f.Status))
	}
	if f.Attribute != "" {
		where = append(where, arg(f.Attribute)+" = ANY(c.attributes)")
	}
	if f.Specialization != "" {
		where = append(where, "p.details->>'specialization' = "+arg(f.Specialization))
	}
	if f.RecruiterID != 0 {
		where = append(where, "c.recruiter_id = "+arg(f.RecruiterID))
	}
	cond := " WHERE " + strings.Join(where, " AND ")
	var page domain.CandidatePage
	if err := r.db.q(ctx).QueryRow(ctx, `SELECT COUNT(*) FROM users u
		JOIN organization_members om ON om.user_id = u.id AND om.organization_id = $1 AND om.role = 'CANDIDATE'
		LEFT JOIN candidates c ON c.user_id = u.id AND c.organization_id = $1
		LEFT JOIN profiles p ON p.user_id = u.id AND p.organization_id = $1`+cond, args...).Scan(&page.Total); err != nil {
		return page, err
	}
	rows, err := r.db.q(ctx).Query(ctx, candidateSelect+cond+` ORDER BY COALESCE(c.updated_at, u.created_at) DESC, u.id DESC
		OFFSET `+arg(f.Offset)+` LIMIT `+arg(f.Limit), args...)
	if err != nil {
		return page, err
	}
	page.Candidates, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Candidate, error) { return scanCandidate(row) })
	return page, err
}

func (r *CandidateRepo) Get(ctx context.Context, orgID, userID int64) (domain.Candidate, error) {
	return scanCandidate(r.db.q(ctx).QueryRow(ctx, candidateSelect+` WHERE u.id = $2`, orgID, userID))
}

func (r *CandidateRepo) Save(ctx context.Context, orgID int64, c domain.Candidate) error {
	var recruiter *int64
	if c.Recruiter != nil {
		recruiter = &c.Recruiter.ID
	}
	return exec(ctx, r.db, `INSERT INTO candidates (user_id, status, attributes, channel, min_salary, max_salary, min_rate, max_rate,
			hiring_commission, commission_receiver, trello_link, recruiter_id, rejection_reason, organization_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (organization_id, user_id) DO UPDATE SET status = $2, attributes = $3, channel = $4, min_salary = $5, max_salary = $6,
			min_rate = $7, max_rate = $8, hiring_commission = $9, commission_receiver = $10, trello_link = $11,
			recruiter_id = $12, rejection_reason = $13, updated_at = now()`,
		c.User.ID, c.Status, c.Attributes, c.Channel, c.MinSalary, c.MaxSalary, c.MinRate, c.MaxRate,
		c.HiringCommission, c.CommissionReceiver, c.TrelloLink, recruiter, c.RejectionReason, orgID)
}

// StartIfUnset implements hr.Accounts: a hire's start date, unless HR already set one.
func (r *UserRepo) StartIfUnset(ctx context.Context, orgID, userID int64, day domain.Date) error {
	return exec(ctx, r.db, `INSERT INTO profiles (organization_id, user_id, start_date) VALUES ($1, $2, $3)
		ON CONFLICT (organization_id, user_id) DO UPDATE SET start_date = COALESCE(profiles.start_date, EXCLUDED.start_date)`,
		orgID, userID, day.Time)
}

// --- HR events ---

type HREventRepo struct{ db *DB }

func NewHREventRepo(db *DB) *HREventRepo { return &HREventRepo{db: db} }

func (r *HREventRepo) Record(ctx context.Context, e domain.HREvent) error {
	return exec(ctx, r.db, `INSERT INTO hr_events (organization_id, user_id, kind, day, from_candidate, reason, actor_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, e.OrgID, e.User.ID, e.Kind, e.Day.Time, e.FromCandidate, e.Reason, e.ActorID)
}

func (r *HREventRepo) Between(ctx context.Context, orgID int64, from, to domain.Date) ([]domain.HREvent, error) {
	rows, err := r.db.q(ctx).Query(ctx, `SELECT e.id, e.kind, e.day, e.from_candidate, e.reason, e.actor_id, e.created_at,
			u.id, u.name, u.email, u.color, COALESCE(u.avatar_key, ''), u.avatar_updated_at, COALESCE(m.role, '')
		FROM hr_events e JOIN users u ON u.id = e.user_id
		LEFT JOIN organization_members m ON m.organization_id = e.organization_id AND m.user_id = e.user_id
		WHERE e.organization_id = $1 AND e.day BETWEEN $2 AND $3 ORDER BY e.day DESC, e.id DESC`, orgID, from.Time, to.Time)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.HREvent, error) {
		var e domain.HREvent
		var day time.Time
		u := &e.User
		err := row.Scan(&e.ID, &e.Kind, &day, &e.FromCandidate, &e.Reason, &e.ActorID, &e.CreatedAt,
			&u.ID, &u.Name, &u.Email, &u.Color, &u.AvatarKey, &u.AvatarUpdatedAt, &u.Role)
		e.Day = *toDate(&day)
		return e, err
	})
}

func (r *HREventRepo) Delete(ctx context.Context, orgID, id int64) error {
	return execOne(ctx, r.db, `DELETE FROM hr_events WHERE organization_id = $1 AND id = $2`, orgID, id)
}

func (r *HREventRepo) DeleteLatest(ctx context.Context, orgID, userID int64, kind domain.HREventKind) error {
	return exec(ctx, r.db, `DELETE FROM hr_events WHERE id = (SELECT id FROM hr_events WHERE organization_id = $1 AND user_id = $2
		AND kind = $3 ORDER BY created_at DESC, id DESC LIMIT 1)`, orgID, userID, kind)
}
