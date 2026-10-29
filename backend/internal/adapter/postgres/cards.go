package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"clodapp/backend/internal/domain"
	"clodapp/backend/internal/usecase/board"
)

type CardRepo struct{ db *DB }

func NewCardRepo(db *DB) *CardRepo { return &CardRepo{db: db} }

const cardSelect = `
	SELECT c.id, c.project_id, c.column_id, p.key || '-' || c.number, c.title, c.description, c.priority,
		c.position, c.author_id, c.assignee_id, c.created_at, c.updated_at
	FROM cards c JOIN projects p ON p.id = c.project_id`

func scanCard(row pgx.Row) (domain.Card, error) {
	var c domain.Card
	err := row.Scan(&c.ID, &c.ProjectID, &c.ColumnID, &c.Key, &c.Title, &c.Description, &c.Priority,
		&c.Position, &c.AuthorID, &c.AssigneeID, &c.CreatedAt, &c.UpdatedAt)
	return c, mapErr(err)
}

func (r *CardRepo) Get(ctx context.Context, cardID int64) (domain.Card, error) {
	return scanCard(r.db.q(ctx).QueryRow(ctx, cardSelect+` WHERE c.id = $1`, cardID))
}

func (r *CardRepo) List(ctx context.Context, f board.CardFilter) (domain.CardPage, error) {
	args := []any{f.ColumnID}
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	where := []string{"c.column_id = $1"}
	if f.OnlyIDs != nil {
		where = append(where, "c.id = ANY("+arg(f.OnlyIDs)+")")
	}
	switch {
	case len(f.Assignees) > 0 && f.Unassigned:
		where = append(where, "(c.assignee_id = ANY("+arg(f.Assignees)+") OR c.assignee_id IS NULL)")
	case len(f.Assignees) > 0:
		where = append(where, "c.assignee_id = ANY("+arg(f.Assignees)+")")
	case f.Unassigned:
		where = append(where, "c.assignee_id IS NULL")
	}
	filter := " WHERE " + strings.Join(where, " AND ")

	page := domain.CardPage{Cards: []domain.Card{}}
	if err := r.db.q(ctx).QueryRow(ctx, `SELECT COUNT(*) FROM cards c`+filter, args...).Scan(&page.Total); err != nil {
		return page, err
	}

	if f.After != nil {
		filter += fmt.Sprintf(" AND (c.position, c.id) > (%s, %s)", arg(f.After.Position), arg(f.After.ID))
	}
	rows, err := r.db.q(ctx).Query(ctx, cardSelect+filter+` ORDER BY c.position, c.id LIMIT `+arg(f.Limit+1), args...)
	if err != nil {
		return page, err
	}
	cards, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Card, error) { return scanCard(row) })
	if err != nil {
		return page, err
	}
	if len(cards) > f.Limit {
		cards = cards[:f.Limit]
		last := cards[len(cards)-1]
		page.NextCursor = &domain.CardCursor{Position: last.Position, ID: last.ID}
	}
	page.Cards = cards
	return page, nil
}

// Create issues the next card number in the project and appends the card to the bottom of its column.
// Run it in a transaction: the number and the insert must commit together.
func (r *CardRepo) Create(ctx context.Context, projectID, columnID, authorID int64, d domain.CardDraft) (int64, error) {
	var id int64
	err := r.db.q(ctx).QueryRow(ctx, `
		WITH seq AS (UPDATE projects SET card_seq = card_seq + 1 WHERE id = $1 RETURNING card_seq)
		INSERT INTO cards (project_id, column_id, number, title, description, priority, position, author_id, assignee_id)
		SELECT $1, $2, seq.card_seq, $3, $4, $5,
			(SELECT COALESCE(MAX(position), -1) + 1 FROM cards WHERE column_id = $2), $6, $7
		FROM seq
		RETURNING id`,
		projectID, columnID, d.Title, d.Description, d.Priority, authorID, d.AssigneeID).Scan(&id)
	return id, mapErr(err)
}

func (r *CardRepo) Update(ctx context.Context, cardID int64, d domain.CardDraft) error {
	return exec(ctx, r.db, `
		UPDATE cards SET title = $2, description = $3, priority = $4, assignee_id = $5, updated_at = now()
		WHERE id = $1`, cardID, d.Title, d.Description, d.Priority, d.AssigneeID)
}

func (r *CardRepo) ColumnOrder(ctx context.Context, columnID int64) ([]int64, error) {
	rows, err := r.db.q(ctx).Query(ctx, `SELECT id FROM cards WHERE column_id = $1 ORDER BY position, id`, columnID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[int64])
}

func (r *CardRepo) Place(ctx context.Context, cardID, columnID int64, order []int64) error {
	if err := exec(ctx, r.db, `UPDATE cards SET column_id = $2, updated_at = now() WHERE id = $1`, cardID, columnID); err != nil {
		return err
	}
	return exec(ctx, r.db, `
		UPDATE cards c SET position = t.ord - 1
		FROM unnest($1::bigint[]) WITH ORDINALITY AS t(id, ord)
		WHERE c.id = t.id AND c.position IS DISTINCT FROM t.ord - 1`, order)
}

func (r *CardRepo) MoveAll(ctx context.Context, from, to int64) error {
	// Offsetting past the target's max position keeps positions unique and preserves the moved cards' order.
	return exec(ctx, r.db, `
		UPDATE cards SET column_id = $2, updated_at = now(),
			position = position + (SELECT COALESCE(MAX(position), -1) + 1 FROM cards WHERE column_id = $2)
		WHERE column_id = $1`, from, to)
}

func (r *CardRepo) Delete(ctx context.Context, cardID int64) error {
	return exec(ctx, r.db, `DELETE FROM cards WHERE id = $1`, cardID)
}

func (r *CardRepo) UnassignUser(ctx context.Context, projectID, userID int64) error {
	return exec(ctx, r.db, `UPDATE cards SET assignee_id = NULL WHERE project_id = $1 AND assignee_id = $2`, projectID, userID)
}

func (r *CardRepo) SearchDocs(ctx context.Context, cardIDs ...int64) ([]domain.SearchDoc, error) {
	q := `SELECT c.id, c.project_id, p.key || '-' || c.number, c.title, c.description
		FROM cards c JOIN projects p ON p.id = c.project_id`
	var args []any
	if len(cardIDs) > 0 {
		q += ` WHERE c.id = ANY($1)`
		args = append(args, cardIDs)
	}
	rows, err := r.db.q(ctx).Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.SearchDoc, error) {
		var d domain.SearchDoc
		err := row.Scan(&d.ID, &d.ProjectID, &d.Key, &d.Title, &d.Description)
		return d, err
	})
}

func (r *CardRepo) CountAll(ctx context.Context) (int, error) {
	var n int
	err := r.db.q(ctx).QueryRow(ctx, `SELECT COUNT(*) FROM cards`).Scan(&n)
	return n, err
}
