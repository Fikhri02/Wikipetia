package progress_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/enrolment"
	"github.com/Fikhri02/wikipetia/internal/events"
	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/db"
	"github.com/Fikhri02/wikipetia/internal/platform/db/pgtest"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
	"github.com/Fikhri02/wikipetia/internal/progress"
	"github.com/Fikhri02/wikipetia/internal/user"
)

type scene struct {
	ctx         context.Context
	runner      db.Runner
	progress    *progress.Service
	learner     uuid.UUID
	stranger    uuid.UUID
	enrolmentID uuid.UUID
	courseID    uuid.UUID
	moduleID    uuid.UUID
	reading     uuid.UUID
	video       uuid.UUID
}

func newScene(t *testing.T) scene {
	t.Helper()
	ctx, runner := pgtest.NewRunner(t)

	store := user.Store{}
	var learner, stranger uuid.UUID
	require.NoError(t, runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		var err error
		if learner, err = store.EnsureBySubject(ctx, q, "dev|learner", "learner"); err != nil {
			return err
		}
		stranger, err = store.EnsureBySubject(ctx, q, "dev|stranger", "stranger")
		return err
	}))

	courses := course.New(runner)
	c, err := courses.Create(ctx, learner, course.CreateInput{Title: "Chemistry"})
	require.NoError(t, err)
	mod, err := courses.CreateModule(ctx, learner, c.ID, "M1")
	require.NoError(t, err)

	reading, err := courses.CreateBlock(ctx, learner, mod.ID, course.CreateBlockInput{
		Title: "Reading", Type: course.BlockReading,
		Content: json.RawMessage(`{"body_md":"# Atoms"}`),
	})
	require.NoError(t, err)

	video, err := courses.CreateBlock(ctx, learner, mod.ID, course.CreateBlockInput{
		Title: "Video", Type: course.BlockVideoYouTube,
		Content: json.RawMessage(`{"video_id":"abc123"}`),
	})
	require.NoError(t, err)

	enrolments := enrolment.New(runner)
	e, err := enrolments.Enrol(ctx, learner, c.ID, enrolment.ModeSelfStudy)
	require.NoError(t, err)

	return scene{
		ctx:         ctx,
		runner:      runner,
		progress:    progress.New(runner, enrolments),
		learner:     learner,
		stranger:    stranger,
		enrolmentID: e.ID,
		courseID:    c.ID,
		moduleID:    mod.ID,
		reading:     reading.ID,
		video:       video.ID,
	}
}

func TestMarkReadingCompleteWritesProgressAndEvent(t *testing.T) {
	s := newScene(t)

	got, err := s.progress.MarkComplete(s.ctx, s.learner, s.enrolmentID, s.reading,
		progress.Signal{MarkedDone: true})
	require.NoError(t, err)
	assert.Equal(t, progress.StatusCompleted, got.Status)
	require.NotNil(t, got.CompletedAt)

	require.NoError(t, s.runner.Tx(s.ctx, func(q *sqlcgen.Queries) error {
		records, err := events.ListUnconsumed(s.ctx, q, 10)
		require.NoError(t, err)
		require.Len(t, records, 1)
		assert.Equal(t, events.KindBlockCompleted, records[0].Kind)
		return nil
	}))
}

func TestMarkCompleteIsIdempotent(t *testing.T) {
	s := newScene(t)

	first, err := s.progress.MarkComplete(s.ctx, s.learner, s.enrolmentID, s.reading,
		progress.Signal{MarkedDone: true})
	require.NoError(t, err)

	second, err := s.progress.MarkComplete(s.ctx, s.learner, s.enrolmentID, s.reading,
		progress.Signal{MarkedDone: true})
	require.NoError(t, err)

	assert.Equal(t, progress.StatusCompleted, second.Status)
	require.NotNil(t, first.CompletedAt)
	require.NotNil(t, second.CompletedAt)
	assert.Equal(t, first.CompletedAt.Unix(), second.CompletedAt.Unix(),
		"the first completion time must not be overwritten")

	require.NoError(t, s.runner.Tx(s.ctx, func(q *sqlcgen.Queries) error {
		records, err := events.ListUnconsumed(s.ctx, q, 10)
		require.NoError(t, err)
		assert.Len(t, records, 1,
			"an event fires only on a status transition, never on a repeat")
		return nil
	}))
}

func TestIncompleteSignalStoresInProgressAndEmitsNothing(t *testing.T) {
	s := newScene(t)

	got, err := s.progress.MarkComplete(s.ctx, s.learner, s.enrolmentID, s.video,
		progress.Signal{MarkedDone: true}) // wrong signal for a video
	require.NoError(t, err)
	assert.Equal(t, progress.StatusInProgress, got.Status)
	assert.Nil(t, got.CompletedAt)

	require.NoError(t, s.runner.Tx(s.ctx, func(q *sqlcgen.Queries) error {
		records, err := events.ListUnconsumed(s.ctx, q, 10)
		require.NoError(t, err)
		assert.Empty(t, records)
		return nil
	}))
}

func TestVideoCompletesOnPlayerEndEvent(t *testing.T) {
	s := newScene(t)

	got, err := s.progress.MarkComplete(s.ctx, s.learner, s.enrolmentID, s.video,
		progress.Signal{WatchedToEnd: true})
	require.NoError(t, err)
	assert.Equal(t, progress.StatusCompleted, got.Status)
}

func TestMarkCompleteRejectsForeignEnrolment(t *testing.T) {
	s := newScene(t)

	_, err := s.progress.MarkComplete(s.ctx, s.stranger, s.enrolmentID, s.reading,
		progress.Signal{MarkedDone: true})
	assert.True(t, errors.Is(err, apperr.ErrForbidden),
		"progress writes must be authorized through the enrolment's owner")
}

func TestMarkCompleteRejectsBlockFromAnotherCourse(t *testing.T) {
	s := newScene(t)

	courses := course.New(s.runner)
	other, err := courses.Create(s.ctx, s.learner, course.CreateInput{Title: "Physics"})
	require.NoError(t, err)
	mod, err := courses.CreateModule(s.ctx, s.learner, other.ID, "M1")
	require.NoError(t, err)
	foreign, err := courses.CreateBlock(s.ctx, s.learner, mod.ID, course.CreateBlockInput{
		Title: "Forces", Type: course.BlockReading, Content: json.RawMessage(`{"body_md":"x"}`),
	})
	require.NoError(t, err)

	_, err = s.progress.MarkComplete(s.ctx, s.learner, s.enrolmentID, foreign.ID,
		progress.Signal{MarkedDone: true})
	assert.True(t, errors.Is(err, apperr.ErrValidation),
		"a block must belong to the enrolment's course")
}
