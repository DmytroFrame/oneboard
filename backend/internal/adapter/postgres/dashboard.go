package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

// DashboardRepo is a read model for the home page. Every figure only counts projects the user is in.
type DashboardRepo struct{ db *DB }

func NewDashboardRepo(db *DB) *DashboardRepo { return &DashboardRepo{db: db} }

func (r *DashboardRepo) Stats(ctx context.Context, userID int64) (domain.DashboardStats, error) {
	var s domain.DashboardStats
	err := r.db.q(ctx).QueryRow(ctx, `
		WITH mine AS (
			SELECT c.assignee_id, c.author_id, c.priority
			FROM cards c JOIN project_members pm ON pm.project_id = c.project_id AND pm.user_id = $1
		)
		SELECT
			(SELECT COUNT(*) FROM organization_members WHERE user_id = $1),
			(SELECT COUNT(*) FROM project_members WHERE user_id = $1),
			(SELECT COUNT(*) FROM mine WHERE assignee_id = $1),
			(SELECT COUNT(*) FROM mine WHERE assignee_id = $1 AND priority IN ('high', 'critical')),
			(SELECT COUNT(*) FROM mine WHERE author_id = $1)`, userID,
	).Scan(&s.Organizations, &s.Projects, &s.Assigned, &s.Urgent, &s.Created)
	return s, err
}

func (r *DashboardRepo) AssignedCards(ctx context.Context, userID int64, limit int) ([]domain.AssignedCard, error) {
	rows, err := r.db.q(ctx).Query(ctx, `SELECT `+cardColumns+`, p.name, bc.name`+cardJoins+`
		JOIN board_columns bc ON bc.id = c.column_id
		JOIN project_members pm ON pm.project_id = c.project_id AND pm.user_id = $1
		WHERE c.assignee_id = $1
		ORDER BY CASE c.priority WHEN 'critical' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END,
			c.updated_at DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.AssignedCard, error) {
		var a domain.AssignedCard
		c, err := scanCard(row, &a.ProjectName, &a.ColumnName)
		a.Card = c
		return a, err
	})
}
