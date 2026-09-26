package course_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
)

func TestValidateQuestionMCQRequiresOptionsAndIndex(t *testing.T) {
	_, _, err := course.ValidateQuestion(course.QuestionMCQSingle,
		json.RawMessage(`{"text":"Pick one","options":[]}`),
		json.RawMessage(`{"correct_index":0}`))
	assert.True(t, errors.Is(err, apperr.ErrValidation), "a question needs at least two options")

	_, _, err = course.ValidateQuestion(course.QuestionMCQSingle,
		json.RawMessage(`{"text":"Pick one","options":["a","b"]}`),
		json.RawMessage(`{"correct_index":5}`))
	assert.True(t, errors.Is(err, apperr.ErrValidation), "correct_index must exist in options")
}

func TestValidateQuestionMCQAccepts(t *testing.T) {
	prompt, answer, err := course.ValidateQuestion(course.QuestionMCQSingle,
		json.RawMessage(`{"text":"Which is an element?","options":["Water","Iron"]}`),
		json.RawMessage(`{"correct_index":1}`))
	require.NoError(t, err)
	assert.Contains(t, string(prompt), "Iron")
	assert.JSONEq(t, `{"correct_index":1}`, string(answer))
}

func TestValidateQuestionTypedRequiresAccepted(t *testing.T) {
	_, _, err := course.ValidateQuestion(course.QuestionTyped,
		json.RawMessage(`{"text":"Symbol for iron?"}`),
		json.RawMessage(`{"accepted":[]}`))
	assert.True(t, errors.Is(err, apperr.ErrValidation))
}

func TestCreateQuestionRequiresQuizBlock(t *testing.T) {
	svc, ctx, owner, _, moduleID := moduleFixture(t)

	reading, err := svc.CreateBlock(ctx, owner, moduleID, course.CreateBlockInput{
		Title: "Reading", Type: course.BlockReading,
		Content: json.RawMessage(`{"body_md":"x"}`),
	})
	require.NoError(t, err)

	_, err = svc.CreateQuestion(ctx, owner, reading.ID, course.QuestionInput{
		Kind:   course.QuestionTrueFalse,
		Prompt: json.RawMessage(`{"text":"Iron is an element"}`),
		Answer: json.RawMessage(`{"correct":true}`),
	})
	assert.True(t, errors.Is(err, apperr.ErrValidation), "questions belong to quiz blocks only")
}

func TestCreateQuestionOnQuizBlock(t *testing.T) {
	svc, ctx, owner, _, moduleID := moduleFixture(t)

	quiz, err := svc.CreateBlock(ctx, owner, moduleID, course.CreateBlockInput{
		Title: "Quiz", Type: course.BlockQuiz,
	})
	require.NoError(t, err)

	q, err := svc.CreateQuestion(ctx, owner, quiz.ID, course.QuestionInput{
		Kind:   course.QuestionTrueFalse,
		Prompt: json.RawMessage(`{"text":"Iron is an element"}`),
		Answer: json.RawMessage(`{"correct":true}`),
	})
	require.NoError(t, err)
	assert.Equal(t, course.QuestionTrueFalse, q.Kind)
	assert.Nil(t, q.TopicID, "a question may be untagged")
}

func TestSetBlockTopicsReplacesTheWholeSet(t *testing.T) {
	svc, ctx, owner, _, courseID := fixture(t)

	mod, err := svc.CreateModule(ctx, owner, courseID, "M1")
	require.NoError(t, err)
	block, err := svc.CreateBlock(ctx, owner, mod.ID, course.CreateBlockInput{
		Title: "R", Type: course.BlockReading, Content: json.RawMessage(`{"body_md":"x"}`),
	})
	require.NoError(t, err)

	atoms, err := svc.CreateTopic(ctx, owner, courseID, "Atoms")
	require.NoError(t, err)
	bonding, err := svc.CreateTopic(ctx, owner, courseID, "Bonding")
	require.NoError(t, err)

	require.NoError(t, svc.SetBlockTopics(ctx, owner, block.ID, []uuid.UUID{atoms.ID, bonding.ID}))
	got, err := svc.GetBlockTopics(ctx, owner, block.ID)
	require.NoError(t, err)
	assert.Len(t, got, 2)

	// PUT semantics: the new list replaces, it does not merge.
	require.NoError(t, svc.SetBlockTopics(ctx, owner, block.ID, []uuid.UUID{bonding.ID}))
	got, err = svc.GetBlockTopics(ctx, owner, block.ID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, bonding.ID, got[0])
}

func TestSetBlockTopicsRejectsForeignTopic(t *testing.T) {
	svc, ctx, owner, _, courseID := fixture(t)

	mod, err := svc.CreateModule(ctx, owner, courseID, "M1")
	require.NoError(t, err)
	block, err := svc.CreateBlock(ctx, owner, mod.ID, course.CreateBlockInput{
		Title: "R", Type: course.BlockReading, Content: json.RawMessage(`{"body_md":"x"}`),
	})
	require.NoError(t, err)

	otherCourse, err := svc.Create(ctx, owner, course.CreateInput{Title: "Physics"})
	require.NoError(t, err)
	foreign, err := svc.CreateTopic(ctx, owner, otherCourse.ID, "Forces")
	require.NoError(t, err)

	err = svc.SetBlockTopics(ctx, owner, block.ID, []uuid.UUID{foreign.ID})
	assert.True(t, errors.Is(err, apperr.ErrValidation),
		"a block may only be tagged with topics from its own course")
}
