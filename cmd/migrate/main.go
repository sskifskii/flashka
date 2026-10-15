package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/skif/flashka/internal/platform/config"
)

type migration struct {
	version int64
	up      string
	down    string
}

var migrationName = regexp.MustCompile(`^(\d{6})_.+\.(up|down)\.sql$`)

func migrations() ([]migration, error) {
	entries, err := os.ReadDir("migrations")
	if err != nil {
		return nil, err
	}
	byVersion := map[int64]*migration{}
	for _, entry := range entries {
		match := migrationName.FindStringSubmatch(entry.Name())
		if entry.IsDir() || match == nil {
			continue
		}
		version, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			return nil, err
		}
		m := byVersion[version]
		if m == nil {
			m = &migration{version: version}
			byVersion[version] = m
		}
		path := filepath.Join("migrations", entry.Name())
		if match[2] == "up" {
			m.up = path
		} else {
			m.down = path
		}
	}
	result := make([]migration, 0, len(byVersion))
	for _, m := range byVersion {
		if m.up == "" || m.down == "" {
			return nil, fmt.Errorf("migration %06d must have up and down files", m.version)
		}
		result = append(result, *m)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].version < result[j].version })
	return result, nil
}

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
	cfg, err := config.Load()
	if err != nil {
		fail(err)
	}
	all, err := migrations()
	if err != nil {
		fail(err)
	}
	conn, err := pgx.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		fail(err)
	}
	defer conn.Close(context.Background())
	ctx := context.Background()
	tx, err := conn.Begin(ctx)
	if err != nil {
		fail(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(1179911142)`); err == nil {
		_, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations(version bigint PRIMARY KEY, applied_at timestamptz NOT NULL)`)
	}
	if err != nil {
		fail(err)
	}
	if direction == "down" {
		sort.Slice(all, func(i, j int) bool { return all[i].version > all[j].version })
	}
	for _, m := range all {
		var applied bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, m.version).Scan(&applied); err != nil {
			fail(err)
		}
		if (direction == "up" && applied) || (direction == "down" && !applied) {
			continue
		}
		path := m.up
		if direction == "down" {
			path = m.down
		}
		sql, readErr := os.ReadFile(path)
		if readErr != nil {
			fail(readErr)
		}
		if _, err = tx.Exec(ctx, string(sql)); err != nil {
			fail(fmt.Errorf("migration %06d %s: %w", m.version, direction, err))
		}
		if direction == "up" {
			_, err = tx.Exec(ctx, `INSERT INTO schema_migrations(version,applied_at) VALUES($1,now())`, m.version)
		} else {
			_, err = tx.Exec(ctx, `DELETE FROM schema_migrations WHERE version=$1`, m.version)
		}
		if err != nil {
			fail(err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
