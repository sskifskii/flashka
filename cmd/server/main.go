package main

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/skif/flashka/internal/auth"
	"github.com/skif/flashka/internal/platform/config"
	"github.com/skif/flashka/internal/platform/httpserver"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg, e := config.Load()
	if e != nil {
		slog.Error("invalid configuration", "error", e)
		os.Exit(1)
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	pool, e := pgxpool.New(ctx, cfg.DatabaseURL)
	if e != nil {
		log.Error("database configuration failed", "error", e)
		os.Exit(1)
	}
	defer pool.Close()
	if e = pool.Ping(ctx); e != nil {
		log.Error("database unavailable", "error", e)
		os.Exit(1)
	}
	hasher := auth.PasswordHasher{Memory: cfg.ArgonMemory, Iterations: cfg.ArgonIterations, Parallelism: cfg.ArgonParallelism}
	dummy, e := hasher.Hash("flashka-dummy-password")
	if e != nil {
		log.Error("dummy password hash failed", "error", e)
		os.Exit(1)
	}
	clock := auth.RealClock{}
	service := auth.Service{Repo: auth.Repository{Pool: pool}, Hasher: hasher, Tokens: auth.TokenSource{}, Clock: clock, TTL: cfg.SessionTTL, LastSeen: cfg.LastSeenInterval, DummyHash: dummy}
	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: httpserver.New(cfg, pool, service, log), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(c)
	}()
	log.Info("server started", "addr", cfg.HTTPAddr)
	if e = srv.ListenAndServe(); e != nil && e != http.ErrServerClosed {
		log.Error("server failed", "error", e)
		os.Exit(1)
	}
}
