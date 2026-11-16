package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment      string
	HTTPAddr         string
	DatabaseURL      string
	CookieName       string
	SessionTTL       time.Duration
	LastSeenInterval time.Duration
	ArgonMemory      uint32
	ArgonIterations  uint32
	ArgonParallelism uint8
	TrustedOrigin    string
	LogLevel         slog.Level
}

func Load() (Config, error) {
	c := Config{
		Environment: strings.ToLower(strings.TrimSpace(env("APP_ENV", "development"))), HTTPAddr: env("HTTP_ADDR", ":8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"), CookieName: env("SESSION_COOKIE_NAME", "flashka_session"),
		SessionTTL: 30 * 24 * time.Hour, LastSeenInterval: 15 * time.Minute,
		ArgonMemory: 19456, ArgonIterations: 2, ArgonParallelism: 1,
		TrustedOrigin: env("TRUSTED_ORIGIN", "http://localhost:8080"), LogLevel: slog.LevelInfo,
	}
	var err error
	if c.SessionTTL, err = durationEnv("SESSION_TTL", c.SessionTTL); err != nil {
		return Config{}, err
	}
	if c.LastSeenInterval, err = durationEnv("SESSION_LAST_SEEN_INTERVAL", c.LastSeenInterval); err != nil {
		return Config{}, err
	}
	if c.ArgonMemory, err = uint32Env("ARGON2_MEMORY", c.ArgonMemory); err != nil {
		return Config{}, err
	}
	if c.ArgonIterations, err = uint32Env("ARGON2_ITERATIONS", c.ArgonIterations); err != nil {
		return Config{}, err
	}
	p, err := uint32Env("ARGON2_PARALLELISM", uint32(c.ArgonParallelism))
	if err != nil || p > 255 {
		return Config{}, errors.New("ARGON2_PARALLELISM must be an integer from 1 to 255")
	}
	c.ArgonParallelism = uint8(p)
	switch strings.ToLower(env("LOG_LEVEL", "info")) {
	case "debug":
		c.LogLevel = slog.LevelDebug
	case "info":
		c.LogLevel = slog.LevelInfo
	case "warn":
		c.LogLevel = slog.LevelWarn
	case "error":
		c.LogLevel = slog.LevelError
	default:
		return Config{}, errors.New("LOG_LEVEL must be debug, info, warn or error")
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func (c Config) Validate() error {
	switch c.Environment {
	case "development", "test", "production":
	default:
		return errors.New("APP_ENV must be development, test or production")
	}
	if c.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	if c.HTTPAddr == "" {
		return errors.New("HTTP_ADDR is required")
	}
	if c.CookieName == "" || strings.ContainsAny(c.CookieName, " ;,\t\r\n") {
		return errors.New("SESSION_COOKIE_NAME is invalid")
	}
	if c.SessionTTL < time.Hour || c.SessionTTL > 90*24*time.Hour {
		return errors.New("SESSION_TTL must be between 1h and 2160h")
	}
	if c.LastSeenInterval < time.Minute || c.LastSeenInterval > 24*time.Hour {
		return errors.New("SESSION_LAST_SEEN_INTERVAL must be between 1m and 24h")
	}
	if c.ArgonMemory < 19456 {
		return errors.New("ARGON2_MEMORY must be at least 19456 KiB")
	}
	if c.ArgonIterations < 2 {
		return errors.New("ARGON2_ITERATIONS must be at least 2")
	}
	if c.ArgonParallelism < 1 {
		return errors.New("ARGON2_PARALLELISM must be at least 1")
	}
	u, err := url.Parse(c.TrustedOrigin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("TRUSTED_ORIGIN must be an http(s) origin without path")
	}
	if c.Environment == "production" && u.Scheme != "https" {
		return errors.New("production TRUSTED_ORIGIN must use https")
	}
	return nil
}

func env(k, v string) string {
	if x := os.Getenv(k); x != "" {
		return x
	}
	return v
}
func durationEnv(k string, d time.Duration) (time.Duration, error) {
	x := os.Getenv(k)
	if x == "" {
		return d, nil
	}
	v, e := time.ParseDuration(x)
	if e != nil {
		return 0, fmt.Errorf("%s: %w", k, e)
	}
	return v, nil
}
func uint32Env(k string, d uint32) (uint32, error) {
	x := os.Getenv(k)
	if x == "" {
		return d, nil
	}
	v, e := strconv.ParseUint(x, 10, 32)
	if e != nil {
		return 0, fmt.Errorf("%s must be an integer", k)
	}
	return uint32(v), nil
}
