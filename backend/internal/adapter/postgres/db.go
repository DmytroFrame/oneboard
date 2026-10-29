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
	"go.uber.org/fx"

	"clodapp/backend/internal/config"
	"clodapp/backend/internal/domain"
	"clodapp/backend/internal/usecase"
	"clodapp/backend/internal/usecase/auth"
	"clodapp/backend/internal/usecase/board"
	"clodapp/backend/internal/usecase/project"
	"clodapp/backend/internal/usecase/user"
)

// Module provides the pool, the transactor and every repository bound to the ports that use it.
var Module = fx.Module("postgres",
	fx.Provide(
		NewDB,
		NewUserRepo, NewProjectRepo, NewColumnRepo, NewCardRepo, NewSeeder,
		func(db *DB) usecase.Transactor { return db },

		func(r *UserRepo) auth.Users { return r },
		func(r *UserRepo) user.Users { return r },
		func(r *UserRepo) project.Users { return r },

		func(r *ProjectRepo) project.Projects { return r },
		func(r *ProjectRepo) project.Members { return r },
		func(r *ProjectRepo) board.Members { return r },

		func(r *ColumnRepo) project.Columns { return r },
		func(r *ColumnRepo) board.Columns { return r },

		func(r *CardRepo) project.Cards { return r },
		func(r *CardRepo) board.Cards { return r },
	),
)

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
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, sub)
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
