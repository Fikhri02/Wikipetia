package progress

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Fikhri02/wikipetia/internal/course"
	"github.com/Fikhri02/wikipetia/internal/events"
	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/pgconv"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
)

type TopicStrength struct {
	TopicID  uuid.UUID
	Strength float64
	Level    string
}

type QuizResult struct {
	Score    int
	Progress BlockProgress
	Results  []QuestionResult
	Mastery  []TopicStrength
}

// SubmitQuiz grades a submission and records everything it implies — block
// progress, per-topic mastery, and the events describing both — in ONE
// transaction. This is the pattern every other write follows.
func (s *Service) SubmitQuiz(ctx context.Context, userID, enrolmentID, blockID uuid.UUID, answers []Answer) (QuizResult, error) {
	var out QuizResult

	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		enrol, err := s.enrolments.Load(ctx, q, userID, enrolmentID)
		if err != nil {
			return err
		}

		block, err := q.GetBlockForProgress(ctx, blockID)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("block %s", blockID)
		}
		if err != nil {
			return fmt.Errorf("get block: %w", err)
		}
		if block.CourseID != enrol.CourseID {
			return apperr.Invalid("block_id", "block %s is not part of this enrolment's course", blockID)
		}
		if block.Type != course.BlockQuiz {
			return apperr.Invalid("block_id", "block %s is not a quiz", blockID)
		}

		// Answer keys are loaded here and never serialised outward.
		questionRows, err := q.ListQuestionsByBlock(ctx, blockID)
		if err != nil {
			return fmt.Errorf("list questions: %w", err)
		}
		questions := make([]course.Question, 0, len(questionRows))
		for _, row := range questionRows {
			questions = append(questions, course.Question{
				ID: row.ID, BlockID: row.BlockID, TopicID: pgconv.UUIDPtr(row.TopicID),
				Kind: row.Kind, Prompt: row.Prompt, Answer: row.Answer, SortKey: row.SortKey,
			})
		}

		results, score, err := Grade(questions, answers)
		if err != nil {
			return err
		}
		out.Score = score
		out.Results = results

		// ---- Block progress ----
		existing, getErr := q.GetBlockProgress(ctx, sqlcgen.GetBlockProgressParams{
			EnrolmentID: enrolmentID, BlockID: blockID,
		})
		if getErr != nil && !errors.Is(getErr, pgx.ErrNoRows) {
			return fmt.Errorf("get block progress: %w", getErr)
		}
		hadProgress := getErr == nil
		alreadyCompleted := hadProgress && existing.Status == StatusCompleted

		done, err := Evaluate(block.Type, block.Completion, Signal{Score: &score})
		if err != nil {
			return err
		}

		status := StatusInProgress
		var completedAt *time.Time
		if done || alreadyCompleted {
			status = StatusCompleted
			now := time.Now().UTC()
			completedAt = &now
		}

		attempts := int32(1)
		if hadProgress {
			attempts = existing.Attempts + 1
		}
		score32 := int32(score)

		progressRow, err := q.UpsertBlockProgress(ctx, sqlcgen.UpsertBlockProgressParams{
			EnrolmentID: enrolmentID,
			BlockID:     blockID,
			Status:      status,
			Score:       &score32,
			Attempts:    attempts,
			CompletedAt: pgconv.Timestamptz(completedAt),
		})
		if err != nil {
			return fmt.Errorf("upsert block progress: %w", err)
		}
		out.Progress = toBlockProgress(progressRow)

		if done && !alreadyCompleted {
			if err := events.Append(ctx, q, events.Event{
				UserID: userID, EnrolmentID: &enrolmentID,
				Kind: events.KindBlockCompleted,
				Payload: map[string]any{
					"block_id": blockID.String(), "block_type": block.Type, "score": score,
				},
			}); err != nil {
				return err
			}
		}

		// ---- Per-topic mastery ----
		// Score each topic on this submission alone, then fold it into that
		// topic's running strength.
		type tally struct{ correct, total int }
		perTopic := map[uuid.UUID]*tally{}
		order := []uuid.UUID{}
		for _, r := range results {
			if r.TopicID == nil {
				continue // untagged questions count toward completion, not mastery
			}
			t, ok := perTopic[*r.TopicID]
			if !ok {
				t = &tally{}
				perTopic[*r.TopicID] = t
				order = append(order, *r.TopicID)
			}
			t.total++
			if r.Correct {
				t.correct++
			}
		}

		for _, topicID := range order {
			t := perTopic[topicID]
			topicScore := int(float64(t.correct) / float64(t.total) * 100)

			prevStrength := 0.0
			prevCount := 0
			prevMastered := false
			current, err := q.GetTopicMastery(ctx, sqlcgen.GetTopicMasteryParams{
				EnrolmentID: enrolmentID, TopicID: topicID,
			})
			if err == nil {
				prevStrength = current.Strength
				prevCount = int(current.EvidenceCount)
				prevMastered = current.Strength >= MasteredThreshold
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("get topic mastery: %w", err)
			}

			newStrength := UpdateStrength(prevStrength, prevCount, topicScore)
			row, err := q.UpsertTopicMastery(ctx, sqlcgen.UpsertTopicMasteryParams{
				EnrolmentID:   enrolmentID,
				TopicID:       topicID,
				Strength:      newStrength,
				EvidenceCount: int32(prevCount + 1),
			})
			if err != nil {
				return fmt.Errorf("upsert topic mastery: %w", err)
			}

			out.Mastery = append(out.Mastery, TopicStrength{
				TopicID:  row.TopicID,
				Strength: row.Strength,
				Level:    Level(row.Strength, int(row.EvidenceCount)),
			})

			// Fires on the crossing only, so a learner who stays mastered does
			// not generate an event per submission.
			if newStrength >= MasteredThreshold && !prevMastered {
				if err := events.Append(ctx, q, events.Event{
					UserID: userID, EnrolmentID: &enrolmentID,
					Kind: events.KindTopicMastered,
					Payload: map[string]any{
						"topic_id": topicID.String(), "strength": newStrength,
					},
				}); err != nil {
					return err
				}
			}
		}

		return nil
	})

	if err != nil {
		return QuizResult{}, err
	}
	return out, nil
}
