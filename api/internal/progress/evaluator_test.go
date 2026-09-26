package progress_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/progress"
)

func TestReadingCompletesOnMarkDone(t *testing.T) {
	completion := json.RawMessage(`{"kind":"mark_done"}`)

	done, err := progress.Evaluate(course.BlockReading, completion, progress.Signal{MarkedDone: true})
	require.NoError(t, err)
	assert.True(t, done)

	done, err = progress.Evaluate(course.BlockReading, completion, progress.Signal{})
	require.NoError(t, err)
	assert.False(t, done)
}

func TestVideoCompletesOnWatchedToEnd(t *testing.T) {
	completion := json.RawMessage(`{"kind":"watched_to_end"}`)

	done, err := progress.Evaluate(course.BlockVideoYouTube, completion, progress.Signal{WatchedToEnd: true})
	require.NoError(t, err)
	assert.True(t, done)

	done, err = progress.Evaluate(course.BlockVideoYouTube, completion, progress.Signal{MarkedDone: true})
	require.NoError(t, err)
	assert.False(t, done, "marking a video done must not substitute for watching it")
}

func TestQuizCompletesAtOrAboveThreshold(t *testing.T) {
	completion := json.RawMessage(`{"kind":"score_threshold","threshold":70}`)

	for _, tc := range []struct {
		score int
		want  bool
	}{{69, false}, {70, true}, {100, true}, {0, false}} {
		score := tc.score
		done, err := progress.Evaluate(course.BlockQuiz, completion, progress.Signal{Score: &score})
		require.NoError(t, err)
		assert.Equal(t, tc.want, done, "score %d against threshold 70", tc.score)
	}
}

func TestQuizWithoutScoreIsNotComplete(t *testing.T) {
	completion := json.RawMessage(`{"kind":"score_threshold","threshold":70}`)
	done, err := progress.Evaluate(course.BlockQuiz, completion, progress.Signal{})
	require.NoError(t, err)
	assert.False(t, done)
}

func TestEvaluateRejectsUnknownBlockType(t *testing.T) {
	_, err := progress.Evaluate("hologram", json.RawMessage(`{}`), progress.Signal{})
	assert.Error(t, err)
}

func TestEvaluateRejectsMalformedCompletion(t *testing.T) {
	_, err := progress.Evaluate(course.BlockQuiz, json.RawMessage(`{"kind":"score_threshold"`), progress.Signal{})
	assert.Error(t, err)
}
