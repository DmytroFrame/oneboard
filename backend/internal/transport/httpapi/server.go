// Package httpapi is the REST transport on top of chi. Each feature contributes a Routes
// implementation through the "routes" fx group; the server mounts them all.
package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/fx"

	"clodapp/backend/internal/config"
)

var Module = fx.Module("http",
	fx.Provide(
		NewMiddleware,
		asRoutes(NewAuthHandler),
		asRoutes(NewUserHandler),
		asRoutes(NewProjectHandler),
		asRoutes(NewColumnHandler),
		asRoutes(NewCardHandler),
		fx.Annotate(NewRouter, fx.ParamTags(``, ``, `group:"routes"`)),
	),
	fx.Invoke(StartServer),
)

// Routes registers a feature's endpoints. public is open to everyone, private requires a session.
type Routes interface {
	Mount(public, private chi.Router)
}

func asRoutes(constructor any) any {
	return fx.Annotate(constructor, fx.As(new(Routes)), fx.ResultTags(`group:"routes"`))
}

func NewRouter(cfg config.Config, mw *Middleware, routes []Routes) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RealIP, middleware.Recoverer)

	r.Route("/api", func(api chi.Router) {
		api.Use(mw.Log)
		public, private := api.With(), api.With(mw.RequireAuth)
		for _, rt := range routes {
			rt.Mount(public, private)
		}
		api.NotFound(func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusNotFound, "not_found", "Not found")
		})
		api.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		})
	})
	r.Handle("/*", spaHandler(cfg.StaticDir))
	return r
}

func StartServer(lc fx.Lifecycle, cfg config.Config, handler http.Handler, log *slog.Logger) {
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			ln, err := net.Listen("tcp", cfg.Addr)
			if err != nil {
				return err
			}
			log.Info("listening", "addr", cfg.Addr, "static", cfg.StaticDir)
			go func() {
				if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
					log.Error("http server", "err", err)
				}
			}()
			return nil
		},
		OnStop: srv.Shutdown,
	})
}
