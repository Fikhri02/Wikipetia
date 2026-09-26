package progress_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/progress"
)

func TestPlayerViewCarriesTheWholeCourse(t *testing.T) {
	s := newQuizScene(t)

	view, err := s.progress.Player(s.ctx, s.learner, s.enrolmentID)
	require.NoError(t, err)

	assert.Equal(t, "Chemistry", view.CourseTitle)
	assert.False(t, view.StartedAt.IsZero())
	require.Len(t, view.Modules, 1)
	assert.Equal(t, 3, view.BlocksTotal, "reading, video and quiz")
	assert.Equal(t, 0, view.BlocksCompleted)

	require.Len(t, view.Topics, 1)
	assert.Equal(t, "Atoms", view.Topics[0].Title)
	assert.Equal(t, "new", view.Topics[0].Level, "no evidence yet")
}

func TestPlayerViewNeverContainsAnswerKeys(t *testing.T) {
	s := newQuizScene(t)

	view, err := s.progress.Player(s.ctx, s.learner, s.enrolmentID)
	require.NoError(t, err)

	// Serialise the whole view and assert no answer key survives anywhere in it.
	encoded, err := json.Marshal(view)
	require.NoError(t, err)
	body := string(encoded)

	assert.NotContains(t, body, "correct_index")
	assert.NotContains(t, body, `"correct"`)
	assert.NotContains(t, body, "accepted")
	assert.False(t, strings.Contains(body, `"answer"`),
		"the player view must never carry an answer field — devtools would defeat every quiz")

	// And the questions are still usable: prompts are present.
	var found bool
	for _, mod := range view.Modules {
		for _, block := range mod.Blocks {
			if len(block.Questions) > 0 {
				found = true
				assert.Contains(t, string(block.Questions[0].Prompt), "Iron")
			}
		}
	}
	assert.True(t, found, "the quiz block must expose its questions")
}

func TestPlayerViewReflectsProgressAndMastery(t *testing.T) {
	s := newQuizScene(t)

	_, err := s.progress.MarkComplete(s.ctx, s.learner, s.enrolmentID, s.reading,
		progress.Signal{MarkedDone: true})
	require.NoError(t, err)

	_, err = s.progress.SubmitQuiz(s.ctx, s.learner, s.enrolmentID, s.quizID, []progress.Answer{
		{QuestionID: s.questionA, Boolean: boolPtr(true)},
		{QuestionID: s.questionB, Boolean: boolPtr(false)},
	})
	require.NoError(t, err)

	view, err := s.progress.Player(s.ctx, s.learner, s.enrolmentID)
	require.NoError(t, err)

	assert.Equal(t, 2, view.BlocksCompleted, "the reading and the quiz")
	require.Len(t, view.Topics, 1)
	assert.Equal(t, "mastered", view.Topics[0].Level)
	assert.InDelta(t, 1.0, view.Topics[0].Strength, 0.0001)

	var quizStatus string
	for _, mod := range view.Modules {
		for _, block := range mod.Blocks {
			if block.ID == s.quizID {
				quizStatus = block.Status
				require.NotNil(t, block.Score)
				assert.Equal(t, 100, *block.Score)
			}
		}
	}
	assert.Equal(t, progress.StatusCompleted, quizStatus)
}

func TestPlayerViewRejectsForeignEnrolment(t *testing.T) {
	s := newQuizScene(t)
	_, err := s.progress.Player(s.ctx, s.stranger, s.enrolmentID)
	assert.True(t, errors.Is(err, apperr.ErrForbidden))
}

func TestPlayerViewUsesNotStartedForUntouchedBlocks(t *testing.T) {
	s := newQuizScene(t)

	view, err := s.progress.Player(s.ctx, s.learner, s.enrolmentID)
	require.NoError(t, err)

	for _, mod := range view.Modules {
		for _, block := range mod.Blocks {
			assert.Equal(t, "not_started", block.Status,
				"a block with no progress row reads as not_started, not empty string")
			assert.Equal(t, 0, block.Attempts)
		}
	}
}
