// Package postgres provides the project-wide pgx connection pool and a
// transaction helper. The pool is constructed with otelpgx tracing wired in
// and pgx's internal logs routed through slog, so every query participates
// in distributed traces and structured logs without per-call boilerplate.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/multitracer"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
)

type Config struct {
	URL               string        `envconfig:"URL" required:"true"`
	MaxConns          int32         `envconfig:"MAX_CONNS" default:"25"`
	MinConns          int32         `envconfig:"MIN_CONNS" default:"2"`
	HealthCheckPeriod time.Duration `envconfig:"HEALTHCHECK_PERIOD" default:"30s"`
	ConnectTimeout    time.Duration `envconfig:"CONNECT_TIMEOUT" default:"5s"`
}

func NewPool(ctx context.Context, cfg Config, log *slog.Logger) (*pgxpool.Pool, error) {
	if cfg.URL == "" {
		return nil, errors.New("postgres: URL is required")
	}
	if log == nil {
		log = slog.Default()
	}

	pgxCfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("postgres: parse URL: %w", err)
	}
	pgxCfg.MaxConns = cfg.MaxConns
	pgxCfg.MinConns = cfg.MinConns
	pgxCfg.HealthCheckPeriod = cfg.HealthCheckPeriod
	pgxCfg.ConnConfig.ConnectTimeout = cfg.ConnectTimeout

	pgxCfg.ConnConfig.Tracer = multitracer.New(
		&tracelog.TraceLog{
			Logger:   slogAdapter{log: log},
			LogLevel: tracelog.LogLevelWarn,
		},
		otelpgx.NewTracer(),
	)

	pool, err := pgxpool.NewWithConfig(ctx, pgxCfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	return pool, nil
}

type slogAdapter struct {
	log *slog.Logger
}

func (a slogAdapter) Log(ctx context.Context, level tracelog.LogLevel, msg string, data map[string]any) {
	attrs := make([]slog.Attr, 0, len(data))
	for k, v := range data {
		attrs = append(attrs, slog.Any(k, v))
	}

	var slogLevel slog.Level
	switch level {
	case tracelog.LogLevelTrace, tracelog.LogLevelDebug:
		slogLevel = slog.LevelDebug
	case tracelog.LogLevelInfo:
		slogLevel = slog.LevelInfo
	case tracelog.LogLevelWarn:
		slogLevel = slog.LevelWarn
	case tracelog.LogLevelError:
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}
	a.log.LogAttrs(ctx, slogLevel, "pgx: "+msg, attrs...)
}
