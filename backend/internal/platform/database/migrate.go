package database

import (
	"context"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/reihnxx/go-react-boilerplate/backend/migrations"
)

// Migrate runs a goose command ("up", "down", "status") against the embedded
// migrations. Output for "status" is written to out.
func Migrate(ctx context.Context, pool *pgxpool.Pool, command string, out io.Writer) error {
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		return fmt.Errorf("init migrations: %w", err)
	}

	switch command {
	case "up":
		results, err := provider.Up(ctx)
		for _, r := range results {
			fmt.Fprintf(out, "applied %s (%s)\n", r.Source.Path, r.Duration)
		}
		if err != nil {
			return err
		}
		if len(results) == 0 {
			fmt.Fprintln(out, "no pending migrations")
		}
	case "down":
		r, err := provider.Down(ctx)
		if err != nil {
			return err
		}
		if r != nil {
			fmt.Fprintf(out, "rolled back %s\n", r.Source.Path)
		}
	case "status":
		statuses, err := provider.Status(ctx)
		if err != nil {
			return err
		}
		for _, s := range statuses {
			applied := "pending"
			if s.State == goose.StateApplied {
				applied = "applied " + s.AppliedAt.Format("2006-01-02 15:04:05")
			}
			fmt.Fprintf(out, "%-40s %s\n", s.Source.Path, applied)
		}
	default:
		return fmt.Errorf("unknown migrate command %q (use up, down, status)", command)
	}
	return nil
}
