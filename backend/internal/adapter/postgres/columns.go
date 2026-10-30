package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

type ColumnRepo struct{ db *DB }

func NewColumnRepo(db *DB) *ColumnRepo { return &ColumnRepo{db: db} }

const columnSelect = `
	SELECT bc.id, bc.project_id, bc.name, bc.position, (SELECT COUNT(*) FROM cards WHERE column_id = bc.id)
	FROM board_columns bc`

func scanColumn(row pgx.Row) (domain.Column, error) {
	var c domain.Column
	err := row.Scan(&c.ID, &c.ProjectID, &c.Name, &c.Position, &c.CardCount)
	return c, mapErr(err)
}

func (r *ColumnRepo) List(ctx context.Context, projectID int64) ([]domain.Column, error) {
	rows, err := r.db.q(ctx).Query(ctx, columnSelect+` WHERE bc.project_id = $1 ORDER BY bc.position, bc.id`, projectID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Column, error) { return scanColumn(row) })
}

func (r *ColumnRepo) Get(ctx context.Context, columnID int64) (domain.Column, error) {
	return scanColumn(r.db.q(ctx).QueryRow(ctx, columnSelect+` WHERE bc.id = $1`, columnID))
}

func (r *ColumnRepo) Count(ctx context.Context, projectID int64) (int, error) {
	var n int
	err := r.db.q(ctx).QueryRow(ctx, `SELECT COUNT(*) FROM board_columns WHERE project_id = $1`, projectID).Scan(&n)
	return n, err
}

func (r *ColumnRepo) Create(ctx context.Context, projectID int64, name string) error {
	return exec(ctx, r.db, `
		INSERT INTO board_columns (project_id, name, position)
		VALUES ($1, $2, (SELECT COALESCE(MAX(position), -1) + 1 FROM board_columns WHERE project_id = $1))`,
		projectID, name)
}

func (r *ColumnRepo) CreateMany(ctx context.Context, projectID int64, names []string) error {
	return exec(ctx, r.db, `
		INSERT INTO board_columns (project_id, name, position)
		SELECT $1, name, ord - 1 FROM unnest($2::text[]) WITH ORDINALITY AS t(name, ord)`,
		projectID, names)
}

func (r *ColumnRepo) Rename(ctx context.Context, columnID int64, name string) error {
	return exec(ctx, r.db, `UPDATE board_columns SET name = $2 WHERE id = $1`, columnID, name)
}

func (r *ColumnRepo) Reorder(ctx context.Context, columnIDs []int64) error {
	return exec(ctx, r.db, `
		UPDATE board_columns bc SET position = t.ord - 1
		FROM unnest($1::bigint[]) WITH ORDINALITY AS t(id, ord)
		WHERE bc.id = t.id`, columnIDs)
}

func (r *ColumnRepo) Delete(ctx context.Context, columnID int64) error {
	return exec(ctx, r.db, `DELETE FROM board_columns WHERE id = $1`, columnID)
}

func (r *ColumnRepo) Lock(ctx context.Context, columnID int64) error {
	var id int64
	return mapErr(r.db.q(ctx).QueryRow(ctx, `SELECT id FROM board_columns WHERE id = $1 FOR UPDATE`, columnID).Scan(&id))
}
