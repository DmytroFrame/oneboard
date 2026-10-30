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
	SELECT p.id, p.organization_id, o.name, p.name, p.key, p.owner_id, m.role,
		(SELECT COUNT(*) FROM project_members WHERE project_id = p.id),
		(SELECT COUNT(*) FROM cards WHERE project_id = p.id),
		p.created_at
	FROM projects p
	JOIN organizations o ON o.id = p.organization_id
	JOIN project_members m ON m.project_id = p.id AND m.user_id = $1`

func scanProject(row pgx.Row) (domain.Project, error) {
	var p domain.Project
	err := row.Scan(&p.ID, &p.OrgID, &p.OrgName, &p.Name, &p.Key, &p.OwnerID, &p.Role, &p.MemberCount, &p.CardCount, &p.CreatedAt)
	return p, mapErr(err)
}

func (r *ProjectRepo) ListForUser(ctx context.Context, userID int64) ([]domain.Project, error) {
	rows, err := r.db.q(ctx).Query(ctx, projectSelect+` ORDER BY lower(o.name), o.id, lower(p.name)`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Project, error) { return scanProject(row) })
}

func (r *ProjectRepo) GetForUser(ctx context.Context, projectID, userID int64) (domain.Project, error) {
	return scanProject(r.db.q(ctx).QueryRow(ctx, projectSelect+` WHERE p.id = $2`, userID, projectID))
}

func (r *ProjectRepo) Create(ctx context.Context, orgID int64, name, key string, ownerID int64) (int64, error) {
	var id int64
	err := r.db.q(ctx).QueryRow(ctx,
		`INSERT INTO projects (organization_id, name, key, owner_id) VALUES ($1, $2, $3, $4) RETURNING id`,
		orgID, name, key, ownerID).Scan(&id)
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

func (r *ProjectRepo) TransferOwnership(ctx context.Context, orgID, from, to int64) error {
	if err := exec(ctx, r.db, `
		INSERT INTO project_members (project_id, user_id, role)
		SELECT id, $3, 'owner' FROM projects WHERE organization_id = $1 AND owner_id = $2
		ON CONFLICT (project_id, user_id) DO UPDATE SET role = 'owner'`, orgID, from, to); err != nil {
		return err
	}
	return exec(ctx, r.db, `UPDATE projects SET owner_id = $3 WHERE organization_id = $1 AND owner_id = $2`, orgID, from, to)
}

func (r *ProjectRepo) RemoveFromOrgProjects(ctx context.Context, orgID, userID int64) error {
	if err := exec(ctx, r.db, `
		DELETE FROM project_members
		WHERE user_id = $2 AND project_id IN (SELECT id FROM projects WHERE organization_id = $1)`, orgID, userID); err != nil {
		return err
	}
	return exec(ctx, r.db, `
		UPDATE cards SET assignee_id = NULL
		WHERE assignee_id = $2 AND project_id IN (SELECT id FROM projects WHERE organization_id = $1)`, orgID, userID)
}
