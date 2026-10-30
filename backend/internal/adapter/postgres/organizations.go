package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

// OrgRepo stores organizations, their members and invitations.
type OrgRepo struct{ db *DB }

func NewOrgRepo(db *DB) *OrgRepo { return &OrgRepo{db: db} }

// orgSelect lists organizations of the user bound to $1.
const orgSelect = `
	SELECT o.id, o.name, m.role,
		(SELECT COUNT(*) FROM organization_members WHERE organization_id = o.id),
		(SELECT COUNT(*) FROM projects WHERE organization_id = o.id),
		o.created_at
	FROM organizations o
	JOIN organization_members m ON m.organization_id = o.id AND m.user_id = $1`

func scanOrg(row pgx.Row) (domain.Organization, error) {
	var o domain.Organization
	err := row.Scan(&o.ID, &o.Name, &o.Role, &o.MemberCount, &o.ProjectCount, &o.CreatedAt)
	return o, mapErr(err)
}

func (r *OrgRepo) ListForUser(ctx context.Context, userID int64) ([]domain.Organization, error) {
	rows, err := r.db.q(ctx).Query(ctx, orgSelect+` ORDER BY lower(o.name), o.id`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Organization, error) { return scanOrg(row) })
}

func (r *OrgRepo) GetForUser(ctx context.Context, orgID, userID int64) (domain.Organization, error) {
	return scanOrg(r.db.q(ctx).QueryRow(ctx, orgSelect+` WHERE o.id = $2`, userID, orgID))
}

func (r *OrgRepo) Create(ctx context.Context, name string, createdBy int64) (int64, error) {
	var id int64
	err := r.db.q(ctx).QueryRow(ctx, `INSERT INTO organizations (name, created_by) VALUES ($1, $2) RETURNING id`, name, createdBy).Scan(&id)
	return id, mapErr(err)
}

func (r *OrgRepo) Rename(ctx context.Context, orgID int64, name string) error {
	return exec(ctx, r.db, `UPDATE organizations SET name = $2 WHERE id = $1`, orgID, name)
}

func (r *OrgRepo) Delete(ctx context.Context, orgID int64) error {
	return exec(ctx, r.db, `DELETE FROM organizations WHERE id = $1`, orgID)
}

func (r *OrgRepo) ProjectIDs(ctx context.Context, orgID int64) ([]int64, error) {
	rows, err := r.db.q(ctx).Query(ctx, `SELECT id FROM projects WHERE organization_id = $1`, orgID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[int64])
}

// OrgMemberRepo is the membership side of organizations; a separate type because
// "Role/List/Add/Remove" would clash with the organization methods of the same name.
type OrgMemberRepo struct{ db *DB }

func NewOrgMemberRepo(db *DB) *OrgMemberRepo { return &OrgMemberRepo{db: db} }

func (r *OrgMemberRepo) Role(ctx context.Context, orgID, userID int64) (domain.OrgRole, error) {
	var role domain.OrgRole
	err := r.db.q(ctx).QueryRow(ctx,
		`SELECT role FROM organization_members WHERE organization_id = $1 AND user_id = $2`, orgID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.OrgRoleNone, nil
	}
	return role, err
}

func (r *OrgMemberRepo) List(ctx context.Context, orgID int64) ([]domain.OrgMember, error) {
	rows, err := r.db.q(ctx).Query(ctx, `
		SELECT u.id, u.name, u.email, u.color, COALESCE(u.avatar_key, ''), u.avatar_updated_at, m.role, m.added_at
		FROM organization_members m JOIN users u ON u.id = m.user_id
		WHERE m.organization_id = $1
		ORDER BY CASE m.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END, lower(u.name)`, orgID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.OrgMember, error) {
		var m domain.OrgMember
		err := row.Scan(&m.ID, &m.Name, &m.Email, &m.Color, &m.AvatarKey, &m.AvatarUpdatedAt, &m.Role, &m.AddedAt)
		return m, err
	})
}

func (r *OrgMemberRepo) Add(ctx context.Context, orgID, userID int64, role domain.OrgRole) error {
	return exec(ctx, r.db, `INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, $3)`, orgID, userID, role)
}

func (r *OrgMemberRepo) SetRole(ctx context.Context, orgID, userID int64, role domain.OrgRole) error {
	return exec(ctx, r.db, `UPDATE organization_members SET role = $3 WHERE organization_id = $1 AND user_id = $2`, orgID, userID, role)
}

func (r *OrgMemberRepo) Remove(ctx context.Context, orgID, userID int64) error {
	return exec(ctx, r.db, `DELETE FROM organization_members WHERE organization_id = $1 AND user_id = $2`, orgID, userID)
}

func (r *OrgMemberRepo) CountOwners(ctx context.Context, orgID int64) (int, error) {
	var n int
	// FOR UPDATE on the owner rows keeps two concurrent demotions from leaving no owner.
	err := r.db.q(ctx).QueryRow(ctx, `
		SELECT COUNT(*) FROM (
			SELECT 1 FROM organization_members WHERE organization_id = $1 AND role = 'owner' FOR UPDATE
		) owners`, orgID).Scan(&n)
	return n, err
}

func (r *OrgMemberRepo) FirstOwnerExcept(ctx context.Context, orgID, userID int64) (int64, error) {
	var id int64
	err := r.db.q(ctx).QueryRow(ctx, `
		SELECT user_id FROM organization_members
		WHERE organization_id = $1 AND role = 'owner' AND user_id <> $2
		ORDER BY added_at, user_id LIMIT 1`, orgID, userID).Scan(&id)
	return id, mapErr(err)
}

type InvitationRepo struct{ db *DB }

func NewInvitationRepo(db *DB) *InvitationRepo { return &InvitationRepo{db: db} }

func (r *InvitationRepo) List(ctx context.Context, orgID int64) ([]domain.Invitation, error) {
	rows, err := r.db.q(ctx).Query(ctx, `
		SELECT id, organization_id, email, role, invited_by, created_at
		FROM organization_invitations WHERE organization_id = $1 ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Invitation, error) {
		var i domain.Invitation
		err := row.Scan(&i.ID, &i.OrgID, &i.Email, &i.Role, &i.InvitedBy, &i.CreatedAt)
		return i, err
	})
}

func (r *InvitationRepo) Create(ctx context.Context, orgID int64, email string, role domain.OrgRole, invitedBy int64) error {
	return exec(ctx, r.db, `
		INSERT INTO organization_invitations (organization_id, email, role, invited_by) VALUES ($1, $2, $3, $4)`,
		orgID, email, role, invitedBy)
}

func (r *InvitationRepo) Delete(ctx context.Context, orgID, invitationID int64) error {
	tag, err := r.db.q(ctx).Exec(ctx, `DELETE FROM organization_invitations WHERE organization_id = $1 AND id = $2`, orgID, invitationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNoRows
	}
	return nil
}

func (r *InvitationRepo) AcceptPending(ctx context.Context, userID int64, email string) error {
	if err := exec(ctx, r.db, `
		INSERT INTO organization_members (organization_id, user_id, role)
		SELECT organization_id, $1, role FROM organization_invitations WHERE lower(email) = lower($2)
		ON CONFLICT DO NOTHING`, userID, email); err != nil {
		return err
	}
	return exec(ctx, r.db, `DELETE FROM organization_invitations WHERE lower(email) = lower($1)`, email)
}
