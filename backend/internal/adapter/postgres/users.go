package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"clodapp/backend/internal/domain"
)

type UserRepo struct{ db *DB }

func NewUserRepo(db *DB) *UserRepo { return &UserRepo{db: db} }

const userColumns = `id, name, email, color, password_hash, COALESCE(avatar_key, ''), avatar_updated_at, created_at`

func scanUser(row pgx.Row) (domain.User, error) {
	var u domain.User
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.Color, &u.PasswordHash, &u.AvatarKey, &u.AvatarUpdatedAt, &u.CreatedAt)
	return u, mapErr(err)
}

func (r *UserRepo) Create(ctx context.Context, u *domain.User) error {
	row := r.db.q(ctx).QueryRow(ctx,
		`INSERT INTO users (name, email, password_hash, color) VALUES ($1, $2, $3, $4) RETURNING id, created_at`,
		u.Name, u.Email, u.PasswordHash, u.Color)
	return mapErr(row.Scan(&u.ID, &u.CreatedAt))
}

func (r *UserRepo) ByEmail(ctx context.Context, email string) (domain.User, error) {
	return scanUser(r.db.q(ctx).QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE lower(email) = lower($1)`, email))
}

func (r *UserRepo) ByID(ctx context.Context, id int64) (domain.User, error) {
	return scanUser(r.db.q(ctx).QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id))
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
