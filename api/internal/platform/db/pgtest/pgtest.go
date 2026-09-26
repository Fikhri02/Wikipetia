// Package pgtest gives each test its own transaction, rolled back when the
// test ends. Tests therefore share one database without sharing state, and no
// test needs to clean up after itself.
package pgtest

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
)

var (
	once sync.Once
	pool *pgxpool.Pool
)

// Begin returns a context and a *sqlcgen.Queries bound to a transaction that
// is rolled back during test cleanup.
func Begin(t *testing.T) (context.Context, *sqlcgen.Queries) {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; run via `make test`")
	}

	ctx := context.Background()
	once.Do(func() {
		p, err := pgxpool.New(ctx, url)
		if err != nil {
			t.Fatalf("test pool: %v", err)
		}
		pool = p
	})

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	return ctx, sqlcgen.New(tx)
}

type txRunner struct {
	q *sqlcgen.Queries
}

func (r txRunner) Tx(ctx context.Context, fn func(q *sqlcgen.Queries) error) error {
	// Already inside the test's transaction; nesting is unnecessary and the
	// rollback in Begin's cleanup undoes everything.
	return fn(r.q)
}

// NewRunner gives a db.Runner backed by the test's transaction.
func NewRunner(t *testing.T) (context.Context, txRunner) {
	t.Helper()
	ctx, q := Begin(t)
	return ctx, txRunner{q: q}
}
