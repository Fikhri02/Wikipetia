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

func TestOutlineReturnsTheWholeCourse(t *testing.T) {
	svc, ctx, owner, _, courseID := fixture(t)

	atoms, err := svc.CreateTopic(ctx, owner, courseID, "Atoms")
	require.NoError(t, err)

	m1, err := svc.CreateModule(ctx, owner, courseID, "Module 1")
	require.NoError(t, err)
	m2, err := svc.CreateModule(ctx, owner, courseID, "Module 2")
	require.NoError(t, err)

	reading, err := svc.CreateBlock(ctx, owner, m1.ID, course.CreateBlockInput{
		Title: "Reading", Type: course.BlockReading,
		Content: json.RawMessage(`{"body_md":"# Atoms"}`),
	})
	require.NoError(t, err)
	require.NoError(t, svc.SetBlockTopics(ctx, owner, reading.ID, []uuid.UUID{atoms.ID}))

	quiz, err := svc.CreateBlock(ctx, owner, m2.ID, course.CreateBlockInput{
		Title: "Quiz", Type: course.BlockQuiz,
	})
	require.NoError(t, err)
	_, err = svc.CreateQuestion(ctx, owner, quiz.ID, course.QuestionInput{
		Kind:   course.QuestionTrueFalse,
		Prompt: json.RawMessage(`{"text":"Iron is an element"}`),
		Answer: json.RawMessage(`{"correct":true}`),
	})
	require.NoError(t, err)

	outline, err := svc.Outline(ctx, owner, courseID)
	require.NoError(t, err)

	assert.Equal(t, "Chemistry", outline.Course.Title)
	require.Len(t, outline.Topics, 1)
	require.Len(t, outline.Modules, 2)

	assert.Equal(t, "Module 1", outline.Modules[0].Module.Title)
	require.Len(t, outline.Modules[0].Blocks, 1)
	assert.Equal(t, []uuid.UUID{atoms.ID}, outline.Modules[0].Blocks[0].TopicIDs)
	assert.Equal(t, 0, outline.Modules[0].Blocks[0].QuestionCount)

	require.Len(t, outline.Modules[1].Blocks, 1)
	assert.Equal(t, 1, outline.Modules[1].Blocks[0].QuestionCount)
}

func TestOutlineIncludesEmptyModules(t *testing.T) {
	svc, ctx, owner, _, courseID := fixture(t)

	_, err := svc.CreateModule(ctx, owner, courseID, "Empty")
	require.NoError(t, err)

	outline, err := svc.Outline(ctx, owner, courseID)
	require.NoError(t, err)
	require.Len(t, outline.Modules, 1)
	assert.Empty(t, outline.Modules[0].Blocks,
		"a module with no blocks must still appear, or the builder cannot show it")
}

func TestOutlineRejectsStranger(t *testing.T) {
	svc, ctx, _, stranger, courseID := fixture(t)
	_, err := svc.Outline(ctx, stranger, courseID)
	assert.True(t, errors.Is(err, apperr.ErrForbidden))
}

func TestOutlineExcludesArchived(t *testing.T) {
	svc, ctx, owner, _, courseID := fixture(t)

	mod, err := svc.CreateModule(ctx, owner, courseID, "M1")
	require.NoError(t, err)
	block, err := svc.CreateBlock(ctx, owner, mod.ID, course.CreateBlockInput{
		Title: "Gone", Type: course.BlockReading, Content: json.RawMessage(`{"body_md":"x"}`),
	})
	require.NoError(t, err)
	require.NoError(t, svc.ArchiveBlock(ctx, owner, block.ID))

	outline, err := svc.Outline(ctx, owner, courseID)
	require.NoError(t, err)
	require.Len(t, outline.Modules, 1)
	assert.Empty(t, outline.Modules[0].Blocks)
}
