// Package app is the composition root: it assembles the registered modules into one fx application.
package app

import (
	"context"
	"log/slog"
	"os"
	"time"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"github.com/DmytroFrame/oneboard/backend/internal/adapter/postgres"
	"github.com/DmytroFrame/oneboard/backend/internal/config"
	"github.com/DmytroFrame/oneboard/backend/internal/modules"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/board"
)

// New assembles the application from every registered module (see package modules and link.go):
// configuration, adapters, use cases, the HTTP transport and its routes.
func New() *fx.App {
	return fx.New(Options())
}

// Options is the whole application as one fx option (also what tests validate).
func Options() fx.Option {
	return fx.Options(
		fx.Provide(newLogger),
		fx.WithLogger(func(log *slog.Logger) fxevent.Logger {
			l := &fxevent.SlogLogger{Logger: log}
			l.UseLogLevel(slog.LevelDebug) // DI wiring details only at debug; errors still surface
			return l
		}),

		modules.All(),

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
