package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/skif/flashka/internal/platform/config"
	"os"
)

func main() {
	flag.Parse()
	direction := "up"
	if flag.NArg() > 0 {
		direction = flag.Arg(0)
	}
	if direction != "up" && direction != "down" {
		fmt.Fprintln(os.Stderr, "usage: migrate [up|down]")
		os.Exit(2)
	}
	cfg, e := config.Load()
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	path := "migrations/000001_authentication." + direction + ".sql"
	sql, e := os.ReadFile(path)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	conn, e := pgx.Connect(context.Background(), cfg.DatabaseURL)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	defer conn.Close(context.Background())
	ctx := context.Background()
	tx, e := conn.Begin(ctx)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(1179911142)`); e == nil {
		_, e = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations(version bigint PRIMARY KEY, applied_at timestamptz NOT NULL)`)
	}
	var applied bool
	if e == nil {
		e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=1)`).Scan(&applied)
	}
	if e == nil && ((direction == "up" && !applied) || (direction == "down" && applied)) {
		_, e = tx.Exec(ctx, string(sql))
		if e == nil && direction == "up" {
			_, e = tx.Exec(ctx, `INSERT INTO schema_migrations(version,applied_at) VALUES(1,now())`)
		} else if e == nil {
			_, e = tx.Exec(ctx, `DELETE FROM schema_migrations WHERE version=1`)
		}
	}
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
