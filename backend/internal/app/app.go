// Package app is the composition root: it assembles every module into one fx application.
package app

import (
	"context"
	"log/slog"
	"os"
	"time"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"clodapp/backend/internal/adapter/bcrypthash"
	"clodapp/backend/internal/adapter/imagethumb"
	"clodapp/backend/internal/adapter/jwttoken"
	"clodapp/backend/internal/adapter/postgres"
	"clodapp/backend/internal/adapter/redisstore"
	"clodapp/backend/internal/adapter/s3store"
	"clodapp/backend/internal/adapter/searchindex"
	"clodapp/backend/internal/config"
	"clodapp/backend/internal/transport/httpapi"
	"clodapp/backend/internal/usecase/attachment"
	"clodapp/backend/internal/usecase/auth"
	"clodapp/backend/internal/usecase/board"
	"clodapp/backend/internal/usecase/dashboard"
	"clodapp/backend/internal/usecase/organization"
	"clodapp/backend/internal/usecase/project"
	"clodapp/backend/internal/usecase/user"
)

func New() *fx.App {
	return fx.New(
		fx.Provide(newLogger),
		fx.WithLogger(func(log *slog.Logger) fxevent.Logger {
			l := &fxevent.SlogLogger{Logger: log}
			l.UseLogLevel(slog.LevelDebug) // DI wiring details only at debug; errors still surface
			return l
		}),

		config.Module,

		// Adapters (infrastructure), each binding itself to the ports it implements.
		postgres.Module,
		redisstore.Module,
		searchindex.Module,
		s3store.Module,
		bcrypthash.Module,
		jwttoken.Module,
		imagethumb.Module,

		// Use cases.
		auth.Module,
		user.Module,
		project.Module,
		board.Module,
		organization.Module,
		attachment.Module,
		dashboard.Module,

		// Delivery.
		httpapi.Module,

		fx.Invoke(bootstrap),
	)
}

func newLogger() *slog.Logger {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)
	return log
}

// bootstrap runs before the server starts: demo data for an empty database, then the search index.
func bootstrap(cfg config.Config, seeder *postgres.Seeder, cards *board.CardService, log *slog.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if cfg.Seed {
		seeded, err := seeder.Seed(ctx)
		if err != nil {
			return err
		}
		if seeded {
			log.Info("demo data created", "login", "demo@example.com / demo1234")
		}
	}
	return cards.SyncIndex(ctx)
}
