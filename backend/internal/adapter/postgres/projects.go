package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"clodapp/backend/internal/domain"
)

// ProjectRepo stores projects and their members.
type ProjectRepo struct{ db *DB }

func NewProjectRepo(db *DB) *ProjectRepo { return &ProjectRepo{db: db} }

// projectSelect lists projects visible to the user bound to $1.
const projectSelect = `
	SELECT p.id, p.name, p.key, p.owner_id, m.role,
		(SELECT COUNT(*) FROM project_members WHERE project_id = p.id),
		(SELECT COUNT(*) FROM cards WHERE project_id = p.id),
		p.created_at
	FROM projects p
	JOIN project_members m ON m.project_id = p.id AND m.user_id = $1`

func scanProject(row pgx.Row) (domain.Project, error) {
	var p domain.Project
	err := row.Scan(&p.ID, &p.Name, &p.Key, &p.OwnerID, &p.Role, &p.MemberCount, &p.CardCount, &p.CreatedAt)
	return p, mapErr(err)
}

func (r *ProjectRepo) ListForUser(ctx context.Context, userID int64) ([]domain.Project, error) {
	rows, err := r.db.q(ctx).Query(ctx, projectSelect+` ORDER BY lower(p.name)`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Project, error) { return scanProject(row) })
}

func (r *ProjectRepo) GetForUser(ctx context.Context, projectID, userID int64) (domain.Project, error) {
	return scanProject(r.db.q(ctx).QueryRow(ctx, projectSelect+` WHERE p.id = $2`, userID, projectID))
}

func (r *ProjectRepo) Create(ctx context.Context, name, key string, ownerID int64) (int64, error) {
	var id int64
	err := r.db.q(ctx).QueryRow(ctx,
		`INSERT INTO projects (name, key, owner_id) VALUES ($1, $2, $3) RETURNING id`, name, key, ownerID).Scan(&id)
	return id, mapErr(err)
}

func (r *ProjectRepo) Rename(ctx context.Context, projectID int64, name string) error {
	return exec(ctx, r.db, `UPDATE projects SET name = $2 WHERE id = $1`, projectID, name)
}

func (r *ProjectRepo) Delete(ctx context.Context, projectID int64) error {
	return exec(ctx, r.db, `DELETE FROM projects WHERE id = $1`, projectID)
}

func (r *ProjectRepo) Role(ctx context.Context, projectID, userID int64) (domain.Role, error) {
	var role domain.Role
	err := r.db.q(ctx).QueryRow(ctx,
		`SELECT role FROM project_members WHERE project_id = $1 AND user_id = $2`, projectID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RoleNone, nil
	}
	return role, err
}

func (r *ProjectRepo) List(ctx context.Context, projectID int64) ([]domain.Member, error) {
	rows, err := r.db.q(ctx).Query(ctx, `
		SELECT u.id, u.name, u.email, u.color, COALESCE(u.avatar_key, ''), u.avatar_updated_at, m.role,
			(SELECT COUNT(*) FROM cards WHERE project_id = m.project_id AND assignee_id = u.id)
		FROM project_members m JOIN users u ON u.id = m.user_id
		WHERE m.project_id = $1
		ORDER BY m.role = 'owner' DESC, lower(u.name)`, projectID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Member, error) {
		var m domain.Member
		err := row.Scan(&m.ID, &m.Name, &m.Email, &m.Color, &m.AvatarKey, &m.AvatarUpdatedAt, &m.Role, &m.AssignedCount)
		return m, err
	})
}

func (r *ProjectRepo) Add(ctx context.Context, projectID, userID int64, role domain.Role) error {
	return exec(ctx, r.db, `INSERT INTO project_members (project_id, user_id, role) VALUES ($1, $2, $3)`, projectID, userID, role)
}

func (r *ProjectRepo) Remove(ctx context.Context, projectID, userID int64) error {
	return exec(ctx, r.db, `DELETE FROM project_members WHERE project_id = $1 AND user_id = $2`, projectID, userID)
}
