package course_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
)

func TestValidateBlockRejectsUnknownType(t *testing.T) {
	_, _, err := course.ValidateBlock("hologram", json.RawMessage(`{}`), nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperr.ErrValidation))
}

func TestReadingBlockRequiresBody(t *testing.T) {
	_, _, err := course.ValidateBlock(course.BlockReading, json.RawMessage(`{"body_md":""}`), nil)
	assert.True(t, errors.Is(err, apperr.ErrValidation))
}

func TestReadingBlockGetsDefaultCompletion(t *testing.T) {
	_, completion, err := course.ValidateBlock(
		course.BlockReading, json.RawMessage(`{"body_md":"# Atoms"}`), nil)
	require.NoError(t, err)
	assert.JSONEq(t, `{"kind":"mark_done"}`, string(completion))
}

func TestVideoBlockRequiresVideoID(t *testing.T) {
	_, _, err := course.ValidateBlock(course.BlockVideoYouTube, json.RawMessage(`{}`), nil)
	assert.True(t, errors.Is(err, apperr.ErrValidation))
}

func TestVideoBlockDefaultsToWatchedToEnd(t *testing.T) {
	_, completion, err := course.ValidateBlock(
		course.BlockVideoYouTube, json.RawMessage(`{"video_id":"dQw4w9WgXcQ"}`), nil)
	require.NoError(t, err)
	assert.JSONEq(t, `{"kind":"watched_to_end"}`, string(completion))
}

func TestQuizBlockDefaultsToSeventyPercent(t *testing.T) {
	_, completion, err := course.ValidateBlock(course.BlockQuiz, json.RawMessage(`{}`), nil)
	require.NoError(t, err)
	assert.JSONEq(t, `{"kind":"score_threshold","threshold":70}`, string(completion))
}

func TestQuizThresholdMustBeAPercentage(t *testing.T) {
	_, _, err := course.ValidateBlock(course.BlockQuiz, json.RawMessage(`{}`),
		json.RawMessage(`{"kind":"score_threshold","threshold":150}`))
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperr.ErrValidation),
		"threshold is a 0-100 percentage, in the same units as block_progress.score")
}

func TestQuizThresholdAcceptsValidValue(t *testing.T) {
	_, completion, err := course.ValidateBlock(course.BlockQuiz, json.RawMessage(`{}`),
		json.RawMessage(`{"kind":"score_threshold","threshold":80}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"kind":"score_threshold","threshold":80}`, string(completion))
}

func TestCompletionKindMustMatchBlockType(t *testing.T) {
	_, _, err := course.ValidateBlock(course.BlockReading,
		json.RawMessage(`{"body_md":"x"}`),
		json.RawMessage(`{"kind":"score_threshold","threshold":70}`))
	assert.True(t, errors.Is(err, apperr.ErrValidation))
}

func TestValidateBlockRejectsUnknownContentFields(t *testing.T) {
	_, _, err := course.ValidateBlock(course.BlockReading,
		json.RawMessage(`{"body_md":"x","sneaky":true}`), nil)
	assert.True(t, errors.Is(err, apperr.ErrValidation),
		"unknown fields are rejected so typos do not silently vanish into jsonb")
}
