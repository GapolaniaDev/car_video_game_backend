// Package database provides a reusable PostgreSQL connection layer for
// the racing-game-backend services. It uses pgxpool under the hood and
// is intentionally small: open, ping, close, plus a HealthCheck helper
// for the /api/v1/health endpoint.
package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gustavo/racing-game-backend/internal/config"
)

// Pool is a thin wrapper around pgxpool.Pool that carries the
// underlying logger. Callers obtain one via New and Close it during
// graceful shutdown.
type Pool struct {
	*pgxpool.Pool
	log *slog.Logger
}

// New opens a pgxpool against the configured Postgres instance and
// verifies reachability within the configured timeout. It does NOT
// run migrations — that responsibility belongs to the dedicated
// migrate service (see Spec 04 / Spec 14).
func New(ctx context.Context, cfg *config.Config, log *slog.Logger) (*Pool, error) {
	if cfg == nil {
		return nil, errors.New("database: nil config")
	}
	if log == nil {
		log = slog.Default()
	}

	pcfg, err := pgxpool.ParseConfig(cfg.DatabaseURL())
	if err != nil {
		return nil, fmt.Errorf("parse pgx config: %w", err)
	}

	// Conservative pool sizing for the MVP. Adjust later when load
	// characteristics are known.
	pcfg.MaxConns = 10
	pcfg.MinConns = 2
	pcfg.MaxConnLifetime = time.Hour
	pcfg.MaxConnIdleTime = 10 * time.Minute
	pcfg.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, fmt.Errorf("open pgxpool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, cfg.PostgresReadyTimeout())
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	log.Info("postgres connection pool ready",
		slog.String("host", cfg.PostgresHost),
		slog.Int("port", cfg.PostgresPort),
		slog.String("database", cfg.PostgresDB),
	)

	return &Pool{Pool: pool, log: log}, nil
}

// Close releases all pool resources. Safe to call multiple times.
func (p *Pool) Close() {
	if p == nil || p.Pool == nil {
		return
	}
	p.Pool.Close()
	p.log.Info("postgres connection pool closed")
}

// HealthCheck pings the database with a short timeout and returns nil
// when reachable. Used by the REST API /health handler.
func (p *Pool) HealthCheck(ctx context.Context) error {
	if p == nil || p.Pool == nil {
		return errors.New("database: pool not initialized")
	}
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return p.Pool.Ping(pingCtx)
}