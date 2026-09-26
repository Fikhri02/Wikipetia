package course

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/pgconv"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
)

const (
	QuestionMCQSingle = "mcq_single"
	QuestionTrueFalse = "true_false"
	QuestionTyped     = "typed"
)

type MCQPrompt struct {
	Text    string   `json:"text"`
	Options []string `json:"options"`
}

type MCQAnswer struct {
	CorrectIndex int `json:"correct_index"`
}

type TextPrompt struct {
	Text string `json:"text"`
}

type BoolAnswer struct {
	Correct bool `json:"correct"`
}

type TypedAnswer struct {
	Accepted      []string `json:"accepted"`
	CaseSensitive bool     `json:"case_sensitive"`
}

type Question struct {
	ID      uuid.UUID
	BlockID uuid.UUID
	TopicID *uuid.UUID
	Kind    string
	Prompt  json.RawMessage
	Answer  json.RawMessage
	SortKey string
}

type QuestionInput struct {
	Kind    string
	TopicID *uuid.UUID
	Prompt  json.RawMessage
	Answer  json.RawMessage
}

// ValidateQuestion checks a prompt and answer against the question kind,
// returning both normalised. Grading relies on these shapes.
func ValidateQuestion(kind string, prompt, answer json.RawMessage) (json.RawMessage, json.RawMessage, error) {
	switch kind {
	case QuestionMCQSingle:
		var p MCQPrompt
		if err := strictUnmarshal(prompt, &p); err != nil {
			return nil, nil, apperr.Invalid("prompt", "%s", err)
		}
		if strings.TrimSpace(p.Text) == "" {
			return nil, nil, apperr.Invalid("prompt.text", "must not be empty")
		}
		if len(p.Options) < 2 {
			return nil, nil, apperr.Invalid("prompt.options", "needs at least two options")
		}
		var a MCQAnswer
		if err := strictUnmarshal(answer, &a); err != nil {
			return nil, nil, apperr.Invalid("answer", "%s", err)
		}
		if a.CorrectIndex < 0 || a.CorrectIndex >= len(p.Options) {
			return nil, nil, apperr.Invalid("answer.correct_index", "must index one of the options")
		}
		return mustPair(p, a)

	case QuestionTrueFalse:
		var p TextPrompt
		if err := strictUnmarshal(prompt, &p); err != nil {
			return nil, nil, apperr.Invalid("prompt", "%s", err)
		}
		if strings.TrimSpace(p.Text) == "" {
			return nil, nil, apperr.Invalid("prompt.text", "must not be empty")
		}
		var a BoolAnswer
		if err := strictUnmarshal(answer, &a); err != nil {
			return nil, nil, apperr.Invalid("answer", "%s", err)
		}
		return mustPair(p, a)

	case QuestionTyped:
		var p TextPrompt
		if err := strictUnmarshal(prompt, &p); err != nil {
			return nil, nil, apperr.Invalid("prompt", "%s", err)
		}
		if strings.TrimSpace(p.Text) == "" {
			return nil, nil, apperr.Invalid("prompt.text", "must not be empty")
		}
		var a TypedAnswer
		if err := strictUnmarshal(answer, &a); err != nil {
			return nil, nil, apperr.Invalid("answer", "%s", err)
		}
		if len(a.Accepted) == 0 {
			return nil, nil, apperr.Invalid("answer.accepted", "needs at least one accepted answer")
		}
		return mustPair(p, a)

	default:
		return nil, nil, apperr.Invalid("kind", "unknown question kind %q", kind)
	}
}

func mustPair(prompt, answer any) (json.RawMessage, json.RawMessage, error) {
	p, err := remarshal(prompt)
	if err != nil {
		return nil, nil, err
	}
	a, err := remarshal(answer)
	if err != nil {
		return nil, nil, err
	}
	return p, a, nil
}

func (s *Service) CreateQuestion(ctx context.Context, userID, blockID uuid.UUID, in QuestionInput) (Question, error) {
	prompt, answer, err := ValidateQuestion(in.Kind, in.Prompt, in.Answer)
	if err != nil {
		return Question{}, err
	}

	var out Question
	err = s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		block, err := s.loadBlock(ctx, q, userID, blockID)
		if err != nil {
			return err
		}
		if block.Type != BlockQuiz {
			return apperr.Invalid("block", "questions may only be added to quiz blocks")
		}
		if err := s.checkTopicBelongsToBlockCourse(ctx, q, blockID, in.TopicID); err != nil {
			return err
		}

		rows, err := q.ListQuestionsByBlock(ctx, blockID)
		if err != nil {
			return fmt.Errorf("list questions: %w", err)
		}
		sibs := make([]Ordered, 0, len(rows))
		for _, row := range rows {
			sibs = append(sibs, Ordered{ID: row.ID, SortKey: row.SortKey})
		}
		key, err := PlaceKey(sibs, nil, nil)
		if err != nil {
			return err
		}

		row, err := q.CreateQuestion(ctx, sqlcgen.CreateQuestionParams{
			BlockID: blockID,
			TopicID: pgconv.UUID(in.TopicID), // nullable uuid -> pgtype.UUID
			Kind:    in.Kind,
			Prompt:  prompt,
			Answer:  answer,
			SortKey: key,
		})
		if err != nil {
			return fmt.Errorf("create question: %w", err)
		}
		out = toQuestion(row)
		return nil
	})
	return out, err
}

func (s *Service) UpdateQuestion(ctx context.Context, userID, questionID uuid.UUID, in QuestionInput) (Question, error) {
	var out Question
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		existing, err := s.loadQuestion(ctx, q, userID, questionID)
		if err != nil {
			return err
		}

		prompt := in.Prompt
		if len(prompt) == 0 {
			prompt = existing.Prompt
		}
		answer := in.Answer
		if len(answer) == 0 {
			answer = existing.Answer
		}
		validPrompt, validAnswer, err := ValidateQuestion(existing.Kind, prompt, answer)
		if err != nil {
			return err
		}
		if err := s.checkTopicBelongsToBlockCourse(ctx, q, existing.BlockID, in.TopicID); err != nil {
			return err
		}

		row, err := q.UpdateQuestion(ctx, sqlcgen.UpdateQuestionParams{
			ID:      questionID,
			TopicID: pgconv.UUID(in.TopicID),
			Prompt:  validPrompt,
			Answer:  validAnswer,
		})
		if err != nil {
			return fmt.Errorf("update question: %w", err)
		}
		out = toQuestion(row)
		return nil
	})
	return out, err
}

func (s *Service) ArchiveQuestion(ctx context.Context, userID, questionID uuid.UUID) error {
	return s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if _, err := s.loadQuestion(ctx, q, userID, questionID); err != nil {
			return err
		}
		n, err := q.ArchiveQuestion(ctx, questionID)
		if err != nil {
			return fmt.Errorf("archive question: %w", err)
		}
		if n == 0 {
			return apperr.NotFound("question %s", questionID)
		}
		return nil
	})
}

// ListQuestionsForAuthor includes answer keys. It is ONLY reachable by the
// course owner; the learner-facing read strips answers.
func (s *Service) ListQuestionsForAuthor(ctx context.Context, userID, blockID uuid.UUID) ([]Question, error) {
	out := []Question{}
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if _, err := s.loadBlock(ctx, q, userID, blockID); err != nil {
			return err
		}
		rows, err := q.ListQuestionsByBlock(ctx, blockID)
		if err != nil {
			return fmt.Errorf("list questions: %w", err)
		}
		for _, row := range rows {
			out = append(out, toQuestion(row))
		}
		return nil
	})
	return out, err
}

func (s *Service) loadQuestion(ctx context.Context, q *sqlcgen.Queries, userID, questionID uuid.UUID) (sqlcgen.Question, error) {
	row, err := q.GetQuestionByID(ctx, questionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcgen.Question{}, apperr.NotFound("question %s", questionID)
	}
	if err != nil {
		return sqlcgen.Question{}, fmt.Errorf("get question: %w", err)
	}
	if _, err := s.loadBlock(ctx, q, userID, row.BlockID); err != nil {
		return sqlcgen.Question{}, err
	}
	return row, nil
}

func toQuestion(row sqlcgen.Question) Question {
	return Question{
		ID: row.ID, BlockID: row.BlockID, TopicID: pgconv.UUIDPtr(row.TopicID),
		Kind: row.Kind, Prompt: row.Prompt, Answer: row.Answer, SortKey: row.SortKey,
	}
}
