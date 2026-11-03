package postgres

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

// --- backups ---

type BackupRepo struct{ db *DB }

func NewBackupRepo(db *DB) *BackupRepo { return &BackupRepo{db: db} }

const backupSelect = `SELECT b.id, b.object_key, b.size_bytes, b.tables, b.created_at, u.id, COALESCE(u.name, ''), ''::text
	FROM backups b LEFT JOIN users u ON u.id = b.author_id`

func scanBackup(row pgx.Row) (domain.Backup, error) {
	var b domain.Backup
	var authorID *int64
	var name string
	var role domain.CompanyRole
	err := row.Scan(&b.ID, &b.Key, &b.Size, &b.Tables, &b.CreatedAt, &authorID, &name, &role)
	if err == nil && authorID != nil {
		b.Author = &domain.User{ID: *authorID, Name: name, Role: role}
	}
	return b, mapErr(err)
}

func (r *BackupRepo) Last(ctx context.Context, n int) ([]domain.Backup, error) {
	rows, err := r.db.q(ctx).Query(ctx, backupSelect+` ORDER BY b.created_at DESC, b.id DESC LIMIT $1`, n)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Backup, error) { return scanBackup(row) })
}

func (r *BackupRepo) Get(ctx context.Context, id int64) (domain.Backup, error) {
	return scanBackup(r.db.q(ctx).QueryRow(ctx, backupSelect+` WHERE b.id = $1`, id))
}

func (r *BackupRepo) Record(ctx context.Context, b *domain.Backup) error {
	var author *int64
	if b.Author != nil {
		author = &b.Author.ID
	}
	return mapErr(r.db.q(ctx).QueryRow(ctx, `INSERT INTO backups (object_key, size_bytes, tables, author_id) VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`, b.Key, b.Size, b.Tables, author).Scan(&b.ID, &b.CreatedAt))
}

// backupLock is the advisory lock key that keeps two instances from dumping at once.
const backupLock = 0x0b0a_bac0

// Dump writes a tar.gz with schema.json and one CSV per table, all read from one REPEATABLE READ
// snapshot, so the files agree with each other.
func (db *DB) Dump(ctx context.Context, w io.Writer) (int, error) {
	conn, err := db.pool.Acquire(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Release()
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // read-only
	var locked bool
	if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock($1)`, backupLock).Scan(&locked); err != nil {
		return 0, err
	}
	if !locked {
		return 0, domain.ErrBackupRunning
	}
	schema, err := inspectSchema(ctx, tx)
	if err != nil {
		return 0, err
	}
	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)
	now := time.Now()
	add := func(name string, data []byte) error {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(data)), ModTime: now}); err != nil {
			return err
		}
		_, err := tw.Write(data)
		return err
	}
	meta, _ := json.MarshalIndent(schema, "", "  ")
	if err := add("schema.json", meta); err != nil {
		return 0, err
	}
	for _, t := range schema.Tables {
		var buf strings.Builder
		// table names come from the catalog, quoted as identifiers
		if _, err := tx.Conn().PgConn().CopyTo(ctx, &buf, fmt.Sprintf(`COPY %s TO STDOUT WITH (FORMAT csv, HEADER)`, pgx.Identifier{t.Name}.Sanitize())); err != nil {
			return 0, fmt.Errorf("dump %s: %w", t.Name, err)
		}
		if err := add(t.Name+".csv", []byte(buf.String())); err != nil {
			return 0, err
		}
	}
	if err := tw.Close(); err != nil {
		return 0, err
	}
	return len(schema.Tables), gz.Close()
}

// --- inspector ---

type InspectRepo struct{ db *DB }

func NewInspectRepo(db *DB) *InspectRepo { return &InspectRepo{db: db} }

type queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// inspectSchema reads tables, columns, indexes and row counts of the public schema (no values).
func inspectSchema(ctx context.Context, q queryer) (domain.DBSchema, error) {
	var s domain.DBSchema
	if err := q.QueryRow(ctx, `SELECT current_database(),
		COALESCE((SELECT MAX(version_id) FROM goose_db_version WHERE is_applied), 0)`).Scan(&s.Database, &s.MigrationVersion); err != nil {
		return s, err
	}
	rows, err := q.Query(ctx, `SELECT c.table_name, c.column_name, c.udt_name, c.is_nullable = 'YES', c.column_default IS NOT NULL
		FROM information_schema.columns c JOIN information_schema.tables t
			ON t.table_schema = c.table_schema AND t.table_name = c.table_name AND t.table_type = 'BASE TABLE'
		WHERE c.table_schema = 'public' ORDER BY c.table_name, c.ordinal_position`)
	if err != nil {
		return s, err
	}
	byName := map[string]*domain.TableSchema{}
	var order []string
	for rows.Next() {
		var table string
		var col domain.ColumnSchema
		if err := rows.Scan(&table, &col.Name, &col.Type, &col.Nullable, &col.Default); err != nil {
			rows.Close()
			return s, err
		}
		if byName[table] == nil {
			byName[table] = &domain.TableSchema{Name: table}
			order = append(order, table)
		}
		byName[table].Columns = append(byName[table].Columns, col)
	}
	if err := rows.Err(); err != nil {
		return s, err
	}
	rows, err = q.Query(ctx, `SELECT tablename, indexname, indexdef FROM pg_indexes WHERE schemaname = 'public' ORDER BY tablename, indexname`)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var table string
		var idx domain.IndexSchema
		if err := rows.Scan(&table, &idx.Name, &idx.Definition); err != nil {
			rows.Close()
			return s, err
		}
		idx.Unique = strings.HasPrefix(idx.Definition, "CREATE UNIQUE")
		if t := byName[table]; t != nil {
			t.Indexes = append(t.Indexes, idx)
		}
	}
	if err := rows.Err(); err != nil {
		return s, err
	}
	for _, name := range order {
		t := byName[name]
		if err := q.QueryRow(ctx, `SELECT COUNT(*) FROM `+pgx.Identifier{name}.Sanitize()).Scan(&t.Rows); err != nil {
			return s, err
		}
		s.Tables = append(s.Tables, *t)
	}

	s.UsersByRole = map[domain.CompanyRole]int{}
	s.HashVersions = map[string]int{}
	// Memberships by role across all organizations; hash versions per account.
	rows, err = q.Query(ctx, `SELECT role, COUNT(*) FROM organization_members GROUP BY 1`)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var role domain.CompanyRole
		var n int
		if err := rows.Scan(&role, &n); err != nil {
			rows.Close()
			return s, err
		}
		s.UsersByRole[role] += n
	}
	if err := rows.Err(); err != nil {
		return s, err
	}
	rows, err = q.Query(ctx, `SELECT substring(password_hash for 7), COUNT(*) FROM users GROUP BY 1`)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var prefix string
		var n int
		if err := rows.Scan(&prefix, &n); err != nil {
			rows.Close()
			return s, err
		}
		s.HashVersions[hashScheme(prefix)] += n
	}
	return s, rows.Err()
}

// hashScheme names a password hash by its prefix ("$2a$10$" → "bcrypt $2a$ cost 10").
func hashScheme(prefix string) string {
	if parts := strings.Split(prefix, "$"); len(parts) >= 3 && strings.HasPrefix(parts[1], "2") {
		return fmt.Sprintf("bcrypt $%s$ cost %s", parts[1], parts[2])
	}
	return "unknown"
}

func (r *InspectRepo) Schema(ctx context.Context) (domain.DBSchema, error) {
	return inspectSchema(ctx, r.db.q(ctx))
}

// SpecialTaskProjects: projects other than the main one with tasks of a special type (vacation,
// sick day, bench…), which belong in the main project.
func (r *InspectRepo) SpecialTaskProjects(ctx context.Context, orgID int64) ([]domain.ProjectFinding, error) {
	rows, err := r.db.q(ctx).Query(ctx, `SELECT p.id, p.name, p.active, c.id, p.key || '-' || c.number, c.title, c.special_type
		FROM cards c JOIN projects p ON p.id = c.project_id
		WHERE p.organization_id = $1 AND c.special_type <> '' AND NOT p.is_main AND NOT c.is_template ORDER BY p.name, c.number`, orgID)
	if err != nil {
		return nil, err
	}
	var out []domain.ProjectFinding
	for rows.Next() {
		var f domain.ProjectFinding
		var t domain.FoundTask
		if err := rows.Scan(&f.ProjectID, &f.Name, &f.Active, &t.ID, &t.Key, &t.Title, &t.SpecialType); err != nil {
			rows.Close()
			return nil, err
		}
		if n := len(out); n > 0 && out[n-1].ProjectID == f.ProjectID {
			out[n-1].Tasks = append(out[n-1].Tasks, t)
			out[n-1].Count++
			continue
		}
		f.Tasks, f.Count = []domain.FoundTask{t}, 1
		out = append(out, f)
	}
	return out, rows.Err()
}

// CommentedProjects: projects with comments to review before moving them to the project wiki.
func (r *InspectRepo) CommentedProjects(ctx context.Context, orgID int64) ([]domain.ProjectFinding, error) {
	rows, err := r.db.q(ctx).Query(ctx, `SELECT p.id, p.name, p.active, COUNT(*) FROM project_comments c JOIN projects p ON p.id = c.project_id
		WHERE p.organization_id = $1 GROUP BY p.id ORDER BY COUNT(*) DESC, p.name`, orgID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.ProjectFinding, error) {
		var f domain.ProjectFinding
		return f, row.Scan(&f.ProjectID, &f.Name, &f.Active, &f.Count)
	})
}
