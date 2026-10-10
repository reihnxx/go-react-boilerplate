// Command api runs the HTTP server and administrative tasks.
//
//	api serve                      start the HTTP server (default)
//	api migrate up|down|status     manage database migrations
//	api create-admin -email … -username … -name …
//	                               create the first super-admin (password from
//	                               ADMIN_PASSWORD or an interactive prompt)
//	api seed                       create development sample users (not in production)
//	api healthcheck                exit 0 if the local server's /readyz is OK (for
//	                               container health checks; needs no config)
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/reihnxx/go-react-boilerplate/backend/internal/app"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/config"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/database"
	"github.com/reihnxx/go-react-boilerplate/backend/internal/platform/mailer"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 0 && args[0] == "healthcheck" {
		return healthcheck()
	}
	if err := config.LoadDotEnv(".env"); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := newLogger(cfg)
	slog.SetDefault(logger)

	cmd := "serve"
	if len(args) > 0 {
		cmd, args = args[0], args[1:]
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch cmd {
	case "serve", "migrate", "create-admin", "seed":
	case "help", "-h", "--help":
		fmt.Println("usage: api [serve | migrate up|down|status | create-admin | seed | healthcheck]")
		return nil
	default:
		return fmt.Errorf("unknown command %q (try: api help)", cmd)
	}

	pool, err := database.Connect(ctx, cfg.DatabaseURL, cfg.DBMaxConns, nil)
	if err != nil {
		return err
	}
	defer pool.Close()

	switch cmd {
	case "migrate":
		sub := "up"
		if len(args) > 0 {
			sub = args[0]
		}
		return database.Migrate(ctx, pool, sub, os.Stdout)
	case "create-admin":
		return createAdmin(ctx, app.New(cfg, pool, logger, mailer.New(cfg.Mail, logger)), args)
	case "seed":
		if cfg.IsProduction() {
			return errors.New("seed is disabled when APP_ENV=production")
		}
		return seed(ctx, app.New(cfg, pool, logger, mailer.New(cfg.Mail, logger)))
	}

	// serve
	if cfg.MigrateOnStart {
		if err := database.Migrate(ctx, pool, "up", os.Stdout); err != nil {
			return fmt.Errorf("migrate on start: %w", err)
		}
	}
	a := app.New(cfg, pool, logger, mailer.New(cfg.Mail, logger))
	go a.RunJanitor(ctx, time.Hour)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           a.Handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      cfg.RequestTimeout + 5*time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		logger.Info("http server listening", "addr", cfg.HTTPAddr, "env", cfg.Env)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("graceful shutdown: %w", err)
		}
	}
	return nil
}

func newLogger(cfg config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	if cfg.LogFormat == "json" {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}

// healthcheck probes the running server on HTTP_ADDR. It deliberately skips
// config loading so it works in minimal images without a .env file.
func healthcheck() error {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid HTTP_ADDR %q: %w", addr, err)
	}
	client := &http.Client{Timeout: 3 * time.Second}
	res, err := client.Get("http://127.0.0.1:" + port + "/readyz")
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("readyz returned %d", res.StatusCode)
	}
	return nil
}
