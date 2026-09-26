package enrolment_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/enrolment"
	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/db/pgtest"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
	"github.com/Fikhri02/wikipetia/internal/user"
)

func setup(t *testing.T) (*enrolment.Service, context.Context, uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx, runner := pgtest.NewRunner(t)

	store := user.Store{}
	var learner uuid.UUID
	require.NoError(t, runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		var err error
		learner, err = store.EnsureBySubject(ctx, q, "dev|learner", "learner")
		return err
	}))

	created, err := course.New(runner).Create(ctx, learner, course.CreateInput{Title: "Chemistry"})
	require.NoError(t, err)

	return enrolment.New(runner), ctx, learner, created.ID
}

func TestEnrolSelfStudy(t *testing.T) {
	svc, ctx, learner, courseID := setup(t)

	e, err := svc.Enrol(ctx, learner, courseID, enrolment.ModeSelfStudy)
	require.NoError(t, err)
	assert.Equal(t, enrolment.ModeSelfStudy, e.Mode)
	assert.Equal(t, courseID, e.CourseID)
	assert.False(t, e.StartedAt.IsZero(), "started_at is fixed at enrolment")
}

func TestEnrolTwiceIsAConflict(t *testing.T) {
	svc, ctx, learner, courseID := setup(t)

	_, err := svc.Enrol(ctx, learner, courseID, enrolment.ModeSelfStudy)
	require.NoError(t, err)

	_, err = svc.Enrol(ctx, learner, courseID, enrolment.ModeSelfStudy)
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperr.ErrConflict),
		"the partial unique index must surface as a 409, not a 500")
}

func TestEnrolRejectsClassroomModeInSlice1(t *testing.T) {
	svc, ctx, learner, courseID := setup(t)

	_, err := svc.Enrol(ctx, learner, courseID, enrolment.ModeClassroom)
	assert.True(t, errors.Is(err, apperr.ErrValidation),
		"classroom mode needs a class, which does not exist until slice 6")
}

func TestEnrolRejectsUnknownMode(t *testing.T) {
	svc, ctx, learner, courseID := setup(t)
	_, err := svc.Enrol(ctx, learner, courseID, "telepathy")
	assert.True(t, errors.Is(err, apperr.ErrValidation))
}

func TestEnrolRejectsMissingCourse(t *testing.T) {
	svc, ctx, learner, _ := setup(t)
	_, err := svc.Enrol(ctx, learner, uuid.New(), enrolment.ModeSelfStudy)
	assert.True(t, errors.Is(err, apperr.ErrNotFound))
}

func TestListMineCarriesCourseTitle(t *testing.T) {
	svc, ctx, learner, courseID := setup(t)

	_, err := svc.Enrol(ctx, learner, courseID, enrolment.ModeSelfStudy)
	require.NoError(t, err)

	list, err := svc.ListMine(ctx, learner)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "Chemistry", list[0].CourseTitle)
}
