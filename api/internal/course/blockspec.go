package course

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
)

// Block types. Adding a type means adding one registry entry below and one
// React component in the web app — nothing else in the codebase changes.
const (
	BlockReading      = "reading"
	BlockVideoYouTube = "video_youtube"
	BlockQuiz         = "quiz"
)

type ReadingContent struct {
	BodyMD string `json:"body_md"`
}

type VideoContent struct {
	VideoID string `json:"video_id"`
}

// MarkDoneCompletion covers reading blocks: the learner says they are done.
type MarkDoneCompletion struct {
	Kind string `json:"kind"`
}

// WatchedCompletion covers YouTube blocks: the player reports the end event.
type WatchedCompletion struct {
	Kind string `json:"kind"`
}

// QuizCompletion is satisfied when the graded score reaches Threshold.
// Threshold is a percentage 0-100, the same units as block_progress.score.
type QuizCompletion struct {
	Kind      string `json:"kind"`
	Threshold int    `json:"threshold"`
}

type blockSpec struct {
	validateContent    func(raw json.RawMessage) (json.RawMessage, error)
	defaultCompletion  func() json.RawMessage
	validateCompletion func(raw json.RawMessage) (json.RawMessage, error)
}

var blockSpecs = map[string]blockSpec{
	BlockReading: {
		validateContent: func(raw json.RawMessage) (json.RawMessage, error) {
			var c ReadingContent
			if err := strictUnmarshal(raw, &c); err != nil {
				return nil, apperr.Invalid("content", "%s", err)
			}
			if c.BodyMD == "" {
				return nil, apperr.Invalid("content.body_md", "must not be empty")
			}
			return remarshal(c)
		},
		defaultCompletion: func() json.RawMessage {
			return json.RawMessage(`{"kind":"mark_done"}`)
		},
		validateCompletion: func(raw json.RawMessage) (json.RawMessage, error) {
			var c MarkDoneCompletion
			if err := strictUnmarshal(raw, &c); err != nil {
				return nil, apperr.Invalid("completion", "%s", err)
			}
			if c.Kind != "mark_done" {
				return nil, apperr.Invalid("completion.kind", "a reading block completes with mark_done")
			}
			return remarshal(c)
		},
	},

	BlockVideoYouTube: {
		validateContent: func(raw json.RawMessage) (json.RawMessage, error) {
			var c VideoContent
			if err := strictUnmarshal(raw, &c); err != nil {
				return nil, apperr.Invalid("content", "%s", err)
			}
			if c.VideoID == "" {
				return nil, apperr.Invalid("content.video_id", "must not be empty")
			}
			return remarshal(c)
		},
		defaultCompletion: func() json.RawMessage {
			return json.RawMessage(`{"kind":"watched_to_end"}`)
		},
		validateCompletion: func(raw json.RawMessage) (json.RawMessage, error) {
			var c WatchedCompletion
			if err := strictUnmarshal(raw, &c); err != nil {
				return nil, apperr.Invalid("completion", "%s", err)
			}
			if c.Kind != "watched_to_end" {
				return nil, apperr.Invalid("completion.kind", "a video block completes with watched_to_end")
			}
			return remarshal(c)
		},
	},

	BlockQuiz: {
		validateContent: func(raw json.RawMessage) (json.RawMessage, error) {
			// Quiz content is empty: the questions live in their own table.
			var c struct{}
			if err := strictUnmarshal(raw, &c); err != nil {
				return nil, apperr.Invalid("content", "a quiz block carries no content fields")
			}
			return json.RawMessage(`{}`), nil
		},
		defaultCompletion: func() json.RawMessage {
			return json.RawMessage(`{"kind":"score_threshold","threshold":70}`)
		},
		validateCompletion: func(raw json.RawMessage) (json.RawMessage, error) {
			var c QuizCompletion
			if err := strictUnmarshal(raw, &c); err != nil {
				return nil, apperr.Invalid("completion", "%s", err)
			}
			if c.Kind != "score_threshold" {
				return nil, apperr.Invalid("completion.kind", "a quiz block completes with score_threshold")
			}
			if c.Threshold < 0 || c.Threshold > 100 {
				return nil, apperr.Invalid("completion.threshold", "must be between 0 and 100")
			}
			return remarshal(c)
		},
	},
}

// ValidateBlock checks a block's content and completion rule against its type,
// returning both normalised. A nil or empty completion gets the type's default.
func ValidateBlock(blockType string, content, completion json.RawMessage) (json.RawMessage, json.RawMessage, error) {
	spec, ok := blockSpecs[blockType]
	if !ok {
		return nil, nil, apperr.Invalid("type", "unknown block type %q", blockType)
	}

	if len(content) == 0 {
		content = json.RawMessage(`{}`)
	}
	normalisedContent, err := spec.validateContent(content)
	if err != nil {
		return nil, nil, err
	}

	if len(completion) == 0 || bytes.Equal(bytes.TrimSpace(completion), []byte("null")) {
		return normalisedContent, spec.defaultCompletion(), nil
	}
	normalisedCompletion, err := spec.validateCompletion(completion)
	if err != nil {
		return nil, nil, err
	}
	return normalisedContent, normalisedCompletion, nil
}

// KnownBlockType reports whether a type exists, for use by handlers.
func KnownBlockType(blockType string) bool {
	_, ok := blockSpecs[blockType]
	return ok
}

// strictUnmarshal rejects unknown fields, so a typo in a content payload is an
// error rather than a value silently dropped into jsonb.
func strictUnmarshal(raw json.RawMessage, into any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	return dec.Decode(into)
}

func remarshal(v any) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	return b, nil
}
