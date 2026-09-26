package course_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/db/pgtest"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
	"github.com/Fikhri02/wikipetia/internal/user"
)

// fixture creates a service, an owner, a stranger and one course.
// Shared by every later test in this package.
func fixture(t *testing.T) (svc *course.Service, ctx context.Context, owner, stranger, courseID uuid.UUID) {
	t.Helper()
	ctx, runner := pgtest.NewRunner(t)
	svc = course.New(runner)

	store := user.Store{}
	require.NoError(t, runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		var err error
		// NOTE: a composite literal in an `if` header is ambiguous to the Go
		// parser, so the store is built here rather than inline.
		if owner, err = store.EnsureBySubject(ctx, q, "dev|owner", "owner"); err != nil {
			return err
		}
		stranger, err = store.EnsureBySubject(ctx, q, "dev|stranger", "stranger")
		return err
	}))

	created, err := svc.Create(ctx, owner, course.CreateInput{Title: "Chemistry"})
	require.NoError(t, err)
	return svc, ctx, owner, stranger, created.ID
}

func TestCreateAndGet(t *testing.T) {
	svc, ctx, owner, _, _ := fixture(t)

	created, err := svc.Create(ctx, owner, course.CreateInput{Title: "Form 4 Chemistry"})
	require.NoError(t, err)
	assert.Equal(t, "Form 4 Chemistry", created.Title)
	assert.Equal(t, "private", created.Visibility, "courses default to private")
	assert.False(t, created.PetEnabled, "pets are opt-in, so default off")

	got, err := svc.Get(ctx, owner, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
}

func TestCreateRejectsEmptyTitle(t *testing.T) {
	svc, ctx, owner, _, _ := fixture(t)
	_, err := svc.Create(ctx, owner, course.CreateInput{Title: "  "})
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperr.ErrValidation))
}

func TestGetRejectsNonOwner(t *testing.T) {
	svc, ctx, _, stranger, courseID := fixture(t)

	_, err := svc.Get(ctx, stranger, courseID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperr.ErrForbidden), "a stranger must not read another user's course")
}

func TestGetMissingCourseIsNotFound(t *testing.T) {
	svc, ctx, _, _, _ := fixture(t)
	_, err := svc.Get(ctx, uuid.New(), uuid.New())
	assert.True(t, errors.Is(err, apperr.ErrNotFound))
}

func TestUpdateChangesOnlyProvidedFields(t *testing.T) {
	svc, ctx, owner, _, _ := fixture(t)

	created, err := svc.Create(ctx, owner, course.CreateInput{
		Title:       "Chemistry",
		Description: "original",
	})
	require.NoError(t, err)

	newTitle := "Chemistry SPM"
	updated, err := svc.Update(ctx, owner, created.ID, course.UpdateInput{Title: &newTitle})
	require.NoError(t, err)
	assert.Equal(t, "Chemistry SPM", updated.Title)
	assert.Equal(t, "original", updated.Description, "omitted fields must be untouched")
}

func TestArchiveHidesFromListAndGet(t *testing.T) {
	svc, ctx, owner, _, courseID := fixture(t)

	require.NoError(t, svc.Archive(ctx, owner, courseID))

	_, err := svc.Get(ctx, owner, courseID)
	assert.True(t, errors.Is(err, apperr.ErrNotFound), "archived courses read as absent")

	list, err := svc.ListOwned(ctx, owner)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestArchiveRejectsNonOwner(t *testing.T) {
	svc, ctx, _, stranger, courseID := fixture(t)
	err := svc.Archive(ctx, stranger, courseID)
	assert.True(t, errors.Is(err, apperr.ErrForbidden))
}
