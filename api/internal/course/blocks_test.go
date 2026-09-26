package course_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
)

// moduleFixture extends fixture with one module.
func moduleFixture(t *testing.T) (*course.Service, context.Context, uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	svc, ctx, owner, stranger, courseID := fixture(t)
	mod, err := svc.CreateModule(ctx, owner, courseID, "Module 1")
	require.NoError(t, err)
	return svc, ctx, owner, stranger, mod.ID
}

func TestCreateReadingBlock(t *testing.T) {
	svc, ctx, owner, _, moduleID := moduleFixture(t)

	block, err := svc.CreateBlock(ctx, owner, moduleID, course.CreateBlockInput{
		Title:   "What is an atom",
		Type:    course.BlockReading,
		Content: json.RawMessage(`{"body_md":"# Atoms"}`),
	})
	require.NoError(t, err)
	assert.Equal(t, course.BlockReading, block.Type)
	assert.JSONEq(t, `{"kind":"mark_done"}`, string(block.Completion))
}

func TestCreateBlockRejectsInvalidContent(t *testing.T) {
	svc, ctx, owner, _, moduleID := moduleFixture(t)

	_, err := svc.CreateBlock(ctx, owner, moduleID, course.CreateBlockInput{
		Title:   "Broken",
		Type:    course.BlockVideoYouTube,
		Content: json.RawMessage(`{}`),
	})
	assert.True(t, errors.Is(err, apperr.ErrValidation))
}

func TestCreateBlockRejectsStranger(t *testing.T) {
	svc, ctx, _, stranger, moduleID := moduleFixture(t)

	_, err := svc.CreateBlock(ctx, stranger, moduleID, course.CreateBlockInput{
		Title:   "Sneaky",
		Type:    course.BlockReading,
		Content: json.RawMessage(`{"body_md":"x"}`),
	})
	assert.True(t, errors.Is(err, apperr.ErrForbidden))
}

func TestUpdateBlockRevalidatesContent(t *testing.T) {
	svc, ctx, owner, _, moduleID := moduleFixture(t)

	block, err := svc.CreateBlock(ctx, owner, moduleID, course.CreateBlockInput{
		Title:   "Video",
		Type:    course.BlockVideoYouTube,
		Content: json.RawMessage(`{"video_id":"abc"}`),
	})
	require.NoError(t, err)

	empty := json.RawMessage(`{"video_id":""}`)
	_, err = svc.UpdateBlock(ctx, owner, block.ID, course.UpdateBlockInput{Content: empty})
	assert.True(t, errors.Is(err, apperr.ErrValidation),
		"an update must re-run the type's validator, not just write jsonb")
}

func TestUpdateBlockChangesThreshold(t *testing.T) {
	svc, ctx, owner, _, moduleID := moduleFixture(t)

	block, err := svc.CreateBlock(ctx, owner, moduleID, course.CreateBlockInput{
		Title: "Quiz", Type: course.BlockQuiz,
	})
	require.NoError(t, err)

	completion := json.RawMessage(`{"kind":"score_threshold","threshold":90}`)
	updated, err := svc.UpdateBlock(ctx, owner, block.ID, course.UpdateBlockInput{Completion: completion})
	require.NoError(t, err)
	assert.JSONEq(t, `{"kind":"score_threshold","threshold":90}`, string(updated.Completion))
}

func TestMoveBlockReorders(t *testing.T) {
	svc, ctx, owner, _, moduleID := moduleFixture(t)

	var ids []uuid.UUID
	for _, title := range []string{"A", "B", "C"} {
		b, err := svc.CreateBlock(ctx, owner, moduleID, course.CreateBlockInput{
			Title: title, Type: course.BlockReading,
			Content: json.RawMessage(`{"body_md":"x"}`),
		})
		require.NoError(t, err)
		ids = append(ids, b.ID)
	}

	require.NoError(t, svc.MoveBlock(ctx, owner, ids[2], &ids[0], nil))

	blocks, err := svc.ListBlocks(ctx, owner, moduleID)
	require.NoError(t, err)
	assert.Equal(t, "C", blocks[0].Title)
}

func TestArchiveBlockHidesIt(t *testing.T) {
	svc, ctx, owner, _, moduleID := moduleFixture(t)

	block, err := svc.CreateBlock(ctx, owner, moduleID, course.CreateBlockInput{
		Title: "A", Type: course.BlockReading, Content: json.RawMessage(`{"body_md":"x"}`),
	})
	require.NoError(t, err)

	require.NoError(t, svc.ArchiveBlock(ctx, owner, block.ID))

	blocks, err := svc.ListBlocks(ctx, owner, moduleID)
	require.NoError(t, err)
	assert.Empty(t, blocks)
}
