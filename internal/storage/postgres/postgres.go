// Package postgres provides PostgreSQL persistence using pgx v5.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cory-johannsen/mud/internal/config"
)

// Pool wraps a pgx connection pool with health-check and lifecycle methods.
type Pool struct {
	pool *pgxpool.Pool
}

// NewPool creates a new PostgreSQL connection pool from the given configuration.
//
// Precondition: cfg must contain valid database connection parameters.
// Postcondition: Returns a connected Pool or a non-nil error. The pool is ready
// for queries upon successful return.
func NewPool(ctx context.Context, cfg config.DatabaseConfig) (*Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("parsing database config: %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return &Pool{pool: pool}, nil
}

// Health checks that the database is reachable within the given timeout.
//
// Precondition: The pool must not be closed.
// Postcondition: Returns nil if the database responds within the timeout.
func (p *Pool) Health(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return p.pool.Ping(ctx)
}

// Close releases all pool resources.
//
// Postcondition: The pool is no longer usable after calling Close.
func (p *Pool) Close() {
	p.pool.Close()
}

// DB returns the underlying pgxpool.Pool for use by repositories.
func (p *Pool) DB() *pgxpool.Pool {
	return p.pool
}
