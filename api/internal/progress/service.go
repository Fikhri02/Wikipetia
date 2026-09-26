package progress

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Fikhri02/wikipetia/internal/enrolment"
	"github.com/Fikhri02/wikipetia/internal/events"
	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/db"
	"github.com/Fikhri02/wikipetia/internal/platform/pgconv"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
)

type BlockProgress struct {
	BlockID     uuid.UUID
	Status      string
	Score       *int
	Attempts    int
	CompletedAt *time.Time
}

type Service struct {
	runner     db.Runner
	enrolments *enrolment.Service
}

func New(runner db.Runner, enrolments *enrolment.Service) *Service {
	return &Service{runner: runner, enrolments: enrolments}
}

// MarkComplete records a completion signal for a block. It is idempotent: the
// event fires only when status transitions to completed, and the original
// completion timestamp is preserved.
func (s *Service) MarkComplete(ctx context.Context, userID, enrolmentID, blockID uuid.UUID, sig Signal) (BlockProgress, error) {
	var out BlockProgress
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

		existing, getErr := q.GetBlockProgress(ctx, sqlcgen.GetBlockProgressParams{
			EnrolmentID: enrolmentID, BlockID: blockID,
		})
		if getErr != nil && !errors.Is(getErr, pgx.ErrNoRows) {
			return fmt.Errorf("get block progress: %w", getErr)
		}
		hadProgress := getErr == nil
		alreadyCompleted := hadProgress && existing.Status == StatusCompleted

		done, err := Evaluate(block.Type, block.Completion, sig)
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

		var score *int32
		if sig.Score != nil {
			v := int32(*sig.Score)
			score = &v
		} else if hadProgress {
			score = existing.Score
		}

		row, err := q.UpsertBlockProgress(ctx, sqlcgen.UpsertBlockProgressParams{
			EnrolmentID: enrolmentID,
			BlockID:     blockID,
			Status:      status,
			Score:       score,
			Attempts:    attempts,
			CompletedAt: pgconv.Timestamptz(completedAt),
		})
		if err != nil {
			return fmt.Errorf("upsert block progress: %w", err)
		}

		// The event fires on the transition only — never on a repeat.
		if done && !alreadyCompleted {
			if err := events.Append(ctx, q, events.Event{
				UserID:      userID,
				EnrolmentID: &enrolmentID,
				Kind:        events.KindBlockCompleted,
				Payload: map[string]any{
					"block_id":   blockID.String(),
					"block_type": block.Type,
				},
			}); err != nil {
				return err
			}
		}

		out = toBlockProgress(row)
		return nil
	})
	return out, err
}

func toBlockProgress(row sqlcgen.BlockProgress) BlockProgress {
	bp := BlockProgress{
		BlockID:  row.BlockID,
		Status:   row.Status,
		Attempts: int(row.Attempts),
	}
	if row.Score != nil {
		v := int(*row.Score)
		bp.Score = &v
	}
	bp.CompletedAt = pgconv.TimePtr(row.CompletedAt)
	return bp
}
