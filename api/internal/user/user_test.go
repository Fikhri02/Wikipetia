package user_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/platform/db/pgtest"
	"github.com/Fikhri02/wikipetia/internal/user"
)

func TestEnsureBySubjectCreatesThenReuses(t *testing.T) {
	ctx, q := pgtest.Begin(t)
	store := user.Store{}

	first, err := store.EnsureBySubject(ctx, q, "dev|1", "Irfan")
	require.NoError(t, err)

	second, err := store.EnsureBySubject(ctx, q, "dev|1", "Irfan")
	require.NoError(t, err)

	assert.Equal(t, first, second, "the same subject must map to the same user")
}

func TestEnsureBySubjectSeparatesSubjects(t *testing.T) {
	ctx, q := pgtest.Begin(t)
	store := user.Store{}

	a, err := store.EnsureBySubject(ctx, q, "dev|a", "A")
	require.NoError(t, err)
	b, err := store.EnsureBySubject(ctx, q, "dev|b", "B")
	require.NoError(t, err)

	assert.NotEqual(t, a, b)
}

func TestHarnessIsolatesTests(t *testing.T) {
	ctx, q := pgtest.Begin(t)
	_, err := user.Store{}.EnsureBySubject(ctx, q, "dev|isolated", "X")
	require.NoError(t, err)
	// TestHarnessIsolationProof below must not see this row.
}

func TestHarnessIsolationProof(t *testing.T) {
	ctx, q := pgtest.Begin(t)
	_, err := q.GetUserBySubject(ctx, ptr("dev|isolated"))
	require.Error(t, err, "the previous test's row must have been rolled back")
}

func ptr(s string) *string { return &s }
