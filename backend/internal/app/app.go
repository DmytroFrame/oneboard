// Package app is the composition root: it assembles every module into one fx application.
package app

import (
	"context"
	"log/slog"
	"os"
	"time"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"github.com/DmytroFrame/oneboard/backend/internal/adapter/bcrypthash"
	"github.com/DmytroFrame/oneboard/backend/internal/adapter/imagethumb"
	"github.com/DmytroFrame/oneboard/backend/internal/adapter/jwttoken"
	"github.com/DmytroFrame/oneboard/backend/internal/adapter/postgres"
	"github.com/DmytroFrame/oneboard/backend/internal/adapter/redisstore"
	"github.com/DmytroFrame/oneboard/backend/internal/adapter/s3store"
	"github.com/DmytroFrame/oneboard/backend/internal/adapter/searchindex"
	"github.com/DmytroFrame/oneboard/backend/internal/config"
	"github.com/DmytroFrame/oneboard/backend/internal/transport/httpapi"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/attachment"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/auth"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/board"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/collab"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/dashboard"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/organization"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/project"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/user"
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
		collab.Module,
		fx.Provide(func(a *auth.Service) collab.Authenticator { return a }),

		// Delivery.
		httpapi.Module,

		fx.Invoke(bootstrap),
	)
}

func newLogger(cfg config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	var handler slog.Handler = slog.NewTextHandler(os.Stderr, opts)
	if cfg.LogFormat == "json" { // one JSON object per line: what CloudWatch Logs Insights parses
		handler = slog.NewJSONHandler(os.Stderr, opts)
	}
	log := slog.New(handler)
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
