package course

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
)

type OutlineBlock struct {
	Block
	TopicIDs      []uuid.UUID
	QuestionCount int
}

type OutlineModule struct {
	Module Module
	Blocks []OutlineBlock
}

type Outline struct {
	Course  Course
	Topics  []Topic
	Modules []OutlineModule
}

// Outline returns a whole course in one round trip: the course, its topics, its
// modules in order, and every block with its tags and question count.
func (s *Service) Outline(ctx context.Context, userID, courseID uuid.UUID) (Outline, error) {
	var out Outline
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		courseRow, err := s.load(ctx, q, userID, courseID)
		if err != nil {
			return err
		}
		out.Course = toCourse(courseRow)

		topicRows, err := q.ListTopicsByCourse(ctx, courseID)
		if err != nil {
			return fmt.Errorf("list topics: %w", err)
		}
		out.Topics = make([]Topic, 0, len(topicRows))
		for _, row := range topicRows {
			out.Topics = append(out.Topics, toTopic(row))
		}

		moduleRows, err := q.ListModulesByCourse(ctx, courseID)
		if err != nil {
			return fmt.Errorf("list modules: %w", err)
		}

		blockRows, err := q.OutlineBlocks(ctx, courseID)
		if err != nil {
			return fmt.Errorf("outline blocks: %w", err)
		}

		// Group blocks by module. The query already orders by module then block
		// sort key, so appending preserves order without another sort.
		byModule := make(map[uuid.UUID][]OutlineBlock, len(moduleRows))
		for _, row := range blockRows {
			topicIDs := row.TopicIds
			if topicIDs == nil {
				topicIDs = []uuid.UUID{}
			}
			byModule[row.ModuleID] = append(byModule[row.ModuleID], OutlineBlock{
				Block: Block{
					ID:         row.ID,
					ModuleID:   row.ModuleID,
					Title:      row.Title,
					Type:       row.Type,
					SortKey:    row.SortKey,
					Content:    row.Content,
					Completion: row.Completion,
				},
				TopicIDs:      topicIDs,
				QuestionCount: int(row.QuestionCount),
			})
		}

		out.Modules = make([]OutlineModule, 0, len(moduleRows))
		for _, row := range moduleRows {
			blocks := byModule[row.ID]
			if blocks == nil {
				blocks = []OutlineBlock{}
			}
			out.Modules = append(out.Modules, OutlineModule{
				Module: toModule(row),
				Blocks: blocks,
			})
		}
		return nil
	})
	return out, err
}
