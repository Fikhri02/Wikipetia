package progress_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/events"
	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
	"github.com/Fikhri02/wikipetia/internal/progress"
)

// quizScene extends newScene with a topic and a two-question quiz block.
type quizScene struct {
	scene
	topicID   uuid.UUID
	quizID    uuid.UUID
	questionA uuid.UUID
	questionB uuid.UUID
}

func newQuizScene(t *testing.T) quizScene {
	t.Helper()
	s := newScene(t)
	courses := course.New(s.runner)

	topic, err := courses.CreateTopic(s.ctx, s.learner, s.courseID, "Atoms")
	require.NoError(t, err)

	quiz, err := courses.CreateBlock(s.ctx, s.learner, s.moduleID, course.CreateBlockInput{
		Title: "Quiz", Type: course.BlockQuiz,
		Completion: json.RawMessage(`{"kind":"score_threshold","threshold":70}`),
	})
	require.NoError(t, err)

	qa, err := courses.CreateQuestion(s.ctx, s.learner, quiz.ID, course.QuestionInput{
		Kind: course.QuestionTrueFalse, TopicID: &topic.ID,
		Prompt: json.RawMessage(`{"text":"Iron is an element"}`),
		Answer: json.RawMessage(`{"correct":true}`),
	})
	require.NoError(t, err)

	qb, err := courses.CreateQuestion(s.ctx, s.learner, quiz.ID, course.QuestionInput{
		Kind: course.QuestionTrueFalse, TopicID: &topic.ID,
		Prompt: json.RawMessage(`{"text":"Water is an element"}`),
		Answer: json.RawMessage(`{"correct":false}`),
	})
	require.NoError(t, err)

	return quizScene{scene: s, topicID: topic.ID, quizID: quiz.ID, questionA: qa.ID, questionB: qb.ID}
}

func TestSubmitQuizAllCorrectCompletesAndMasters(t *testing.T) {
	s := newQuizScene(t)

	result, err := s.progress.SubmitQuiz(s.ctx, s.learner, s.enrolmentID, s.quizID, []progress.Answer{
		{QuestionID: s.questionA, Boolean: boolPtr(true)},
		{QuestionID: s.questionB, Boolean: boolPtr(false)},
	})
	require.NoError(t, err)

	assert.Equal(t, 100, result.Score)
	assert.Equal(t, progress.StatusCompleted, result.Progress.Status)
	require.Len(t, result.Mastery, 1)
	assert.Equal(t, s.topicID, result.Mastery[0].TopicID)
	assert.InDelta(t, 1.0, result.Mastery[0].Strength, 0.0001,
		"first evidence sets strength outright")
	assert.Equal(t, "mastered", result.Mastery[0].Level)
}

func TestSubmitQuizBelowThresholdDoesNotComplete(t *testing.T) {
	s := newQuizScene(t)

	result, err := s.progress.SubmitQuiz(s.ctx, s.learner, s.enrolmentID, s.quizID, []progress.Answer{
		{QuestionID: s.questionA, Boolean: boolPtr(true)},
		{QuestionID: s.questionB, Boolean: boolPtr(true)}, // wrong
	})
	require.NoError(t, err)

	assert.Equal(t, 50, result.Score)
	assert.Equal(t, progress.StatusInProgress, result.Progress.Status,
		"50 is below the block's 70 threshold")
	assert.Equal(t, "proficient", result.Mastery[0].Level)
}

func TestSubmitQuizEmitsBlockCompletedAndTopicMastered(t *testing.T) {
	s := newQuizScene(t)

	_, err := s.progress.SubmitQuiz(s.ctx, s.learner, s.enrolmentID, s.quizID, []progress.Answer{
		{QuestionID: s.questionA, Boolean: boolPtr(true)},
		{QuestionID: s.questionB, Boolean: boolPtr(false)},
	})
	require.NoError(t, err)

	require.NoError(t, s.runner.Tx(s.ctx, func(q *sqlcgen.Queries) error {
		records, err := events.ListUnconsumed(s.ctx, q, 10)
		require.NoError(t, err)

		kinds := map[string]int{}
		for _, r := range records {
			kinds[r.Kind]++
		}
		assert.Equal(t, 1, kinds[events.KindBlockCompleted])
		assert.Equal(t, 1, kinds[events.KindTopicMastered])
		return nil
	}))
}

func TestResubmitDoesNotRepeatEvents(t *testing.T) {
	s := newQuizScene(t)

	perfect := []progress.Answer{
		{QuestionID: s.questionA, Boolean: boolPtr(true)},
		{QuestionID: s.questionB, Boolean: boolPtr(false)},
	}

	_, err := s.progress.SubmitQuiz(s.ctx, s.learner, s.enrolmentID, s.quizID, perfect)
	require.NoError(t, err)
	_, err = s.progress.SubmitQuiz(s.ctx, s.learner, s.enrolmentID, s.quizID, perfect)
	require.NoError(t, err)

	require.NoError(t, s.runner.Tx(s.ctx, func(q *sqlcgen.Queries) error {
		records, err := events.ListUnconsumed(s.ctx, q, 20)
		require.NoError(t, err)
		kinds := map[string]int{}
		for _, r := range records {
			kinds[r.Kind]++
		}
		assert.Equal(t, 1, kinds[events.KindBlockCompleted],
			"block.completed fires on the transition only")
		assert.Equal(t, 1, kinds[events.KindTopicMastered],
			"topic.mastered fires when the threshold is crossed, not on every submission above it")
		return nil
	}))
}

func TestSubmitQuizCountsAttempts(t *testing.T) {
	s := newQuizScene(t)

	wrong := []progress.Answer{{QuestionID: s.questionA, Boolean: boolPtr(false)}}

	first, err := s.progress.SubmitQuiz(s.ctx, s.learner, s.enrolmentID, s.quizID, wrong)
	require.NoError(t, err)
	assert.Equal(t, 1, first.Progress.Attempts)

	second, err := s.progress.SubmitQuiz(s.ctx, s.learner, s.enrolmentID, s.quizID, wrong)
	require.NoError(t, err)
	assert.Equal(t, 2, second.Progress.Attempts)
}

func TestSubmitQuizRejectsNonQuizBlock(t *testing.T) {
	s := newQuizScene(t)

	_, err := s.progress.SubmitQuiz(s.ctx, s.learner, s.enrolmentID, s.reading, nil)
	assert.True(t, errors.Is(err, apperr.ErrValidation))
}

func TestSubmitQuizRejectsForeignEnrolment(t *testing.T) {
	s := newQuizScene(t)

	_, err := s.progress.SubmitQuiz(s.ctx, s.stranger, s.enrolmentID, s.quizID, []progress.Answer{
		{QuestionID: s.questionA, Boolean: boolPtr(true)},
	})
	assert.True(t, errors.Is(err, apperr.ErrForbidden))
}
