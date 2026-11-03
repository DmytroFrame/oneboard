package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

type UserRepo struct{ db *DB }

func NewUserRepo(db *DB) *UserRepo { return &UserRepo{db: db} }

const userColumns = `users.id, users.name, users.email, users.color, users.password_hash, COALESCE(users.avatar_key, ''),
	users.avatar_updated_at, users.language, users.theme, users.created_at`

// userFields are the scan targets for userColumns, for queries that select more after them.
func userFields(u *domain.User) []any {
	return []any{&u.ID, &u.Name, &u.Email, &u.Color, &u.PasswordHash, &u.AvatarKey, &u.AvatarUpdatedAt,
		&u.Settings.Language, &u.Settings.Theme, &u.CreatedAt}
}

// memberColumns are a membership's columns (organization_members aliased m), selected after userColumns.
const memberColumns = `m.organization_id, m.role, m.addition_roles, m.active, m.dismissed_at, m.dismissal_reason`

// memberFields scans a user with their membership; an inactive one grants no role (domain.User.In).
func memberFields(u *domain.User, m *domain.Membership) []any {
	return append(userFields(u), &m.OrgID, &m.Role, &m.AdditionRoles, &m.Active, &m.DismissedAt, &m.DismissalReason)
}

// usersInOrg selects people with their membership in the organization bound to $1.
const usersInOrg = `SELECT ` + userColumns + `, ` + memberColumns + `
	FROM users JOIN organization_members m ON m.user_id = users.id AND m.organization_id = $1`

// scanMember reads a row of usersInOrg (or any userColumns + memberColumns row). Unlike User.In it
// keeps the role of inactive members, so lists can show what they were.
func scanMember(row pgx.Row) (domain.User, error) {
	var u domain.User
	var m domain.Membership
	err := row.Scan(memberFields(&u, &m)...)
	u.OrgID, u.Role, u.AdditionRoles, u.Active, u.DismissedAt, u.DismissalReason =
		m.OrgID, m.Role, m.AdditionRoles, m.Active, m.DismissedAt, m.DismissalReason
	return u, mapErr(err)
}

func scanUser(row pgx.Row) (domain.User, error) {
	var u domain.User
	return u, mapErr(row.Scan(userFields(&u)...))
}

func (r *UserRepo) Create(ctx context.Context, u *domain.User) error {
	row := r.db.q(ctx).QueryRow(ctx,
		`INSERT INTO users (name, email, password_hash, color) VALUES ($1, $2, $3, $4)
		 RETURNING id, language, theme, created_at`,
		u.Name, u.Email, u.PasswordHash, u.Color)
	return mapErr(row.Scan(&u.ID, &u.Settings.Language, &u.Settings.Theme, &u.CreatedAt))
}

func (r *UserRepo) ByEmail(ctx context.Context, email string) (domain.User, error) {
	return scanUser(r.db.q(ctx).QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE lower(email) = lower($1)`, email))
}

func (r *UserRepo) ByID(ctx context.Context, id int64) (domain.User, error) {
	return scanUser(r.db.q(ctx).QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id))
}

// InOrg loads a person with their membership in the organization; domain.ErrNoRows for outsiders.
func (r *UserRepo) InOrg(ctx context.Context, orgID, userID int64) (domain.User, error) {
	return scanMember(r.db.q(ctx).QueryRow(ctx, usersInOrg+` WHERE users.id = $2`, orgID, userID))
}

// Membership is a user's place in an organization; domain.ErrNoRows for outsiders.
func (r *UserRepo) Membership(ctx context.Context, orgID, userID int64) (domain.Membership, error) {
	return membership(ctx, r.db, orgID, userID)
}

// Memberships lists the organizations a user belongs to, oldest membership first.
func (r *UserRepo) Memberships(ctx context.Context, userID int64) ([]domain.Membership, error) {
	rows, err := r.db.q(ctx).Query(ctx, `SELECT m.organization_id, o.name, m.role, m.addition_roles, m.active, m.dismissed_at,
			m.dismissal_reason, m.added_at
		FROM organization_members m JOIN organizations o ON o.id = m.organization_id
		WHERE m.user_id = $1 ORDER BY m.added_at, m.organization_id`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Membership, error) {
		var m domain.Membership
		return m, row.Scan(&m.OrgID, &m.OrgName, &m.Role, &m.AdditionRoles, &m.Active, &m.DismissedAt, &m.DismissalReason, &m.AddedAt)
	})
}

func membership(ctx context.Context, db *DB, orgID, userID int64) (domain.Membership, error) {
	var m domain.Membership
	err := db.q(ctx).QueryRow(ctx, `SELECT m.organization_id, o.name, m.role, m.addition_roles, m.active, m.dismissed_at,
			m.dismissal_reason, m.added_at
		FROM organization_members m JOIN organizations o ON o.id = m.organization_id
		WHERE m.organization_id = $1 AND m.user_id = $2`, orgID, userID).
		Scan(&m.OrgID, &m.OrgName, &m.Role, &m.AdditionRoles, &m.Active, &m.DismissedAt, &m.DismissalReason, &m.AddedAt)
	return m, mapErr(err)
}

// Join adds someone to an organization with a company role.
func (r *UserRepo) Join(ctx context.Context, orgID, userID int64, role domain.CompanyRole) error {
	return exec(ctx, r.db, `INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, $3)`, orgID, userID, role)
}

func (r *UserRepo) Count(ctx context.Context) (int, error) {
	var n int
	err := r.db.q(ctx).QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (r *UserRepo) List(ctx context.Context) ([]domain.User, error) {
	rows, err := r.db.q(ctx).Query(ctx, `SELECT `+userColumns+` FROM users ORDER BY lower(name)`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.User, error) { return scanUser(row) })
}

func (r *UserRepo) SetAvatar(ctx context.Context, userID int64, key string, at time.Time) (string, error) {
	var old string
	// The subquery reads the row before the update, giving back the key being replaced.
	err := r.db.q(ctx).QueryRow(ctx, `
		UPDATE users u SET avatar_key = NULLIF($2, ''), avatar_updated_at = $3
		FROM (SELECT id, COALESCE(avatar_key, '') AS old_key FROM users WHERE id = $1 FOR UPDATE) prev
		WHERE u.id = prev.id
		RETURNING prev.old_key`, userID, key, at).Scan(&old)
	return old, mapErr(err)
}

func (r *UserRepo) SetSettings(ctx context.Context, userID int64, s domain.Settings) error {
	return exec(ctx, r.db, `UPDATE users SET language = $2, theme = $3 WHERE id = $1`, userID, s.Language, s.Theme)
}

// SetRole sets someone's company role in an organization.
func (r *UserRepo) SetRole(ctx context.Context, orgID, userID int64, role domain.CompanyRole, additions []domain.AdditionRole) error {
	if additions == nil {
		additions = []domain.AdditionRole{}
	}
	return execOne(ctx, r.db, `UPDATE organization_members SET role = $3, addition_roles = $4 WHERE organization_id = $1 AND user_id = $2`,
		orgID, userID, role, additions)
}

func (r *UserRepo) SetPassword(ctx context.Context, userID int64, hash string) error {
	return exec(ctx, r.db, `UPDATE users SET password_hash = $2 WHERE id = $1`, userID, hash)
}

// SetActive (de)activates someone in an organization. Reactivating clears a dismissal.
func (r *UserRepo) SetActive(ctx context.Context, orgID, userID int64, active bool) error {
	return execOne(ctx, r.db, `UPDATE organization_members SET active = $3,
		dismissed_at = CASE WHEN $3 THEN NULL ELSE dismissed_at END,
		dismissal_reason = CASE WHEN $3 THEN '' ELSE dismissal_reason END
		WHERE organization_id = $1 AND user_id = $2`, orgID, userID, active)
}

func (r *UserRepo) Dismiss(ctx context.Context, orgID, userID int64, reason string, at time.Time) error {
	return execOne(ctx, r.db, `UPDATE organization_members SET active = FALSE, dismissed_at = $4, dismissal_reason = $3
		WHERE organization_id = $1 AND user_id = $2`, orgID, userID, reason, at)
}
