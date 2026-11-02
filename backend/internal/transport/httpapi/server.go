// Package httpapi is the REST + WebSocket transport on top of Echo. Every handler registers itself
// (registerRoutes in its own file's init) in the modules.Routes namespace; the router mounts whatever
// was registered.
package httpapi

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"go.uber.org/fx"

	"github.com/DmytroFrame/oneboard/backend/internal/config"
	"github.com/DmytroFrame/oneboard/backend/internal/health"
	"github.com/DmytroFrame/oneboard/backend/internal/modules"
)

var Module = modules.Register(modules.Transport, fx.Module("http",
	fx.Provide(
		NewMiddleware,
		NewPresenter,
		fx.Annotate(NewRouter, fx.ParamTags(``, ``, ``, `group:"routes"`, `group:"health"`)),
	),
	fx.Invoke(StartServer),
))

// Routes registers a feature's endpoints. public is open to everyone, private requires a session.
// Both are mounted under /api.
type Routes interface {
	Mount(public, private *echo.Group)
}

// registerRoutes adds a handler constructor to the router. Each handler file calls it from init, so
// a new feature only touches its own file.
func registerRoutes(constructor any) {
	modules.Register(modules.Routes, fx.Provide(
		fx.Annotate(constructor, fx.As(new(Routes)), fx.ResultTags(`group:"routes"`)),
	))
}

func NewRouter(cfg config.Config, mw *Middleware, log *slog.Logger, routes []Routes, checks []health.Check) http.Handler {
	e := echo.NewWithConfig(echo.Config{
		Logger:           log,
		HTTPErrorHandler: errorHandler,
		// Groups would otherwise add catch-all 404 routes running their middleware, so an unknown path
		// under the private group would answer 401 and a wrong method 404 instead of 405.
		NoGroupAutoRegister404Routes: true,
	})
	e.Use(middleware.Recover())

	// Liveness: the process answers. Readiness: Postgres and Redis answer too (ALB / ECS health checks).
	e.GET("/healthz", func(c *echo.Context) error { return c.NoContent(http.StatusOK) })
	e.GET("/readyz", func(c *echo.Context) error {
		ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
		defer cancel()
		status, result := http.StatusOK, map[string]string{}
		for _, check := range checks {
			if err := check.Run(ctx); err != nil {
				status, result[check.Name] = http.StatusServiceUnavailable, err.Error()
			} else {
				result[check.Name] = "ok"
			}
		}
		return c.JSON(status, result)
	})

	api := e.Group("/api", mw.Log)
	private := api.Group("", mw.RequireAuth)
	for _, rt := range routes {
		rt.Mount(api, private)
	}

	// Everything else is the SPA, except unknown API paths, which stay JSON 404s.
	spa := spaHandler(cfg.StaticDir)
	fallback := func(c *echo.Context) error {
		if p := c.Request().URL.Path; p == "/api" || strings.HasPrefix(p, "/api/") {
			return errNotFound
		}
		return spa(c)
	}
	e.GET("/*", fallback)
	e.HEAD("/*", fallback)
	return e
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
