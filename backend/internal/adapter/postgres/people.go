package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

// ProfileRepo stores each organization's profiles of its members; a member without a profile row
// gets the defaults.
type ProfileRepo struct{ db *DB }

func NewProfileRepo(db *DB) *ProfileRepo { return &ProfileRepo{db: db} }

const profileColumns = userColumns + `, ` + memberColumns + `, COALESCE(p.name_cyrillic, ''), p.birthday, p.start_date, p.trial_until,
	COALESCE(p.working_hours, 8), COALESCE(p.billable, FALSE), COALESCE(p.details, '{}'), COALESCE(p.finance, '{}')`

// profileFrom: members of the organization bound to $1 and its profiles of them.
const profileFrom = ` FROM users JOIN organization_members m ON m.user_id = users.id AND m.organization_id = $1
	LEFT JOIN profiles p ON p.user_id = users.id AND p.organization_id = $1`

func scanProfile(row pgx.Row) (domain.Profile, error) {
	var p domain.Profile
	var m domain.Membership
	var birthday, start, trial *time.Time
	var details, finance []byte
	err := row.Scan(append(memberFields(&p.User, &m), &p.NameCyrillic, &birthday, &start, &trial, &p.WorkingHours, &p.Billable, &details, &finance)...)
	if err != nil {
		return p, mapErr(err)
	}
	u := &p.User // lists show the role of inactive members too
	u.OrgID, u.Role, u.AdditionRoles, u.Active, u.DismissedAt, u.DismissalReason =
		m.OrgID, m.Role, m.AdditionRoles, m.Active, m.DismissedAt, m.DismissalReason
	p.Birthday, p.StartDate, p.TrialUntil = toDate(birthday), toDate(start), toDate(trial)
	if err := json.Unmarshal(details, &p.Details); err != nil {
		return p, err
	}
	return p, json.Unmarshal(finance, &p.Finance)
}

func toDate(t *time.Time) *domain.Date {
	if t == nil {
		return nil
	}
	d := domain.NewDate(t.Year(), t.Month(), t.Day())
	return &d
}

func fromDate(d *domain.Date) *time.Time {
	if d == nil {
		return nil
	}
	return &d.Time
}

func (r *ProfileRepo) Get(ctx context.Context, orgID, userID int64) (domain.Profile, error) {
	return scanProfile(r.db.q(ctx).QueryRow(ctx, `SELECT `+profileColumns+profileFrom+` WHERE users.id = $2`, orgID, userID))
}

func (r *ProfileRepo) List(ctx context.Context, orgID int64, f domain.PersonFilter) (domain.PersonPage, error) {
	where := []string{"TRUE"}
	args := []any{orgID}
	arg := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }

	if len(f.Roles) > 0 {
		roles := make([]string, len(f.Roles))
		for i, r := range f.Roles {
			roles[i] = string(r)
		}
		where = append(where, "m.role = ANY("+arg(roles)+")")
	} else {
		where = append(where, "m.role <> 'CANDIDATE'")
	}
	if !f.IncludeInactive {
		where = append(where, "m.active")
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		p := arg("%" + escapeLike(q) + "%")
		where = append(where, "(users.name ILIKE "+p+" OR users.email ILIKE "+p+" OR p.name_cyrillic ILIKE "+p+")")
	}
	switch f.Quick {
	case "billable":
		where = append(where, "COALESCE(p.billable, FALSE) AND m.role <> 'CONTRACTOR'")
	case "nonbillable":
		where = append(where, "NOT COALESCE(p.billable, FALSE) AND m.role <> 'CONTRACTOR'")
	case "contractors":
		where = append(where, "m.role = 'CONTRACTOR'")
	}
	if f.Specialization != "" {
		where = append(where, "p.details->>'specialization' = "+arg(f.Specialization))
	}
	if f.TechStack != "" {
		where = append(where, "p.details->'techStack' ? "+arg(f.TechStack))
	}
	if f.Grade != "" {
		where = append(where, "p.details->>'grade' = "+arg(f.Grade))
	}
	if f.EnglishLevel != "" {
		where = append(where, "p.details->>'englishLevel' = "+arg(f.EnglishLevel))
	}
	if f.City != "" {
		where = append(where, "p.details->>'city' ILIKE "+arg("%"+escapeLike(f.City)+"%"))
	}
	cond := " WHERE " + strings.Join(where, " AND ")

	var page domain.PersonPage
	if err := r.db.q(ctx).QueryRow(ctx, `SELECT COUNT(*)`+profileFrom+cond, args...).Scan(&page.Total); err != nil {
		return page, err
	}
	dir := "ASC"
	if f.Desc {
		dir = "DESC"
	}
	order := "lower(users.name) " + dir
	switch f.Sort {
	case "role":
		order = "m.role " + dir + ", lower(users.name)"
	case "startDate":
		order = "p.start_date " + dir + " NULLS LAST, lower(users.name)"
	}
	rows, err := r.db.q(ctx).Query(ctx, `SELECT `+profileColumns+profileFrom+cond+` ORDER BY `+order+
		` OFFSET `+arg(f.Offset)+` LIMIT `+arg(f.Limit), args...)
	if err != nil {
		return page, err
	}
	page.People, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Profile, error) { return scanProfile(row) })
	return page, err
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (r *ProfileRepo) Save(ctx context.Context, orgID int64, p domain.Profile) error {
	details, err := json.Marshal(p.Details)
	if err != nil {
		return err
	}
	finance, err := json.Marshal(p.Finance)
	if err != nil {
		return err
	}
	return exec(ctx, r.db, `
		INSERT INTO profiles (user_id, name_cyrillic, birthday, start_date, trial_until, working_hours, billable, details, finance, updated_at, organization_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now(), $10)
		ON CONFLICT (organization_id, user_id) DO UPDATE SET name_cyrillic = $2, birthday = $3, start_date = $4, trial_until = $5,
			working_hours = $6, billable = $7, details = $8, finance = $9, updated_at = now()`,
		p.User.ID, p.NameCyrillic, fromDate(p.Birthday), fromDate(p.StartDate), fromDate(p.TrialUntil), p.WorkingHours, p.Billable, details, finance, orgID)
}

func (r *UserRepo) Rename(ctx context.Context, userID int64, name string) error {
	return exec(ctx, r.db, `UPDATE users SET name = $2 WHERE id = $1`, userID, name)
}

// --- reference lists ---

type DirectoryRepo struct{ db *DB }

func NewDirectoryRepo(db *DB) *DirectoryRepo { return &DirectoryRepo{db: db} }

func (r *DirectoryRepo) List(ctx context.Context, orgID int64, kind domain.DirectoryKind) ([]domain.DirectoryItem, error) {
	rows, err := r.db.q(ctx).Query(ctx, `SELECT id, kind, name FROM directory_items WHERE organization_id = $1 AND kind = $2
		ORDER BY lower(name)`, orgID, kind)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.DirectoryItem, error) {
		var it domain.DirectoryItem
		return it, row.Scan(&it.ID, &it.Kind, &it.Name)
	})
}

func (r *DirectoryRepo) Create(ctx context.Context, orgID int64, kind domain.DirectoryKind, name string) (domain.DirectoryItem, error) {
	it := domain.DirectoryItem{Kind: kind, Name: name}
	err := r.db.q(ctx).QueryRow(ctx, `INSERT INTO directory_items (organization_id, kind, name) VALUES ($1, $2, $3) RETURNING id`,
		orgID, kind, name).Scan(&it.ID)
	return it, mapErr(err)
}

func (r *DirectoryRepo) Rename(ctx context.Context, orgID, id int64, name string) (domain.DirectoryItem, error) {
	it := domain.DirectoryItem{ID: id, Name: name}
	err := r.db.q(ctx).QueryRow(ctx, `UPDATE directory_items SET name = $3 WHERE organization_id = $1 AND id = $2 RETURNING kind`,
		orgID, id, name).Scan(&it.Kind)
	return it, mapErr(err)
}

func (r *DirectoryRepo) Delete(ctx context.Context, orgID, id int64) error {
	return execOne(ctx, r.db, `DELETE FROM directory_items WHERE organization_id = $1 AND id = $2`, orgID, id)
}

// execOne runs a statement that must affect a row; domain.ErrNoRows when nothing matched.
func execOne(ctx context.Context, db *DB, sql string, args ...any) error {
	tag, err := db.q(ctx).Exec(ctx, sql, args...)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNoRows
	}
	return nil
}

// --- comments and feedback ---

type NoteRepo struct{ db *DB }

func NewNoteRepo(db *DB) *NoteRepo { return &NoteRepo{db: db} }

const commentColumns = `c.id, c.user_id, c.author_id, COALESCE(a.name, ''), c.kind, c.body, c.anonymous, c.created_at`

const commentFrom = ` FROM user_comments c LEFT JOIN users a ON a.id = c.author_id`

func scanComment(row pgx.Row) (domain.PersonComment, error) {
	var c domain.PersonComment
	err := row.Scan(&c.ID, &c.UserID, &c.AuthorID, &c.AuthorName, &c.Kind, &c.Body, &c.Anonymous, &c.CreatedAt)
	return c, mapErr(err)
}

func (r *NoteRepo) Comments(ctx context.Context, orgID, userID int64) ([]domain.PersonComment, error) {
	rows, err := r.db.q(ctx).Query(ctx, `SELECT `+commentColumns+commentFrom+` WHERE c.organization_id = $1 AND c.user_id = $2
		ORDER BY c.created_at DESC`, orgID, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.PersonComment, error) { return scanComment(row) })
}

func (r *NoteRepo) Comment(ctx context.Context, orgID, id int64) (domain.PersonComment, error) {
	return scanComment(r.db.q(ctx).QueryRow(ctx, `SELECT `+commentColumns+commentFrom+` WHERE c.organization_id = $1 AND c.id = $2`, orgID, id))
}

func (r *NoteRepo) AddComment(ctx context.Context, orgID int64, c *domain.PersonComment) error {
	return mapErr(r.db.q(ctx).QueryRow(ctx, `INSERT INTO user_comments (organization_id, user_id, author_id, kind, body, anonymous)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at`, orgID, c.UserID, c.AuthorID, c.Kind, c.Body, c.Anonymous).Scan(&c.ID, &c.CreatedAt))
}

func (r *NoteRepo) DeleteComment(ctx context.Context, orgID, id int64) error {
	return execOne(ctx, r.db, `DELETE FROM user_comments WHERE organization_id = $1 AND id = $2`, orgID, id)
}

func (r *NoteRepo) Feedback(ctx context.Context, orgID, userID int64) ([]domain.Feedback, error) {
	rows, err := r.db.q(ctx).Query(ctx, `SELECT f.id, f.user_id, f.author_id, COALESCE(a.name, ''), f.start_doing, f.stop_doing,
		f.keep_doing, f.anonymous, f.created_at
		FROM user_feedback f LEFT JOIN users a ON a.id = f.author_id WHERE f.organization_id = $1 AND f.user_id = $2
		ORDER BY f.created_at DESC`, orgID, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Feedback, error) {
		var f domain.Feedback
		return f, row.Scan(&f.ID, &f.UserID, &f.AuthorID, &f.AuthorName, &f.StartDoing, &f.StopDoing, &f.KeepDoing, &f.Anonymous, &f.CreatedAt)
	})
}

func (r *NoteRepo) AddFeedback(ctx context.Context, orgID int64, f *domain.Feedback) error {
	return mapErr(r.db.q(ctx).QueryRow(ctx, `INSERT INTO user_feedback (organization_id, user_id, author_id, start_doing, stop_doing, keep_doing, anonymous)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id, created_at`, orgID, f.UserID, f.AuthorID, f.StartDoing, f.StopDoing, f.KeepDoing, f.Anonymous).
		Scan(&f.ID, &f.CreatedAt))
}

// --- documents ---

type DocumentRepo struct{ db *DB }

func NewDocumentRepo(db *DB) *DocumentRepo { return &DocumentRepo{db: db} }

const documentColumns = `id, user_id, uploader_id, name, description, type, private, link, storage_key, file_name, content_type, size, created_at`

func scanDocument(row pgx.Row) (domain.Document, error) {
	var d domain.Document
	err := row.Scan(&d.ID, &d.UserID, &d.UploaderID, &d.Name, &d.Description, &d.Type, &d.Private, &d.Link, &d.StorageKey,
		&d.FileName, &d.ContentType, &d.Size, &d.CreatedAt)
	return d, mapErr(err)
}

func (r *DocumentRepo) List(ctx context.Context, orgID, userID int64) ([]domain.Document, error) {
	rows, err := r.db.q(ctx).Query(ctx, `SELECT `+documentColumns+` FROM user_documents WHERE organization_id = $1 AND user_id = $2
		ORDER BY created_at DESC`, orgID, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Document, error) { return scanDocument(row) })
}

func (r *DocumentRepo) Get(ctx context.Context, orgID, id int64) (domain.Document, error) {
	return scanDocument(r.db.q(ctx).QueryRow(ctx, `SELECT `+documentColumns+` FROM user_documents WHERE organization_id = $1 AND id = $2`, orgID, id))
}

func (r *DocumentRepo) Create(ctx context.Context, orgID int64, d *domain.Document) error {
	return mapErr(r.db.q(ctx).QueryRow(ctx, `INSERT INTO user_documents
		(user_id, uploader_id, name, description, type, private, link, storage_key, file_name, content_type, size, organization_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) RETURNING id, created_at`,
		d.UserID, d.UploaderID, d.Name, d.Description, d.Type, d.Private, d.Link, d.StorageKey, d.FileName, d.ContentType, d.Size, orgID).
		Scan(&d.ID, &d.CreatedAt))
}

func (r *DocumentRepo) Update(ctx context.Context, d domain.Document) error {
	return exec(ctx, r.db, `UPDATE user_documents SET name = $2, description = $3, type = $4, private = $5, link = $6 WHERE id = $1`,
		d.ID, d.Name, d.Description, d.Type, d.Private, d.Link)
}

func (r *DocumentRepo) Delete(ctx context.Context, id int64) error {
	return execOne(ctx, r.db, `DELETE FROM user_documents WHERE id = $1`, id)
}
