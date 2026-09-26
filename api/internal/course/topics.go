package course

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
)

type Topic struct {
	ID       uuid.UUID
	CourseID uuid.UUID
	Title    string
	SortKey  string
}

func (s *Service) CreateTopic(ctx context.Context, userID, courseID uuid.UUID, title string) (Topic, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Topic{}, apperr.Invalid("title", "must not be empty")
	}

	var out Topic
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if err := s.RequireOwner(ctx, q, userID, courseID); err != nil {
			return err
		}
		sibs, err := s.topicSiblings(ctx, q, courseID)
		if err != nil {
			return err
		}
		key, err := PlaceKey(sibs, nil, nil)
		if err != nil {
			return err
		}
		row, err := q.CreateTopic(ctx, sqlcgen.CreateTopicParams{
			CourseID: courseID,
			Title:    title,
			SortKey:  key,
		})
		if err != nil {
			return fmt.Errorf("create topic: %w", err)
		}
		out = toTopic(row)
		return nil
	})
	return out, err
}

func (s *Service) ListTopics(ctx context.Context, userID, courseID uuid.UUID) ([]Topic, error) {
	out := []Topic{}
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if err := s.RequireOwner(ctx, q, userID, courseID); err != nil {
			return err
		}
		rows, err := q.ListTopicsByCourse(ctx, courseID)
		if err != nil {
			return fmt.Errorf("list topics: %w", err)
		}
		for _, row := range rows {
			out = append(out, toTopic(row))
		}
		return nil
	})
	return out, err
}

func (s *Service) UpdateTopic(ctx context.Context, userID, topicID uuid.UUID, title string) (Topic, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Topic{}, apperr.Invalid("title", "must not be empty")
	}

	var out Topic
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		existing, err := s.loadTopic(ctx, q, userID, topicID)
		if err != nil {
			return err
		}
		row, err := q.UpdateTopicTitle(ctx, sqlcgen.UpdateTopicTitleParams{
			ID:    existing.ID,
			Title: title,
		})
		if err != nil {
			return fmt.Errorf("update topic: %w", err)
		}
		out = toTopic(row)
		return nil
	})
	return out, err
}

func (s *Service) ArchiveTopic(ctx context.Context, userID, topicID uuid.UUID) error {
	return s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if _, err := s.loadTopic(ctx, q, userID, topicID); err != nil {
			return err
		}
		n, err := q.ArchiveTopic(ctx, topicID)
		if err != nil {
			return fmt.Errorf("archive topic: %w", err)
		}
		if n == 0 {
			return apperr.NotFound("topic %s", topicID)
		}
		return nil
	})
}

// loadTopic resolves a topic and checks the caller owns its course.
func (s *Service) loadTopic(ctx context.Context, q *sqlcgen.Queries, userID, topicID uuid.UUID) (sqlcgen.Topic, error) {
	row, err := q.GetTopicByID(ctx, topicID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcgen.Topic{}, apperr.NotFound("topic %s", topicID)
	}
	if err != nil {
		return sqlcgen.Topic{}, fmt.Errorf("get topic: %w", err)
	}
	if err := s.RequireOwner(ctx, q, userID, row.CourseID); err != nil {
		return sqlcgen.Topic{}, err
	}
	return row, nil
}

func (s *Service) topicSiblings(ctx context.Context, q *sqlcgen.Queries, courseID uuid.UUID) ([]Ordered, error) {
	rows, err := q.ListTopicsByCourse(ctx, courseID)
	if err != nil {
		return nil, fmt.Errorf("list topics: %w", err)
	}
	sibs := make([]Ordered, 0, len(rows))
	for _, row := range rows {
		sibs = append(sibs, Ordered{ID: row.ID, SortKey: row.SortKey})
	}
	return sibs, nil
}

func toTopic(row sqlcgen.Topic) Topic {
	return Topic{ID: row.ID, CourseID: row.CourseID, Title: row.Title, SortKey: row.SortKey}
}
