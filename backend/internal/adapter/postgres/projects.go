package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

// ProjectRepo stores projects and their members.
type ProjectRepo struct{ db *DB }

func NewProjectRepo(db *DB) *ProjectRepo { return &ProjectRepo{db: db} }

// projectColumns: the current user is bound to $1 (their role and pin).
const projectColumns = `
	SELECT p.id, p.organization_id, o.name, p.name, p.key, p.owner_id, COALESCE(m.role, ''),
		(SELECT COUNT(*) FROM project_members WHERE project_id = p.id),
		(SELECT COUNT(*) FROM cards WHERE project_id = p.id),
		p.created_at, p.client, p.description, p.start_date, p.active, p.billable, p.budget_hours::float8, p.is_main, p.sections,
		EXISTS (SELECT 1 FROM project_pins pp WHERE pp.user_id = $1 AND pp.project_id = p.id)
	FROM projects p
	JOIN organizations o ON o.id = p.organization_id`

// projectSelect lists projects the user bound to $1 is a member of.
const projectSelect = projectColumns + `
	JOIN project_members m ON m.project_id = p.id AND m.user_id = $1`

// projectSelectAll lists every project, with the user's role where they have one (viewAllProjects).
const projectSelectAll = projectColumns + `
	LEFT JOIN project_members m ON m.project_id = p.id AND m.user_id = $1`

func scanProject(row pgx.Row) (domain.Project, error) {
	var p domain.Project
	var start *time.Time
	var sections []byte
	err := row.Scan(&p.ID, &p.OrgID, &p.OrgName, &p.Name, &p.Key, &p.OwnerID, &p.Role, &p.MemberCount, &p.CardCount, &p.CreatedAt,
		&p.Client, &p.Description, &start, &p.Active, &p.Billable, &p.BudgetHours, &p.IsMain, &sections, &p.Pinned)
	if err != nil {
		return p, mapErr(err)
	}
	p.StartDate = toDate(start)
	return p, json.Unmarshal(sections, &p.Sections)
}

func (r *ProjectRepo) ListForUser(ctx context.Context, userID int64) ([]domain.Project, error) {
	return r.list(ctx, projectSelect, userID)
}

func (r *ProjectRepo) ListInOrg(ctx context.Context, orgID, userID int64) ([]domain.Project, error) {
	return r.list(ctx, projectSelect+` WHERE p.organization_id = $2`, userID, orgID)
}

func (r *ProjectRepo) ListAll(ctx context.Context, orgID, userID int64) ([]domain.Project, error) {
	return r.list(ctx, projectSelectAll+` WHERE p.organization_id = $2`, userID, orgID)
}

func (r *ProjectRepo) list(ctx context.Context, query string, args ...any) ([]domain.Project, error) {
	rows, err := r.db.q(ctx).Query(ctx, query+` ORDER BY lower(o.name), o.id, lower(p.name)`, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Project, error) { return scanProject(row) })
}

func (r *ProjectRepo) GetForUser(ctx context.Context, projectID, userID int64) (domain.Project, error) {
	return scanProject(r.db.q(ctx).QueryRow(ctx, projectSelect+` WHERE p.id = $2`, userID, projectID))
}

// GetAny loads a project whether or not the user is a member (role "" if not).
func (r *ProjectRepo) GetAny(ctx context.Context, projectID, userID int64) (domain.Project, error) {
	return scanProject(r.db.q(ctx).QueryRow(ctx, projectSelectAll+` WHERE p.id = $2`, userID, projectID))
}

func (r *ProjectRepo) UpdateSettings(ctx context.Context, projectID int64, s domain.ProjectSettings) error {
	sections, err := json.Marshal(s.Sections)
	if err != nil {
		return err
	}
	if s.IsMain {
		// Only one main project.
		if err := exec(ctx, r.db, `UPDATE projects SET is_main = FALSE WHERE is_main AND id <> $1`, projectID); err != nil {
			return err
		}
	}
	return exec(ctx, r.db, `UPDATE projects SET client = $2, description = $3, start_date = $4, active = $5, billable = $6,
		budget_hours = $7, is_main = $8, sections = $9 WHERE id = $1`,
		projectID, s.Client, s.Description, fromDate(s.StartDate), s.Active, s.Billable, s.BudgetHours, s.IsMain, sections)
}

func (r *ProjectRepo) SetPinned(ctx context.Context, userID, projectID int64, pinned bool) error {
	if pinned {
		return exec(ctx, r.db, `INSERT INTO project_pins (user_id, project_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, projectID)
	}
	return exec(ctx, r.db, `DELETE FROM project_pins WHERE user_id = $1 AND project_id = $2`, userID, projectID)
}

func (r *ProjectRepo) SetRole(ctx context.Context, projectID, userID int64, role domain.Role) error {
	return execOne(ctx, r.db, `UPDATE project_members SET role = $3 WHERE project_id = $1 AND user_id = $2 AND role <> 'owner'`,
		projectID, userID, role)
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
