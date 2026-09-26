package course

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
)

// SetBlockTopics replaces a block's whole tag set. PUT semantics, because the
// UI is a multi-select and set replacement avoids add/remove races.
func (s *Service) SetBlockTopics(ctx context.Context, userID, blockID uuid.UUID, topicIDs []uuid.UUID) error {
	return s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if _, err := s.loadBlock(ctx, q, userID, blockID); err != nil {
			return err
		}
		if err := s.checkTopicsBelongToBlockCourse(ctx, q, blockID, topicIDs); err != nil {
			return err
		}
		if err := q.DeleteBlockTopics(ctx, blockID); err != nil {
			return fmt.Errorf("clear block topics: %w", err)
		}
		for _, topicID := range topicIDs {
			if err := q.AddBlockTopic(ctx, sqlcgen.AddBlockTopicParams{
				BlockID: blockID, TopicID: topicID,
			}); err != nil {
				return fmt.Errorf("add block topic: %w", err)
			}
		}
		return nil
	})
}

func (s *Service) GetBlockTopics(ctx context.Context, userID, blockID uuid.UUID) ([]uuid.UUID, error) {
	out := []uuid.UUID{}
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if _, err := s.loadBlock(ctx, q, userID, blockID); err != nil {
			return err
		}
		ids, err := q.ListBlockTopicIDs(ctx, blockID)
		if err != nil {
			return fmt.Errorf("list block topics: %w", err)
		}
		out = ids
		return nil
	})
	return out, err
}

// checkTopicsBelongToBlockCourse rejects tagging a block with a topic from
// another course, which would make mastery meaningless.
func (s *Service) checkTopicsBelongToBlockCourse(ctx context.Context, q *sqlcgen.Queries, blockID uuid.UUID, topicIDs []uuid.UUID) error {
	if len(topicIDs) == 0 {
		return nil
	}
	courseID, err := q.GetBlockCourseID(ctx, blockID)
	if err != nil {
		return fmt.Errorf("get block course: %w", err)
	}
	count, err := q.CountTopicsInCourse(ctx, sqlcgen.CountTopicsInCourseParams{
		CourseID: courseID, TopicIds: topicIDs,
	})
	if err != nil {
		return fmt.Errorf("count topics: %w", err)
	}
	if int(count) != len(topicIDs) {
		return apperr.Invalid("topic_ids", "every topic must belong to this block's course")
	}
	return nil
}

func (s *Service) checkTopicBelongsToBlockCourse(ctx context.Context, q *sqlcgen.Queries, blockID uuid.UUID, topicID *uuid.UUID) error {
	if topicID == nil {
		return nil
	}
	return s.checkTopicsBelongToBlockCourse(ctx, q, blockID, []uuid.UUID{*topicID})
}
