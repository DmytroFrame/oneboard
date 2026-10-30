// Package httpapi is the REST transport on top of chi. Each feature contributes a Routes
// implementation through the "routes" fx group; the server mounts them all.
package httpapi

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/fx"

	"github.com/DmytroFrame/oneboard/backend/internal/config"
	"github.com/DmytroFrame/oneboard/backend/internal/health"
)

var Module = fx.Module("http",
	fx.Provide(
		NewMiddleware,
		NewPresenter,
		asRoutes(NewAuthHandler),
		asRoutes(NewUserHandler),
		asRoutes(NewProjectHandler),
		asRoutes(NewColumnHandler),
		asRoutes(NewCardHandler),
		asRoutes(NewOrgHandler),
		asRoutes(NewAttachmentHandler),
		asRoutes(NewDashboardHandler),
		asRoutes(NewHub),
		fx.Annotate(NewRouter, fx.ParamTags(``, ``, `group:"routes"`, `group:"health"`)),
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

func NewRouter(cfg config.Config, mw *Middleware, routes []Routes, checks []health.Check) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RealIP, middleware.Recoverer)

	// Liveness: the process answers. Readiness: Postgres and Redis answer too (ALB / ECS health checks).
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		status, result := http.StatusOK, map[string]string{}
		for _, c := range checks {
			if err := c.Run(ctx); err != nil {
				status, result[c.Name] = http.StatusServiceUnavailable, err.Error()
			} else {
				result[c.Name] = "ok"
			}
		}
		writeJSON(w, status, result)
	})

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

// StartServer serves plain HTTP on cfg.Addr, or — when a certificate is configured and found —
// HTTPS on cfg.HTTPSAddr plus a redirect-to-HTTPS listener on cfg.HTTPAddr.
func StartServer(lc fx.Lifecycle, cfg config.Config, handler http.Handler, log *slog.Logger) error {
	if cfg.TLSProblem != "" {
		log.Warn("TLS disabled, serving plain HTTP", "reason", cfg.TLSProblem)
	}
	if !cfg.TLSEnabled {
		serve(lc, log, &http.Server{Addr: cfg.Addr, Handler: handler, ReadHeaderTimeout: 10 * time.Second}, nil, "http")
		return nil
	}

	certs, err := newCertReloader(cfg.TLSCertFile, cfg.TLSKeyFile, log)
	if err != nil {
		return err
	}
	https := &http.Server{
		Addr:              cfg.HTTPSAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		TLSConfig: &tls.Config{
			MinVersion:     tls.VersionTLS12,
			GetCertificate: certs.get, // picks up renewed certificates without a restart
			NextProtos:     []string{"h2", "http/1.1"},
		},
	}
	redirect := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           redirectToHTTPS(cfg.HTTPSAddr, cfg.ACMEWebroot),
		ReadHeaderTimeout: 10 * time.Second,
	}
	serve(lc, log, https, https.TLSConfig, "https")
	serve(lc, log, redirect, nil, "http→https redirect")
	return nil
}

func serve(lc fx.Lifecycle, log *slog.Logger, srv *http.Server, tlsConfig *tls.Config, kind string) {
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			ln, err := net.Listen("tcp", srv.Addr)
			if err != nil {
				return fmt.Errorf("%s listener %s: %w", kind, srv.Addr, err)
			}
			if tlsConfig != nil {
				ln = tls.NewListener(ln, tlsConfig)
			}
			log.Info("listening", "addr", srv.Addr, "kind", kind)
			go func() {
				if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
					log.Error("http server", "kind", kind, "err", err)
				}
			}()
			return nil
		},
		OnStop: srv.Shutdown,
	})
}
