package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"

	"clodapp/backend/internal/domain"
)

type AttachmentRepo struct{ db *DB }

func NewAttachmentRepo(db *DB) *AttachmentRepo { return &AttachmentRepo{db: db} }

const attachmentSelect = `
	SELECT a.id, a.card_id, c.project_id, a.uploader_id, a.file_name, a.content_type, a.size, a.kind,
		a.storage_key, COALESCE(a.thumb_key, ''), a.created_at
	FROM card_attachments a JOIN cards c ON c.id = a.card_id`

func scanAttachment(row pgx.Row) (domain.Attachment, error) {
	var a domain.Attachment
	err := row.Scan(&a.ID, &a.CardID, &a.ProjectID, &a.UploaderID, &a.FileName, &a.ContentType, &a.Size, &a.Kind,
		&a.StorageKey, &a.ThumbKey, &a.CreatedAt)
	return a, mapErr(err)
}

func (r *AttachmentRepo) ListForCard(ctx context.Context, cardID int64) ([]domain.Attachment, error) {
	rows, err := r.db.q(ctx).Query(ctx, attachmentSelect+` WHERE a.card_id = $1 ORDER BY a.id`, cardID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Attachment, error) { return scanAttachment(row) })
}

func (r *AttachmentRepo) Get(ctx context.Context, attachmentID int64) (domain.Attachment, error) {
	return scanAttachment(r.db.q(ctx).QueryRow(ctx, attachmentSelect+` WHERE a.id = $1`, attachmentID))
}

func (r *AttachmentRepo) CountForCard(ctx context.Context, cardID int64) (int, error) {
	var n int
	err := r.db.q(ctx).QueryRow(ctx, `SELECT COUNT(*) FROM card_attachments WHERE card_id = $1`, cardID).Scan(&n)
	return n, err
}

func (r *AttachmentRepo) Create(ctx context.Context, a *domain.Attachment) error {
	err := r.db.q(ctx).QueryRow(ctx, `
		INSERT INTO card_attachments (card_id, uploader_id, file_name, content_type, size, kind, storage_key, thumb_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''))
		RETURNING id, created_at, (SELECT project_id FROM cards WHERE id = $1)`,
		a.CardID, a.UploaderID, a.FileName, a.ContentType, a.Size, a.Kind, a.StorageKey, a.ThumbKey,
	).Scan(&a.ID, &a.CreatedAt, &a.ProjectID)
	return mapErr(err)
}

func (r *AttachmentRepo) Delete(ctx context.Context, attachmentID int64) error {
	return exec(ctx, r.db, `DELETE FROM card_attachments WHERE id = $1`, attachmentID)
}

// keys lists storage objects (files and thumbnails) of the attachments matched by where.
func (r *AttachmentRepo) keys(ctx context.Context, where string, arg any) ([]string, error) {
	rows, err := r.db.q(ctx).Query(ctx, `
		SELECT k FROM card_attachments a JOIN cards c ON c.id = a.card_id,
			LATERAL (VALUES (a.storage_key), (a.thumb_key)) AS t(k)
		WHERE k IS NOT NULL AND `+where, arg)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func (r *AttachmentRepo) KeysForCard(ctx context.Context, cardID int64) ([]string, error) {
	return r.keys(ctx, `a.card_id = $1`, cardID)
}

func (r *AttachmentRepo) KeysForProjects(ctx context.Context, projectIDs []int64) ([]string, error) {
	if len(projectIDs) == 0 {
		return nil, nil
	}
	return r.keys(ctx, `c.project_id = ANY($1)`, projectIDs)
}
