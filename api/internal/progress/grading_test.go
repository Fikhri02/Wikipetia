package progress_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/progress"
)

func mcq(id uuid.UUID, topicID *uuid.UUID) course.Question {
	return course.Question{
		ID: id, TopicID: topicID, Kind: course.QuestionMCQSingle,
		Prompt: json.RawMessage(`{"text":"Which is an element?","options":["Water","Iron"]}`),
		Answer: json.RawMessage(`{"correct_index":1}`),
	}
}

func trueFalse(id uuid.UUID, topicID *uuid.UUID) course.Question {
	return course.Question{
		ID: id, TopicID: topicID, Kind: course.QuestionTrueFalse,
		Prompt: json.RawMessage(`{"text":"Iron is an element"}`),
		Answer: json.RawMessage(`{"correct":true}`),
	}
}

func typed(id uuid.UUID, topicID *uuid.UUID) course.Question {
	return course.Question{
		ID: id, TopicID: topicID, Kind: course.QuestionTyped,
		Prompt: json.RawMessage(`{"text":"Symbol for iron?"}`),
		Answer: json.RawMessage(`{"accepted":["Fe"],"case_sensitive":false}`),
	}
}

func intPtr(v int) *int       { return &v }
func boolPtr(v bool) *bool    { return &v }
func strPtr(v string) *string { return &v }

func TestGradeAllCorrectIsOneHundred(t *testing.T) {
	q1, q2, q3 := uuid.New(), uuid.New(), uuid.New()
	questions := []course.Question{mcq(q1, nil), trueFalse(q2, nil), typed(q3, nil)}

	results, score, err := progress.Grade(questions, []progress.Answer{
		{QuestionID: q1, ChoiceIndex: intPtr(1)},
		{QuestionID: q2, Boolean: boolPtr(true)},
		{QuestionID: q3, Text: strPtr("Fe")},
	})
	require.NoError(t, err)
	assert.Equal(t, 100, score)
	require.Len(t, results, 3)
	for _, r := range results {
		assert.True(t, r.Correct)
	}
}

func TestGradeRoundsScoreToNearestPercent(t *testing.T) {
	q1, q2, q3 := uuid.New(), uuid.New(), uuid.New()
	questions := []course.Question{mcq(q1, nil), mcq(q2, nil), mcq(q3, nil)}

	_, score, err := progress.Grade(questions, []progress.Answer{
		{QuestionID: q1, ChoiceIndex: intPtr(1)},
		{QuestionID: q2, ChoiceIndex: intPtr(1)},
		{QuestionID: q3, ChoiceIndex: intPtr(0)},
	})
	require.NoError(t, err)
	assert.Equal(t, 67, score, "2 of 3 rounds to 67, not 66")
}

func TestTypedAnswerIsCaseInsensitiveByDefault(t *testing.T) {
	q := uuid.New()
	_, score, err := progress.Grade([]course.Question{typed(q, nil)},
		[]progress.Answer{{QuestionID: q, Text: strPtr("  fe  ")}})
	require.NoError(t, err)
	assert.Equal(t, 100, score, "whitespace is trimmed and case ignored unless case_sensitive")
}

func TestUnansweredQuestionCountsWrong(t *testing.T) {
	q1, q2 := uuid.New(), uuid.New()
	results, score, err := progress.Grade(
		[]course.Question{mcq(q1, nil), mcq(q2, nil)},
		[]progress.Answer{{QuestionID: q1, ChoiceIndex: intPtr(1)}})
	require.NoError(t, err)
	assert.Equal(t, 50, score)
	require.Len(t, results, 2)
}

func TestAnswerForUnknownQuestionIsRejected(t *testing.T) {
	q := uuid.New()
	_, _, err := progress.Grade([]course.Question{mcq(q, nil)},
		[]progress.Answer{{QuestionID: uuid.New(), ChoiceIndex: intPtr(0)}})
	assert.Error(t, err, "an answer must refer to a question in this block")
}

func TestGradeEmptyQuizIsAnError(t *testing.T) {
	_, _, err := progress.Grade(nil, nil)
	assert.Error(t, err, "a quiz with no questions cannot be scored")
}

func TestGradeCarriesTopicThrough(t *testing.T) {
	topicID := uuid.New()
	q := uuid.New()

	results, _, err := progress.Grade([]course.Question{mcq(q, &topicID)},
		[]progress.Answer{{QuestionID: q, ChoiceIndex: intPtr(1)}})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.NotNil(t, results[0].TopicID)
	assert.Equal(t, topicID, *results[0].TopicID)
}
