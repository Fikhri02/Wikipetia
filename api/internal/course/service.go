// Package course owns authoring: courses, topics, modules, blocks and questions.
// It never imports net/http — cmd/api translates HTTP into these calls.
package course

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Fikhri02/wikipetia/internal/platform/apperr"
	"github.com/Fikhri02/wikipetia/internal/platform/db"
	"github.com/Fikhri02/wikipetia/internal/platform/sqlcgen"
)

type Course struct {
	ID          uuid.UUID
	OwnerID     uuid.UUID
	Title       string
	Description string
	Visibility  string
	PetEnabled  bool
	CreatedAt   time.Time
}

type Service struct {
	runner db.Runner
}

func New(runner db.Runner) *Service {
	return &Service{runner: runner}
}

type CreateInput struct {
	Title       string
	Description string
	Visibility  string
	PetEnabled  bool
}

type UpdateInput struct {
	Title       *string
	Description *string
	Visibility  *string
	PetEnabled  *bool
}

var validVisibility = map[string]bool{"public": true, "unlisted": true, "private": true}

func (s *Service) Create(ctx context.Context, ownerID uuid.UUID, in CreateInput) (Course, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return Course{}, apperr.Invalid("title", "must not be empty")
	}
	visibility := in.Visibility
	if visibility == "" {
		visibility = "private"
	}
	if !validVisibility[visibility] {
		return Course{}, apperr.Invalid("visibility", "must be public, unlisted or private")
	}

	var out Course
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		row, err := q.CreateCourse(ctx, sqlcgen.CreateCourseParams{
			OwnerID:     ownerID,
			Title:       title,
			Description: strings.TrimSpace(in.Description),
			Visibility:  visibility,
			PetEnabled:  in.PetEnabled,
		})
		if err != nil {
			return fmt.Errorf("create course: %w", err)
		}
		out = toCourse(row)
		return nil
	})
	return out, err
}

func (s *Service) ListOwned(ctx context.Context, ownerID uuid.UUID) ([]Course, error) {
	out := []Course{}
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		rows, err := q.ListCoursesByOwner(ctx, ownerID)
		if err != nil {
			return fmt.Errorf("list courses: %w", err)
		}
		for _, row := range rows {
			out = append(out, toCourse(row))
		}
		return nil
	})
	return out, err
}

func (s *Service) Get(ctx context.Context, userID, courseID uuid.UUID) (Course, error) {
	var out Course
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		row, err := s.load(ctx, q, userID, courseID)
		if err != nil {
			return err
		}
		out = toCourse(row)
		return nil
	})
	return out, err
}

func (s *Service) Update(ctx context.Context, userID, courseID uuid.UUID, in UpdateInput) (Course, error) {
	if in.Title != nil && strings.TrimSpace(*in.Title) == "" {
		return Course{}, apperr.Invalid("title", "must not be empty")
	}
	if in.Visibility != nil && !validVisibility[*in.Visibility] {
		return Course{}, apperr.Invalid("visibility", "must be public, unlisted or private")
	}

	var out Course
	err := s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if _, err := s.load(ctx, q, userID, courseID); err != nil {
			return err
		}
		row, err := q.UpdateCourse(ctx, sqlcgen.UpdateCourseParams{
			ID:          courseID,
			Title:       trimPtr(in.Title),
			Description: trimPtr(in.Description),
			Visibility:  in.Visibility,
			PetEnabled:  in.PetEnabled,
		})
		if err != nil {
			return fmt.Errorf("update course: %w", err)
		}
		out = toCourse(row)
		return nil
	})
	return out, err
}

func (s *Service) Archive(ctx context.Context, userID, courseID uuid.UUID) error {
	return s.runner.Tx(ctx, func(q *sqlcgen.Queries) error {
		if _, err := s.load(ctx, q, userID, courseID); err != nil {
			return err
		}
		n, err := q.ArchiveCourse(ctx, courseID)
		if err != nil {
			return fmt.Errorf("archive course: %w", err)
		}
		if n == 0 {
			return apperr.NotFound("course %s", courseID)
		}
		return nil
	})
}

// RequireOwner is the authorization check every child resource reuses.
func (s *Service) RequireOwner(ctx context.Context, q *sqlcgen.Queries, userID, courseID uuid.UUID) error {
	_, err := s.load(ctx, q, userID, courseID)
	return err
}

func (s *Service) load(ctx context.Context, q *sqlcgen.Queries, userID, courseID uuid.UUID) (sqlcgen.Course, error) {
	row, err := q.GetCourseByID(ctx, courseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlcgen.Course{}, apperr.NotFound("course %s", courseID)
	}
	if err != nil {
		return sqlcgen.Course{}, fmt.Errorf("get course: %w", err)
	}
	if row.OwnerID != userID {
		// Slice 1 has no sharing: only the owner may touch a course.
		return sqlcgen.Course{}, apperr.Forbidden("course %s is not yours", courseID)
	}
	return row, nil
}

func toCourse(row sqlcgen.Course) Course {
	return Course{
		ID:          row.ID,
		OwnerID:     row.OwnerID,
		Title:       row.Title,
		Description: row.Description,
		Visibility:  row.Visibility,
		PetEnabled:  row.PetEnabled,
		CreatedAt:   row.CreatedAt,
	}
}

func trimPtr(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	return &t
}
