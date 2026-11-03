// Package postgres implements the repositories on PostgreSQL (pgx).
package postgres

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
	"go.uber.org/fx"

	"github.com/DmytroFrame/oneboard/backend/internal/config"
	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/health"
	"github.com/DmytroFrame/oneboard/backend/internal/modules"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/absence"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/attachment"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/auth"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/backwoods"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/board"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/collab"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/dashboard"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/hr"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/organization"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/people"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/project"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/reports"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/tasks"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/timetracking"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/user"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/voice"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/wiki"
)

// Module provides the pool, the transactor and every repository bound to the ports that use it.
var Module = modules.Register(modules.Adapters, fx.Module("postgres",
	health.Provide(func(db *DB) health.Check { return health.Check{Name: "postgres", Run: db.pool.Ping} }),
	health.ProvideVersion(func(db *DB) health.Version {
		return health.Version{Name: "PostgreSQL", Get: func(ctx context.Context) (string, error) {
			var v string
			return v, db.pool.QueryRow(ctx, `SHOW server_version`).Scan(&v)
		}}
	}),
	fx.Provide(
		NewDB,
		NewUserRepo, NewProjectRepo, NewColumnRepo, NewCardRepo, NewSeeder,
		NewOrgRepo, NewOrgMemberRepo, NewInvitationRepo, NewAttachmentRepo, NewDashboardRepo,
		NewProfileRepo, NewDirectoryRepo, NewNoteRepo, NewDocumentRepo,
		NewTagRepo, NewChecklistRepo, NewCardCommentRepo, NewHistoryRepo, NewPlanningRepo,
		func(r *CardRepo) tasks.Cards { return r },
		func(r *ColumnRepo) tasks.Columns { return r },
		func(r *TagRepo) tasks.Tags { return r },
		func(r *ChecklistRepo) tasks.Checklists { return r },
		func(r *CardCommentRepo) tasks.Comments { return r },
		func(r *HistoryRepo) tasks.History { return r },
		func(r *HistoryRepo) board.History { return r },
		func(r *PlanningRepo) tasks.Planning { return r },

		NewTimelogRepo, NewHolidayRepo, NewVacationRepo, NewCompanyRepo,
		func(r *CompanyRepo) dashboard.CompanyReader { return r },
		NewReportRepo,
		func(r *ReportRepo) reports.Reader { return r },
		func(r calendar) reports.Calendar { return r },
		func(r *ColumnRepo) reports.Columns { return r },
		NewCandidateRepo, NewHREventRepo,
		func(r *CandidateRepo) hr.Candidates { return r },
		func(r *HREventRepo) hr.Log { return r },
		func(r *HREventRepo) user.HRLog { return r },
		func(r *HREventRepo) people.HRLog { return r },
		func(r *UserRepo) hr.Accounts { return r },
		NewWikiRepo, NewWikiGroupRepo, NewProjectCommentRepo,
		func(r *WikiRepo) wiki.Articles { return r },
		func(r *WikiGroupRepo) wiki.Groups { return r },
		func(r *ProjectCommentRepo) wiki.ProjectComments { return r },
		NewBackupRepo, NewInspectRepo,
		func(r *BackupRepo) backwoods.Backups { return r },
		func(r *InspectRepo) backwoods.Inspector { return r },
		func(db *DB) backwoods.Dumper { return db },
		func(r *TimelogRepo) timetracking.Logs { return r },
		func(r *CardRepo) timetracking.Cards { return r },
		func(r *ProjectRepo) timetracking.Projects { return r },
		func(r *ProfileRepo) timetracking.Profiles { return r },
		func(r calendar) timetracking.Calendar { return r },
		func(h *HolidayRepo, v *VacationRepo) calendar { return calendar{h, v} },
		func(r *HolidayRepo) absence.Holidays { return r },
		func(r *VacationRepo) absence.Vacations { return r },
		func(r *UserRepo) absence.Users { return r },
		func(db *DB) usecase.Transactor { return db },

		func(r *UserRepo) auth.Users { return r },
		func(r *UserRepo) user.Users { return r },
		func(r *UserRepo) organization.Users { return r },
		func(r *UserRepo) people.Users { return r },
		func(r *ProfileRepo) people.Profiles { return r },
		func(r *DirectoryRepo) people.Directory { return r },
		func(r *NoteRepo) people.Notes { return r },
		func(r *DocumentRepo) people.Documents { return r },

		func(r *ProjectRepo) project.Projects { return r },
		func(r *ProjectRepo) project.Members { return r },
		func(r *ProjectRepo) board.Members { return r },
		func(r *ProjectRepo) usecase.Memberships { return r },
		func(r *ProjectRepo) organization.Projects { return r },
		func(r *ProjectRepo) voice.Members { return r },
		func(r *ProjectRepo) voice.Projects { return r },

		func(r *ColumnRepo) project.Columns { return r },
		func(r *ColumnRepo) board.Columns { return r },

		func(r *CardRepo) project.Cards { return r },
		func(r *CardRepo) board.Cards { return r },
		func(r *CardRepo) attachment.Cards { return r },
		func(r *CardRepo) collab.Cards { return r },

		func(r *OrgRepo) organization.Organizations { return r },
		func(r *OrgMemberRepo) organization.Members { return r },
		func(r *OrgMemberRepo) project.OrgRoles { return r },
		func(r *InvitationRepo) organization.Invitations { return r },
		func(r *InvitationRepo) auth.Invitations { return r },
		func(r *OrgRepo) auth.Founder { return r },

		func(r *AttachmentRepo) attachment.Attachments { return r },
		func(r *AttachmentRepo) board.AttachmentFiles { return r },
		func(r *AttachmentRepo) project.AttachmentFiles { return r },
		func(r *AttachmentRepo) organization.AttachmentFiles { return r },

		func(r *DashboardRepo) dashboard.Reader { return r },
	),
))

//go:embed migrations/*.sql
var migrations embed.FS

// DB wraps the pool and implements usecase.Transactor.
type DB struct {
	pool *pgxpool.Pool
}

func NewDB(lc fx.Lifecycle, cfg config.Config, log *slog.Logger) (*DB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}
	// The container may still be starting.
	for {
		if err = pool.Ping(ctx); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			pool.Close()
			return nil, fmt.Errorf("postgres: ping: %w", err)
		case <-time.After(time.Second):
		}
	}
	if err := migrate(ctx, pool, log); err != nil {
		pool.Close()
		return nil, err
	}
	lc.Append(fx.StopHook(pool.Close))
	return &DB{pool: pool}, nil
}

func migrate(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) error {
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	sub, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return err
	}
	// Several app instances may start together (rolling deploy): only one applies migrations at a time.
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return err
	}
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, sub, goose.WithSessionLocker(locker))
	if err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	results, err := p.Up(ctx)
	if err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	for _, r := range results {
		log.Info("migration applied", "version", r.Source.Version, "file", r.Source.Path)
	}
	return nil
}

type txKey struct{}

// querier is satisfied by both the pool and a transaction.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// q returns the transaction bound to ctx by WithinTx, or the pool.
func (db *DB) q(ctx context.Context) querier {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return db.pool
}

func (db *DB) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx) // already inside a transaction: join it
	}
	return pgx.BeginFunc(ctx, db.pool, func(tx pgx.Tx) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

// mapErr translates driver errors into the domain's repository errors.
func mapErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNoRows
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return domain.ErrDuplicate
	}
	return err
}

func exec(ctx context.Context, db *DB, sql string, args ...any) error {
	_, err := db.q(ctx).Exec(ctx, sql, args...)
	return mapErr(err)
}

// calendar combines holidays and absences for time tracking and reports.
type calendar struct {
	*HolidayRepo
	*VacationRepo
}
