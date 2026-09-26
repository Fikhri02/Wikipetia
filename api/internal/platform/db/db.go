package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
)

func NewPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}

// WithTx runs fn inside a single transaction. Every write that must be atomic
// with the events it emits goes through here.
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(q *sqlcgen.Queries) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(sqlcgen.New(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// Runner runs a function inside a transaction. Production uses PoolRunner;
// tests use pgtest.NewRunner, which reuses the test's rolled-back transaction.
// Services depend on this interface so domain tests need no live pool.
type Runner interface {
	Tx(ctx context.Context, fn func(q *sqlcgen.Queries) error) error
}

type PoolRunner struct {
	Pool *pgxpool.Pool
}

func (r PoolRunner) Tx(ctx context.Context, fn func(q *sqlcgen.Queries) error) error {
	return WithTx(ctx, r.Pool, fn)
}
