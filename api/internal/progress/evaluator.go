// Package progress owns block completion, quiz grading and per-topic mastery.
// It emits events; it never reads engagement state and must never import
// internal/engagement.
package progress

import (
	"encoding/json"
	"fmt"

	"github.com/Fikhri02/wikipetia/internal/course"
)

const (
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
)

// Signal is what the learner did. Each evaluator reads only the fields its
// block type cares about, so adding a block type adds a field at most.
type Signal struct {
	MarkedDone   bool
	WatchedToEnd bool
	Score        *int // 0-100, set by grading
}

type evaluator func(completion json.RawMessage, sig Signal) (bool, error)

var evaluators = map[string]evaluator{
	course.BlockReading: func(completion json.RawMessage, sig Signal) (bool, error) {
		var rule course.MarkDoneCompletion
		if err := json.Unmarshal(completion, &rule); err != nil {
			return false, fmt.Errorf("progress: reading completion rule: %w", err)
		}
		return sig.MarkedDone, nil
	},

	course.BlockVideoYouTube: func(completion json.RawMessage, sig Signal) (bool, error) {
		var rule course.WatchedCompletion
		if err := json.Unmarshal(completion, &rule); err != nil {
			return false, fmt.Errorf("progress: video completion rule: %w", err)
		}
		// Deliberately does not accept MarkedDone: the YouTube player reports
		// the end event, which is the whole reason this block type is verifiable.
		return sig.WatchedToEnd, nil
	},

	course.BlockQuiz: func(completion json.RawMessage, sig Signal) (bool, error) {
		var rule course.QuizCompletion
		if err := json.Unmarshal(completion, &rule); err != nil {
			return false, fmt.Errorf("progress: quiz completion rule: %w", err)
		}
		if sig.Score == nil {
			return false, nil
		}
		return *sig.Score >= rule.Threshold, nil
	},
}

// Evaluate reports whether a block is complete given its rule and what the
// learner did.
func Evaluate(blockType string, completion json.RawMessage, sig Signal) (bool, error) {
	eval, ok := evaluators[blockType]
	if !ok {
		return false, fmt.Errorf("progress: no evaluator for block type %q", blockType)
	}
	return eval(completion, sig)
}
