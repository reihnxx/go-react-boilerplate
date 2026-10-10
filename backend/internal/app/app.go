// Package app wires configuration, stores, services and HTTP routes into a
// single http.Handler. It is used by cmd/api and by integration tests.
package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/reihnxx/go-react-boilerplate/backend/api"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/auth"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/config"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/httpserver"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/menu"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/httpx"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/mailer"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/profile"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/rbac"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/role"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/user"
)

const maxBodyBytes = 1 << 20

type App struct {
	Handler http.Handler
	Auth    *auth.Service
	Users   *user.Service
	pool    *pgxpool.Pool
	logger  *slog.Logger
}

func New(cfg config.Config, pool *pgxpool.Pool, logger *slog.Logger, mail mailer.Mailer) *App {
	permStore := rbac.NewStore(pool)
	guard := rbac.NewGuard(permStore)

	users := user.NewService(pool, permStore)
	authSvc := auth.NewService(pool, permStore, users, mail, auth.Config{
		AppName:             cfg.AppName,
		BaseURL:             cfg.BaseURL,
		SessionIdle:         cfg.SessionIdle,
		SessionMaxLife:      cfg.SessionMaxLife,
		MaxAttempts:         cfg.AuthMaxAttempts,
		IPMaxAttempts:       cfg.AuthIPMaxAttempts,
		Lockout:             cfg.AuthLockout,
		RegistrationEnabled: cfg.RegistrationEnabled,
		DefaultRoleSlug:     cfg.DefaultRoleSlug,
	}, logger)
	users.SetSessionRevoker(authSvc)
	menus := menu.NewService(pool)
	roles := role.NewService(pool, menus)
	profiles := profile.NewService(pool, authSvc)

	authH := auth.NewHandler(authSvc, cfg.CookieSecure)

	r := chi.NewRouter()
	r.Use(httpserver.RequestID)
	if cfg.TrustProxy {
		r.Use(httpserver.TrustedProxyIP)
	}
	r.Use(httpserver.AccessLog(logger))
	r.Use(httpserver.Recover(logger))
	r.Use(httpserver.SecurityHeaders(cfg.IsProduction()))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		httpx.OK(w, map[string]string{"status": "ok"})
	})
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			logger.WarnContext(r.Context(), "readiness check failed", "err", err)
			httpx.JSON(w, http.StatusServiceUnavailable, map[string]any{"data": map[string]string{"status": "unavailable"}})
			return
		}
		httpx.OK(w, map[string]string{"status": "ok"})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(httpserver.NoStore)
		r.Use(httpserver.BodyLimit(maxBodyBytes))
		r.Use(middleware.Timeout(cfg.RequestTimeout))
		r.Use(httpserver.ClientInfo)
		r.Use(authH.LoadSession)
		r.Use(auth.CSRF(cfg.AllowedOrigins))

		r.Get("/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
			_, _ = w.Write(api.OpenAPI)
		})

		authH.Routes(r, guard)
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAuth)
			user.NewHandler(users).Routes(r, guard)
			role.NewHandler(roles).Routes(r, guard)
			menu.NewHandler(menus).Routes(r, guard)
			profile.NewHandler(profiles).Routes(r, guard)
		})

		r.NotFound(httpx.Handle(func(w http.ResponseWriter, r *http.Request) error {
			return httpx.NotFound("Endpoint tidak ditemukan.")
		}))
		r.MethodNotAllowed(httpx.Handle(func(w http.ResponseWriter, r *http.Request) error {
			return &httpx.Error{Status: http.StatusMethodNotAllowed, Code: httpx.CodeMethodNotAllowed, Message: "Metode tidak diizinkan."}
		}))
	})

	return &App{Handler: r, Auth: authSvc, Users: users, pool: pool, logger: logger}
}

// RunJanitor purges expired sessions and throttle rows every interval until
// ctx is cancelled.
func (a *App) RunJanitor(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if err := a.Auth.Store().PurgeExpired(ctx); err != nil && ctx.Err() == nil {
			a.logger.Error("purge expired sessions", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
